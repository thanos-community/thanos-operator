package coretls

import (
	"fmt"
	"slices"
	"time"

	cmv1 "github.com/cert-manager/cert-manager/pkg/apis/certmanager/v1"
	"github.com/golang/snappy"
	. "github.com/onsi/ginkgo/v2"
	. "github.com/onsi/gomega"
	"github.com/prometheus/prometheus/prompb"
	appsv1 "k8s.io/api/apps/v1"
	corev1 "k8s.io/api/core/v1"
	"k8s.io/apimachinery/pkg/api/resource"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"sigs.k8s.io/controller-runtime/pkg/client"

	"github.com/thanos-community/thanos-operator/api/v1alpha1"
	"github.com/thanos-community/thanos-operator/internal/controller"
	"github.com/thanos-community/thanos-operator/internal/pkg/featuregate"
	"github.com/thanos-community/thanos-operator/internal/pkg/manifests"
	"github.com/thanos-community/thanos-operator/internal/pkg/manifests/compact"
	"github.com/thanos-community/thanos-operator/internal/pkg/manifests/receive"
	"github.com/thanos-community/thanos-operator/internal/pkg/manifests/store"
	"github.com/thanos-community/thanos-operator/test/e2e/suite"
	"github.com/thanos-community/thanos-operator/test/utils"
)

var _ = Describe("TLS lifecycle", Ordered, func() {
	const namespace = "e2e-core-tls-lifecycle"
	BeforeAll(func() {
		suite.Setup(namespace, featuregate.TLS, featuregate.ServiceMonitor)
	})
	router := controller.ReceiveRouterNameFromParent(suite.ReceiveName)
	query := controller.QueryNameFromParent(suite.QueryName)
	frontend := "thanos-query-frontend-" + suite.QueryName
	ruler := controller.RulerNameFromParent("tls")

	It("issues certificates and starts every Thanos component", func() {
		suite.NewReceive(c, namespace)
		suite.NewQuery(c, namespace)
		q := &v1alpha1.ThanosQuery{}
		Expect(c.Get(ctx, client.ObjectKey{Name: suite.QueryName, Namespace: namespace}, q)).To(Succeed())
		q.Spec.QueryFrontend = &v1alpha1.QueryFrontendSpec{CommonFields: v1alpha1.CommonFields{Version: suite.ThanosVersion()}, Replicas: 1}
		Expect(c.Update(ctx, q)).To(Succeed())
		Expect(c.Create(ctx, &v1alpha1.ThanosStore{ObjectMeta: metav1.ObjectMeta{Name: "tls", Namespace: namespace}, Spec: v1alpha1.ThanosStoreSpec{
			CommonFields: v1alpha1.CommonFields{Version: suite.ThanosVersion()}, Replicas: 1, ObjectStorageConfig: suite.ObjStoreConfig(), StorageConfiguration: v1alpha1.StorageConfiguration{Size: resource.MustParse("100Mi")},
		}})).To(Succeed())
		Expect(c.Create(ctx, &v1alpha1.ThanosCompact{ObjectMeta: metav1.ObjectMeta{Name: "tls", Namespace: namespace}, Spec: v1alpha1.ThanosCompactSpec{
			CommonFields: v1alpha1.CommonFields{Version: suite.ThanosVersion()}, ObjectStorageConfig: suite.ObjStoreConfig(), StorageConfiguration: v1alpha1.StorageConfiguration{Size: resource.MustParse("100Mi")},
		}})).To(Succeed())
		Expect(c.Create(ctx, &corev1.ConfigMap{
			ObjectMeta: metav1.ObjectMeta{Name: "tls-rules", Namespace: namespace, Labels: map[string]string{"tls-rules": "true", manifests.DefaultPrometheusRuleLabel: manifests.DefaultPrometheusRuleValue}},
			Data: map[string]string{"rules.yaml": `groups:
- name: tls
  rules:
  - record: tls_recorded_metric
    expr: sum(tls_test_metric)
`},
		})).To(Succeed())
		Expect(c.Create(ctx, &v1alpha1.ThanosRuler{ObjectMeta: metav1.ObjectMeta{Name: "tls", Namespace: namespace}, Spec: v1alpha1.ThanosRulerSpec{
			CommonFields: v1alpha1.CommonFields{Version: suite.ThanosVersion()}, Replicas: 1, EvaluationInterval: "5s", AlertmanagerURL: "http://alertmanager.invalid:9093",
			StorageConfiguration: v1alpha1.StorageConfiguration{Size: resource.MustParse("100Mi")},
			RulerMode:            v1alpha1.RulerMode{Type: "Stateless", Stateless: &v1alpha1.StatelessSpec{}},
			RuleConfigSelector:   metav1.LabelSelector{MatchLabels: map[string]string{"tls-rules": "true", manifests.DefaultPrometheusRuleLabel: manifests.DefaultPrometheusRuleValue}},
		}})).To(Succeed())
		Eventually(func() bool { return utils.VerifyDeploymentReplicasRunning(c, 1, frontend, namespace) }, 3*time.Minute, time.Second).Should(BeTrue())
		for _, name := range []string{ruler, store.Options{Options: manifests.Options{Owner: "tls"}}.GetGeneratedResourceName(), compact.Options{Options: manifests.Options{Owner: "tls"}}.GetGeneratedResourceName()} {
			Eventually(func() bool { return utils.VerifyStatefulSetReplicasRunning(c, 1, name, namespace) }, 5*time.Minute, time.Second).Should(BeTrue(), name)
		}
		certs := &cmv1.CertificateList{}
		Expect(c.List(ctx, certs, client.InNamespace(namespace))).To(Succeed())
		Expect(certs.Items).To(HaveLen(8), "one namespace CA and seven workload certificates")
	})

	It("verifies HTTPS identity and sends data through Receive, Query, Frontend and Ruler", func() {
		Eventually(func() error {
			write := &prompb.WriteRequest{Timeseries: []prompb.TimeSeries{{Labels: []prompb.Label{{Name: "__name__", Value: "tls_test_metric"}}, Samples: []prompb.Sample{{Value: 7, Timestamp: time.Now().UnixMilli()}}}}}
			data, err := write.Marshal()
			if err != nil {
				return err
			}
			_, err = request(namespace, router, receive.RemoteWritePort, "/api/v1/receive", snappy.Encode(nil, data), "")
			return err
		}, 3*time.Minute, time.Second).Should(Succeed())
		Eventually(func() error { return queryMetric(namespace, frontend, "tls_recorded_metric", 7) }, 3*time.Minute, 2*time.Second).Should(Succeed())
		_, err := request(namespace, query, 9090, "/-/healthy", nil, "wrong.test.svc")
		Expect(err).To(MatchError(ContainSubstring("certificate is valid for")), "a server-name mismatch must fail certificate verification")
	})

	It("reloads rule changes while the Ruler serves HTTPS", func() {
		rules := &corev1.ConfigMap{}
		Expect(c.Get(ctx, client.ObjectKey{Name: "tls-rules", Namespace: namespace}, rules)).To(Succeed())
		rules.Data["rules.yaml"] = `groups:
- name: tls
  rules:
  - record: tls_reloaded_metric
    expr: vector(9)
`
		Expect(c.Update(ctx, rules)).To(Succeed())
		Eventually(func() error { return queryMetric(namespace, frontend, "tls_reloaded_metric", 9) }, 3*time.Minute, 2*time.Second).Should(Succeed())
	})

	It("lets Prometheus scrape all components using the public CA", func() {
		expectPrometheusTargets(namespace, 7)
	})

	It("reissues a missing certificate and rolls its workload", func() {
		deployment := &appsv1.Deployment{}
		Expect(c.Get(ctx, client.ObjectKey{Name: query, Namespace: namespace}, deployment)).To(Succeed())
		oldChecksum := deployment.Spec.Template.Annotations[manifests.TLSChecksumAnnotation]
		Expect(oldChecksum).NotTo(BeEmpty())
		Expect(c.Delete(ctx, &corev1.Secret{ObjectMeta: metav1.ObjectMeta{Name: manifests.TLSResourceName(query), Namespace: namespace}})).To(Succeed())
		Eventually(func() string {
			if err := c.Get(ctx, client.ObjectKeyFromObject(deployment), deployment); err != nil {
				return ""
			}
			return deployment.Spec.Template.Annotations[manifests.TLSChecksumAnnotation]
		}, 3*time.Minute, time.Second).Should(And(Not(BeEmpty()), Not(Equal(oldChecksum))))
		Eventually(func() error { return queryMetric(namespace, frontend, "tls_reloaded_metric", 9) }, 3*time.Minute, 2*time.Second).Should(Succeed())
	})

	It("removes leaf resources on disable and reuses namespace trust on re-enable", func() {
		ca := &corev1.Secret{}
		Expect(c.Get(ctx, client.ObjectKey{Name: featuregate.TLSCAName, Namespace: namespace}, ca)).To(Succeed())
		caUID := ca.UID
		operator := &appsv1.Deployment{}
		Expect(c.Get(ctx, client.ObjectKey{Name: "controller-manager", Namespace: namespace}, operator)).To(Succeed())
		operator.Spec.Template.Spec.Containers[0].Args = slices.DeleteFunc(operator.Spec.Template.Spec.Containers[0].Args, func(arg string) bool { return arg == "--enable-feature=tls" })
		Expect(c.Update(ctx, operator)).To(Succeed())
		Eventually(func() error {
			certs := &cmv1.CertificateList{}
			if err := c.List(ctx, certs, client.InNamespace(namespace)); err != nil {
				return err
			}
			if len(certs.Items) != 1 || certs.Items[0].Name != featuregate.TLSCAName {
				return fmt.Errorf("leaf Certificates remain")
			}
			_, err := request(namespace, frontend, 9090, "/api/v1/query?query=vector(1)", nil, "plaintext")
			return err
		}, 3*time.Minute, 2*time.Second).Should(Succeed())
		Expect(c.Get(ctx, client.ObjectKeyFromObject(ca), ca)).To(Succeed())
		Expect(ca.UID).To(Equal(caUID))
		Expect(c.Get(ctx, client.ObjectKeyFromObject(operator), operator)).To(Succeed())
		operator.Spec.Template.Spec.Containers[0].Args = append(operator.Spec.Template.Spec.Containers[0].Args, "--enable-feature=tls")
		Expect(c.Update(ctx, operator)).To(Succeed())
		Eventually(func() error { return queryMetric(namespace, frontend, "tls_reloaded_metric", 9) }, 3*time.Minute, 2*time.Second).Should(Succeed())
		Expect(c.Get(ctx, client.ObjectKeyFromObject(ca), ca)).To(Succeed())
		Expect(ca.UID).To(Equal(caUID))
	})
})
