package tls

import (
	"context"
	"crypto/ecdsa"
	"crypto/elliptic"
	"crypto/rand"
	"crypto/x509"
	"crypto/x509/pkix"
	"encoding/pem"
	"math/big"
	"strings"
	"time"

	cmv1 "github.com/cert-manager/cert-manager/pkg/apis/certmanager/v1"
	cmmeta "github.com/cert-manager/cert-manager/pkg/apis/meta/v1"
	. "github.com/onsi/ginkgo/v2"
	. "github.com/onsi/gomega"
	monitoringv1 "github.com/prometheus-operator/prometheus-operator/pkg/apis/monitoring/v1"
	corev1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"sigs.k8s.io/controller-runtime/pkg/client"

	"github.com/thanos-community/thanos-operator/api/v1alpha1"
	"github.com/thanos-community/thanos-operator/internal/pkg/featuregate"
	"github.com/thanos-community/thanos-operator/internal/pkg/manifests"
	"github.com/thanos-community/thanos-operator/test/integration/suite"
)

const objstoreYAML = `type: S3
config:
  bucket: test
  endpoint: localhost:9000
  access_key: test
  secret_key: test
  insecure: true
`

func objstoreConfig() v1alpha1.ObjectStorageConfig {
	return v1alpha1.ObjectStorageConfig{
		LocalObjectReference: corev1.LocalObjectReference{Name: "thanos-objstore"},
		Key:                  "thanos.yaml",
	}
}

func commonFields() v1alpha1.CommonFields {
	return v1alpha1.CommonFields{Version: new("v0.42.0")}
}

func argValue(args []string, prefix string) string {
	for _, arg := range args {
		if strings.HasPrefix(arg, prefix) {
			return strings.TrimPrefix(arg, prefix)
		}
	}
	return ""
}

func startNamespace(namespace string) {
	ctx = context.Background()
	Expect(k8sClient.Create(ctx, &corev1.Namespace{ObjectMeta: metav1.ObjectMeta{Name: namespace}})).To(Succeed())
	flags := featuregate.Flag{featuregate.ServerTLS, featuregate.ServiceMonitor}
	_, managerCtx, cancel := suite.StartControllers(env, flags.ToFeatureGate(), suite.WithWatchNamespace(namespace))
	ctx = managerCtx
	DeferCleanup(cancel)
}

func createNamespace(namespace string) {
	startNamespace(namespace)
	Expect(k8sClient.Create(ctx, &corev1.Secret{
		ObjectMeta: metav1.ObjectMeta{Name: "thanos-objstore", Namespace: namespace},
		StringData: map[string]string{"thanos.yaml": objstoreYAML},
	})).To(Succeed())

	// Envtest has no cert-manager controller. Supply its CA output as a fixture.
	Expect(k8sClient.Create(ctx, caSecret(namespace))).To(Succeed())
}

func caSecret(namespace string) *corev1.Secret {
	key, err := ecdsa.GenerateKey(elliptic.P256(), rand.Reader)
	Expect(err).NotTo(HaveOccurred())
	certificate := &x509.Certificate{
		SerialNumber: big.NewInt(1), Subject: pkix.Name{CommonName: namespace},
		NotBefore: time.Now().Add(-time.Hour), NotAfter: time.Now().Add(time.Hour),
		IsCA: true, BasicConstraintsValid: true, KeyUsage: x509.KeyUsageCertSign,
	}
	der, err := x509.CreateCertificate(rand.Reader, certificate, certificate, &key.PublicKey, key)
	Expect(err).NotTo(HaveOccurred())
	keyDER, err := x509.MarshalECPrivateKey(key)
	Expect(err).NotTo(HaveOccurred())
	return &corev1.Secret{
		ObjectMeta: metav1.ObjectMeta{
			Name: featuregate.TLSCAName, Namespace: namespace,
			Labels:      map[string]string{manifests.TLSLabel: "true"},
			Annotations: map[string]string{"cert-manager.io/certificate-name": featuregate.TLSCAName},
		},
		Type: corev1.SecretTypeTLS,
		Data: map[string][]byte{
			corev1.TLSCertKey:       pem.EncodeToMemory(&pem.Block{Type: "CERTIFICATE", Bytes: der}),
			corev1.TLSPrivateKeyKey: pem.EncodeToMemory(&pem.Block{Type: "EC PRIVATE KEY", Bytes: keyDER}),
		},
	}
}

func expectTLSWorkload(workload client.Object, grpc bool, dnsNames ...string) *corev1.PodTemplateSpec {
	EventuallyWithOffset(1, func(g Gomega) {
		g.Expect(k8sClient.Get(ctx, client.ObjectKeyFromObject(workload), workload)).To(Succeed())
		name := manifests.TLSResourceName(workload.GetName())
		template := manifests.PodTemplate(workload)
		g.Expect(template).NotTo(BeNil())
		g.Expect(template.Annotations[manifests.TLSTrustChecksumAnnotation]).NotTo(BeEmpty())
		g.Expect(template.Spec.Containers).NotTo(BeEmpty())
		container := template.Spec.Containers[0]
		g.Expect(container.Args).To(ContainElement("--http.config=" + manifests.TLSWebConfigFile))
		if grpc {
			g.Expect(container.Args).To(ContainElements("--grpc-server-tls-cert="+manifests.TLSCertFile, "--grpc-server-tls-key="+manifests.TLSKeyFile))
		} else {
			g.Expect(container.Args).NotTo(ContainElement(HavePrefix("--grpc-server-tls-")))
		}
		g.Expect(template.Spec.Volumes).To(ContainElements(
			corev1.Volume{Name: "thanos-tls-server", VolumeSource: corev1.VolumeSource{Secret: &corev1.SecretVolumeSource{SecretName: name, DefaultMode: new(int32(420))}}},
			corev1.Volume{Name: "thanos-tls-ca", VolumeSource: corev1.VolumeSource{ConfigMap: &corev1.ConfigMapVolumeSource{LocalObjectReference: corev1.LocalObjectReference{Name: featuregate.TLSCAName}, DefaultMode: new(int32(420)), Items: []corev1.KeyToPath{{Key: featuregate.TLSCAKey, Path: "ca.crt"}}}}},
			corev1.Volume{Name: "thanos-tls-web", VolumeSource: corev1.VolumeSource{ConfigMap: &corev1.ConfigMapVolumeSource{LocalObjectReference: corev1.LocalObjectReference{Name: name}, DefaultMode: new(int32(420))}}},
		))
		g.Expect(container.VolumeMounts).To(ContainElements(
			corev1.VolumeMount{Name: "thanos-tls-server", MountPath: manifests.TLSMountPath + "/server", ReadOnly: true},
			corev1.VolumeMount{Name: "thanos-tls-ca", MountPath: manifests.TLSMountPath + "/ca", ReadOnly: true},
			corev1.VolumeMount{Name: "thanos-tls-web", MountPath: manifests.TLSMountPath + "/web", ReadOnly: true},
		))
		for _, probe := range []*corev1.Probe{container.StartupProbe, container.ReadinessProbe, container.LivenessProbe} {
			if probe != nil && probe.HTTPGet != nil {
				g.Expect(probe.HTTPGet.Scheme).To(Equal(corev1.URISchemeHTTPS))
			}
		}

		key := client.ObjectKey{Name: name, Namespace: workload.GetNamespace()}
		cert := &cmv1.Certificate{}
		g.Expect(k8sClient.Get(ctx, key, cert)).To(Succeed())
		g.Expect(cert.Spec.SecretName).To(Equal(name))
		g.Expect(cert.Spec.DNSNames).To(ConsistOf(dnsNames))
		g.Expect(cert.Spec.IssuerRef).To(Equal(cmmeta.ObjectReference{Name: featuregate.TLSCAName, Kind: "Issuer", Group: "cert-manager.io"}))
		g.Expect(cert.Spec.PrivateKey).NotTo(BeNil())
		g.Expect(cert.Spec.PrivateKey.RotationPolicy).To(Equal(cmv1.RotationPolicyAlways))
		g.Expect(metav1.GetControllerOf(workload)).NotTo(BeNil())
		g.Expect(metav1.GetControllerOf(cert)).To(Equal(metav1.GetControllerOf(workload)))
		web := &corev1.ConfigMap{}
		g.Expect(k8sClient.Get(ctx, key, web)).To(Succeed())
		g.Expect(web.Data).To(HaveKeyWithValue("http.yaml", manifests.TLSWebConfig))
		g.Expect(metav1.GetControllerOf(web)).To(Equal(metav1.GetControllerOf(workload)))

		monitor := &monitoringv1.ServiceMonitor{}
		g.Expect(k8sClient.Get(ctx, client.ObjectKeyFromObject(workload), monitor)).To(Succeed())
		g.Expect(monitor.Spec.Endpoints).To(HaveLen(1))
		endpoint := monitor.Spec.Endpoints[0]
		g.Expect(endpoint.Scheme).To(Equal(new(monitoringv1.SchemeHTTPS)))
		g.Expect(endpoint.TLSConfig).NotTo(BeNil())
		g.Expect(endpoint.TLSConfig.CA.ConfigMap).To(Equal(&corev1.ConfigMapKeySelector{LocalObjectReference: corev1.LocalObjectReference{Name: featuregate.TLSCAName}, Key: featuregate.TLSCAKey}))
		g.Expect(endpoint.TLSConfig.ServerName).To(Equal(new(workload.GetName() + "." + workload.GetNamespace() + ".svc")))
	}).Should(Succeed())
	return manifests.PodTemplate(workload)
}
