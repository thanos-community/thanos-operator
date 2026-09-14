package tls

import (
	"fmt"
	"time"

	cmv1 "github.com/cert-manager/cert-manager/pkg/apis/certmanager/v1"
	cmmeta "github.com/cert-manager/cert-manager/pkg/apis/meta/v1"
	. "github.com/onsi/ginkgo/v2"
	. "github.com/onsi/gomega"
	appsv1 "k8s.io/api/apps/v1"
	corev1 "k8s.io/api/core/v1"
	discoveryv1 "k8s.io/api/discovery/v1"
	apierrors "k8s.io/apimachinery/pkg/api/errors"
	"k8s.io/apimachinery/pkg/api/resource"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"sigs.k8s.io/controller-runtime/pkg/client"
	"sigs.k8s.io/yaml"

	"github.com/thanos-community/thanos-operator/api/v1alpha1"
	"github.com/thanos-community/thanos-operator/internal/controller"
	"github.com/thanos-community/thanos-operator/internal/pkg/featuregate"
	"github.com/thanos-community/thanos-operator/internal/pkg/manifests"
	"github.com/thanos-community/thanos-operator/internal/pkg/manifests/compact"
	"github.com/thanos-community/thanos-operator/internal/pkg/manifests/store"
)

var _ = Describe("TLS feature gate", func() {
	It("bootstraps namespace trust before any component exists", func() {
		const ns = "tls-controller"
		startNamespace(ns)
		key := client.ObjectKey{Namespace: ns, Name: featuregate.TLSCAName}
		Eventually(func(g Gomega) {
			ca := &cmv1.Certificate{}
			g.Expect(k8sClient.Get(ctx, key, ca)).To(Succeed())
			g.Expect(ca.Spec.IsCA).To(BeTrue())
		}).Should(Succeed())
		Expect(apierrors.IsNotFound(k8sClient.Get(ctx, key, &corev1.ConfigMap{}))).To(BeTrue(), "trust waits for cert-manager to issue the CA")
		secret := caSecret(ns)
		Expect(k8sClient.Create(ctx, secret)).To(Succeed())
		bundle := &corev1.ConfigMap{}
		Eventually(func(g Gomega) {
			g.Expect(k8sClient.Get(ctx, key, bundle)).To(Succeed())
			g.Expect(bundle.Data[featuregate.TLSCAKey]).To(ContainSubstring(string(secret.Data[corev1.TLSCertKey])))
		}).Should(Succeed())
		oldUID := bundle.UID
		Expect(k8sClient.Delete(ctx, bundle)).To(Succeed())
		Eventually(func(g Gomega) {
			g.Expect(k8sClient.Get(ctx, key, bundle)).To(Succeed())
			g.Expect(bundle.UID).NotTo(Equal(oldUID))
			g.Expect(bundle.Data[featuregate.TLSCAKey]).To(ContainSubstring(string(secret.Data[corev1.TLSCertKey])))
		}).Should(Succeed())
		issuer := &cmv1.Issuer{ObjectMeta: metav1.ObjectMeta{Name: "thanos-operator-selfsigned", Namespace: ns}}
		Expect(k8sClient.Get(ctx, client.ObjectKeyFromObject(issuer), issuer)).To(Succeed())
		oldUID = issuer.UID
		Expect(k8sClient.Delete(ctx, issuer)).To(Succeed())
		Eventually(func(g Gomega) {
			g.Expect(k8sClient.Get(ctx, client.ObjectKeyFromObject(issuer), issuer)).To(Succeed())
			g.Expect(issuer.UID).NotTo(Equal(oldUID))
			g.Expect(issuer.Spec.SelfSigned).NotTo(BeNil())
		}).Should(Succeed())
		deployments := &appsv1.DeploymentList{}
		Expect(k8sClient.List(ctx, deployments, client.InNamespace(ns))).To(Succeed())
		Expect(deployments.Items).To(BeEmpty(), "the TLS controller must not reconcile workloads")
	})

	It("updates Deployment and StatefulSet templates when namespace trust changes", func() {
		const ns = "tls-trust-update"
		createNamespace(ns)
		Expect(k8sClient.Create(ctx, &v1alpha1.ThanosQuery{
			ObjectMeta: metav1.ObjectMeta{Name: "test", Namespace: ns},
			Spec:       v1alpha1.ThanosQuerySpec{CommonFields: commonFields(), Replicas: 1},
		})).To(Succeed())
		Expect(k8sClient.Create(ctx, &v1alpha1.ThanosStore{
			ObjectMeta: metav1.ObjectMeta{Name: "test", Namespace: ns},
			Spec: v1alpha1.ThanosStoreSpec{
				CommonFields: commonFields(), Replicas: 1, ObjectStorageConfig: objstoreConfig(),
				StorageConfiguration: v1alpha1.StorageConfiguration{Size: resource.MustParse("1Gi")},
			},
		})).To(Succeed())
		workloads := []client.Object{
			&appsv1.Deployment{ObjectMeta: metav1.ObjectMeta{Name: controller.QueryNameFromParent("test"), Namespace: ns}},
			&appsv1.StatefulSet{ObjectMeta: metav1.ObjectMeta{Name: controller.StoreNameFromParent("test", nil), Namespace: ns}},
		}
		for _, workload := range workloads {
			expectTLSWorkload(workload, true, manifests.ServiceDNSName(workload.GetName(), ns))
		}
		secret := &corev1.Secret{}
		Expect(k8sClient.Get(ctx, client.ObjectKey{Name: featuregate.TLSCAName, Namespace: ns}, secret)).To(Succeed())
		oldRoot := string(secret.Data[corev1.TLSCertKey])
		secret.Data = caSecret(ns).Data
		Expect(k8sClient.Update(ctx, secret)).To(Succeed())
		Eventually(func(g Gomega) {
			bundle := &corev1.ConfigMap{}
			g.Expect(k8sClient.Get(ctx, client.ObjectKeyFromObject(secret), bundle)).To(Succeed())
			g.Expect(bundle.Data[featuregate.TLSCAKey]).To(ContainSubstring(oldRoot))
			g.Expect(bundle.Data[featuregate.TLSCAKey]).To(ContainSubstring(string(secret.Data[corev1.TLSCertKey])))
			for _, workload := range workloads {
				current := workload.DeepCopyObject().(client.Object)
				g.Expect(k8sClient.Get(ctx, client.ObjectKeyFromObject(current), current)).To(Succeed())
				checksum := manifests.PodTemplate(current).Annotations[manifests.TLSTrustChecksumAnnotation]
				g.Expect(checksum).NotTo(BeEmpty())
				g.Expect(checksum).NotTo(Equal(manifests.PodTemplate(workload).Annotations[manifests.TLSTrustChecksumAnnotation]))
			}
		}, 10*time.Second, 100*time.Millisecond).Should(Succeed())
	})

	It("creates namespace issuers, a CA Certificate, and a public trust ConfigMap", func() {
		const ns = "tls-ca"
		createNamespace(ns)
		Expect(k8sClient.Create(ctx, &v1alpha1.ThanosQuery{
			ObjectMeta: metav1.ObjectMeta{Name: "test", Namespace: ns},
			Spec:       v1alpha1.ThanosQuerySpec{CommonFields: commonFields(), Replicas: 1},
		})).To(Succeed())

		Eventually(func(g Gomega) {
			bootstrap := &cmv1.Issuer{}
			g.Expect(k8sClient.Get(ctx, client.ObjectKey{Name: "thanos-operator-selfsigned", Namespace: ns}, bootstrap)).To(Succeed())
			g.Expect(bootstrap.Spec.SelfSigned).NotTo(BeNil())
			g.Expect(bootstrap.OwnerReferences).To(BeEmpty())
			issuer := &cmv1.Issuer{}
			key := client.ObjectKey{Name: featuregate.TLSCAName, Namespace: ns}
			g.Expect(k8sClient.Get(ctx, key, issuer)).To(Succeed())
			g.Expect(issuer.Spec.CA).To(Equal(&cmv1.CAIssuer{SecretName: featuregate.TLSCAName}))
			g.Expect(issuer.OwnerReferences).To(BeEmpty())
			cert := &cmv1.Certificate{}
			g.Expect(k8sClient.Get(ctx, key, cert)).To(Succeed())
			g.Expect(cert.Spec.IsCA).To(BeTrue())
			g.Expect(cert.Spec.SecretName).To(Equal(featuregate.TLSCAName))
			g.Expect(cert.Spec.IssuerRef).To(Equal(cmmeta.ObjectReference{Name: bootstrap.Name, Kind: "Issuer", Group: "cert-manager.io"}))
			g.Expect(cert.OwnerReferences).To(BeEmpty())
			secret := &corev1.Secret{}
			g.Expect(k8sClient.Get(ctx, key, secret)).To(Succeed())
			bundle := &corev1.ConfigMap{}
			g.Expect(k8sClient.Get(ctx, key, bundle)).To(Succeed())
			g.Expect(bundle.Data).To(HaveLen(1))
			g.Expect(bundle.Data[featuregate.TLSCAKey]).To(ContainSubstring(string(secret.Data[corev1.TLSCertKey])))
			g.Expect(bundle.OwnerReferences).To(BeEmpty())
		}).Should(Succeed())
	})

	It("configures Query and Frontend resources and their TLS clients", func() {
		const ns = "tls-query"
		createNamespace(ns)
		Expect(k8sClient.Create(ctx, &corev1.Service{
			ObjectMeta: metav1.ObjectMeta{Name: "store", Namespace: ns, Labels: store.GetRequiredStoreServiceLabel()},
			Spec:       corev1.ServiceSpec{Ports: []corev1.ServicePort{{Name: "grpc", Port: 10901}}},
		})).To(Succeed())
		Expect(k8sClient.Create(ctx, &discoveryv1.EndpointSlice{
			ObjectMeta:  metav1.ObjectMeta{Name: "store", Namespace: ns, Labels: map[string]string{discoveryv1.LabelServiceName: "store"}},
			AddressType: discoveryv1.AddressTypeIPv4,
			Ports:       []discoveryv1.EndpointPort{{Name: new("grpc"), Port: new(int32(10901))}},
			Endpoints:   []discoveryv1.Endpoint{{Addresses: []string{"10.0.0.10"}, Conditions: discoveryv1.EndpointConditions{Ready: new(true)}}},
		})).To(Succeed())
		Expect(k8sClient.Create(ctx, &v1alpha1.ThanosQuery{
			ObjectMeta: metav1.ObjectMeta{Name: "test", Namespace: ns},
			Spec: v1alpha1.ThanosQuerySpec{
				CommonFields: commonFields(), Replicas: 1,
				QueryFrontend: &v1alpha1.QueryFrontendSpec{CommonFields: commonFields(), Replicas: 1},
			},
		})).To(Succeed())
		query := controller.QueryNameFromParent("test")
		frontend := controller.QueryFrontendNameFromParent("test")
		pod := expectTLSWorkload(&appsv1.Deployment{ObjectMeta: metav1.ObjectMeta{Name: query, Namespace: ns}}, true, query+"."+ns+".svc")
		Expect(pod.Spec.Containers[0].Args).To(ContainElement("--endpoint.sd-config-file=" + manifests.TLSMountPath + "/endpoints/endpoints.yaml"))
		Eventually(func(g Gomega) {
			endpoints := &corev1.ConfigMap{}
			g.Expect(k8sClient.Get(ctx, client.ObjectKey{Name: query, Namespace: ns}, endpoints)).To(Succeed())
			g.Expect(endpoints.Data["endpoints.yaml"]).To(MatchJSON(fmt.Sprintf(`{
				"default_client_config": {
					"tls_config": {
						"enabled": true,
						"ca_file": %q
					}
				},
				"endpoints": [{
					"address": "10.0.0.10:10901",
					"strict": false,
					"group": false,
					"client_config": {
						"server_name": "store.tls-query.svc"
					}
				}]
			}`, manifests.TLSCAFile)))
		}).Should(Succeed())
		pod = expectTLSWorkload(&appsv1.Deployment{ObjectMeta: metav1.ObjectMeta{Name: frontend, Namespace: ns}}, false, frontend+"."+ns+".svc")
		Expect(pod.Spec.Containers[0].Args).To(ContainElement("--query-frontend.downstream-url=https://" + query + "." + ns + ".svc:9090"))
		Expect(argValue(pod.Spec.Containers[0].Args, "--query-frontend.downstream-tripper-config=")).To(MatchJSON(fmt.Sprintf(`{
			"tls_config": {
				"ca_file": %q,
				"server_name": %q
			}
		}`, manifests.TLSCAFile, query+"."+ns+".svc")))
	})

	It("configures Receive certificates, remote-write listeners, and forwarding flags", func() {
		const ns = "tls-receive"
		createNamespace(ns)
		Expect(k8sClient.Create(ctx, &v1alpha1.ThanosReceive{
			ObjectMeta: metav1.ObjectMeta{Name: "test", Namespace: ns},
			Spec: v1alpha1.ThanosReceiveSpec{
				Router: v1alpha1.RouterSpec{CommonFields: commonFields(), Replicas: 1, ReplicationFactor: 1},
				Ingester: v1alpha1.IngesterSpec{
					DefaultObjectStorageConfig: objstoreConfig(),
					Hashrings: []v1alpha1.IngesterHashringSpec{{Name: "default", CommonFields: commonFields(), Replicas: 1,
						StorageConfiguration: v1alpha1.StorageConfiguration{Size: resource.MustParse("1Gi")}}},
				},
			},
		})).To(Succeed())
		for _, workload := range []client.Object{
			&appsv1.Deployment{ObjectMeta: metav1.ObjectMeta{Name: controller.ReceiveRouterNameFromParent("test"), Namespace: ns}},
			&appsv1.StatefulSet{ObjectMeta: metav1.ObjectMeta{Name: controller.ReceiveIngesterNameFromParent("test", "default"), Namespace: ns}},
		} {
			name := workload.GetName() + "." + ns + ".svc"
			pod := expectTLSWorkload(workload, true, name, "*."+name)
			Expect(pod.Spec.Containers[0].Args).To(ContainElements(
				"--remote-write.server-tls-cert="+manifests.TLSCertFile, "--remote-write.server-tls-key="+manifests.TLSKeyFile,
				"--remote-write.client-tls-secure", "--remote-write.client-tls-ca="+manifests.TLSCAFile,
			))
		}
	})

	DescribeTable("configures Ruler TLS resources and client flags", func(mode string) {
		ns := "tls-ruler-" + mode
		createNamespace(ns)
		Expect(k8sClient.Create(ctx, &corev1.Service{
			ObjectMeta: metav1.ObjectMeta{Name: "query", Namespace: ns, Labels: controller.RequiredQueryServiceLabels},
			Spec:       corev1.ServiceSpec{Ports: []corev1.ServicePort{{Name: "grpc", Port: 10901}, {Name: "http", Port: 9090}}},
		})).To(Succeed())
		Expect(k8sClient.Create(ctx, &corev1.ConfigMap{
			ObjectMeta: metav1.ObjectMeta{Name: "rules", Namespace: ns, Labels: controller.DefaultRuleLabels},
			Data: map[string]string{"rules.yaml": `groups:
- name: test
  rules:
  - record: test_metric
    expr: vector(1)
`},
		})).To(Succeed())
		ruler := &v1alpha1.ThanosRuler{
			ObjectMeta: metav1.ObjectMeta{Name: "test", Namespace: ns},
			Spec: v1alpha1.ThanosRulerSpec{
				CommonFields: commonFields(), Replicas: 1, AlertmanagerURL: "http://alertmanager.invalid:9093",
				StorageConfiguration: v1alpha1.StorageConfiguration{Size: resource.MustParse("1Gi")},
				RuleConfigSelector:   metav1.LabelSelector{MatchLabels: controller.DefaultRuleLabels},
				RulerMode:            v1alpha1.RulerMode{Type: "Stateful", Stateful: &v1alpha1.StatefulSpec{ObjectStorageConfig: objstoreConfig()}},
			},
		}
		if mode == "stateless" {
			ruler.Spec.RulerMode = v1alpha1.RulerMode{Type: "Stateless", Stateless: &v1alpha1.StatelessSpec{}}
			Expect(k8sClient.Create(ctx, &corev1.Service{
				ObjectMeta: metav1.ObjectMeta{Name: "receive", Namespace: ns, Labels: controller.DefaultRemoteWriteLabels},
				Spec:       corev1.ServiceSpec{Ports: []corev1.ServicePort{{Name: "remote-write", Port: 19291}}},
			})).To(Succeed())
		}
		Expect(k8sClient.Create(ctx, ruler)).To(Succeed())
		name := controller.RulerNameFromParent("test")
		pod := expectTLSWorkload(&appsv1.StatefulSet{ObjectMeta: metav1.ObjectMeta{Name: name, Namespace: ns}}, true, name+"."+ns+".svc")
		Expect(argValue(pod.Spec.Containers[0].Args, "--query.config=")).To(MatchJSON(fmt.Sprintf(`[{
			"scheme": "https",
			"static_configs": ["dnssrv+_http._tcp.query.%s.svc"],
			"http_config": {
				"tls_config": {
					"ca_file": %q,
					"server_name": "query.%s.svc"
				}
			}
		}]`, ns, manifests.TLSCAFile, ns)))
		Expect(pod.Spec.Containers).To(ContainElement(And(HaveField("Name", "config-reloader"), HaveField("Args", ContainElement("--reload-url=https://localhost:9090/-/reload")))))
		if mode == "stateless" {
			Expect(pod.Spec.Containers[0].Args).To(ContainElement("--remote-write.config-file=/etc/thanos/remote-write/remote-write.yaml"))
			Eventually(func(g Gomega) {
				config := &corev1.Secret{}
				g.Expect(k8sClient.Get(ctx, client.ObjectKey{Name: name, Namespace: ns}, config)).To(Succeed())
				var remoteWrite struct {
					RemoteWriteConfigs []struct {
						URL       string            `json:"url"`
						TLSConfig map[string]string `json:"tls_config"` //nolint:tagliatelle // Thanos configuration uses snake_case.
					} `json:"remote_write"` //nolint:tagliatelle // Thanos configuration uses snake_case.
				}
				g.Expect(yaml.Unmarshal(config.Data["remote-write.yaml"], &remoteWrite)).To(Succeed())
				g.Expect(remoteWrite.RemoteWriteConfigs).To(HaveLen(1))
				g.Expect(remoteWrite.RemoteWriteConfigs[0].URL).To(Equal("https://receive." + ns + ".svc:19291/api/v1/receive"))
				g.Expect(remoteWrite.RemoteWriteConfigs[0].TLSConfig).To(HaveKeyWithValue("ca_file", manifests.TLSCAFile))
			}).Should(Succeed())
		}
	}, Entry("stateful", "stateful"), Entry("stateless", "stateless"))

	It("creates separate TLS resources for Store shards", func() {
		const ns = "tls-store"
		createNamespace(ns)
		Expect(k8sClient.Create(ctx, &v1alpha1.ThanosStore{
			ObjectMeta: metav1.ObjectMeta{Name: "test", Namespace: ns},
			Spec: v1alpha1.ThanosStoreSpec{
				CommonFields: commonFields(), Replicas: 1, ObjectStorageConfig: objstoreConfig(),
				StorageConfiguration: v1alpha1.StorageConfiguration{Size: resource.MustParse("1Gi")},
				ShardingStrategy:     v1alpha1.ShardingStrategy{Type: v1alpha1.Block, Shards: 2},
			},
		})).To(Succeed())
		for i := range int32(2) {
			name := controller.StoreNameFromParent("test", new(i))
			expectTLSWorkload(&appsv1.StatefulSet{ObjectMeta: metav1.ObjectMeta{Name: name, Namespace: ns}}, true, name+"."+ns+".svc")
		}
	})

	It("creates separate TLS resources for Compactor shards", func() {
		const ns = "tls-compact"
		createNamespace(ns)
		Expect(k8sClient.Create(ctx, &v1alpha1.ThanosCompact{
			ObjectMeta: metav1.ObjectMeta{Name: "test", Namespace: ns},
			Spec: v1alpha1.ThanosCompactSpec{
				CommonFields: commonFields(), ObjectStorageConfig: objstoreConfig(),
				StorageConfiguration: v1alpha1.StorageConfiguration{Size: resource.MustParse("1Gi")},
				ShardingConfig: []v1alpha1.ShardingConfig{
					{ShardName: "one", ExternalLabelSharding: []v1alpha1.ExternalLabelShardingConfig{{Label: "tenant", Value: "one"}}},
					{ShardName: "two", ExternalLabelSharding: []v1alpha1.ExternalLabelShardingConfig{{Label: "tenant", Value: "two"}}},
				},
			},
		})).To(Succeed())
		for _, shard := range []string{"one", "two"} {
			name := (compact.Options{Options: manifests.Options{Owner: "test"}, ShardName: new(shard)}).GetGeneratedResourceName()
			expectTLSWorkload(&appsv1.StatefulSet{ObjectMeta: metav1.ObjectMeta{Name: name, Namespace: ns}}, false, name+"."+ns+".svc")
		}
	})
})
