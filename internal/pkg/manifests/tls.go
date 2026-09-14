package manifests

import (
	"time"

	cmv1 "github.com/cert-manager/cert-manager/pkg/apis/certmanager/v1"
	cmmeta "github.com/cert-manager/cert-manager/pkg/apis/meta/v1"
	appsv1 "k8s.io/api/apps/v1"
	corev1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/utils/ptr"
	"sigs.k8s.io/controller-runtime/pkg/client"

	"github.com/thanos-community/thanos-operator/internal/pkg/featuregate"
)

const (
	TLSBootstrapIssuerName     = "thanos-operator-selfsigned"
	receiveCommand             = "receive"
	TLSLabel                   = "operator.thanos.io/tls"
	TLSTrustChecksumAnnotation = "operator.thanos.io/tls-trust-checksum"
	TLSMountPath               = "/etc/thanos/tls"
	TLSCAFile                  = TLSMountPath + "/ca/ca.crt"
	TLSCertFile                = TLSMountPath + "/server/tls.crt"
	TLSKeyFile                 = TLSMountPath + "/server/tls.key"
	TLSWebConfigFile           = TLSMountPath + "/web/http.yaml"
	TLSWebConfig               = `tls_server_config:
  cert_file: ` + TLSCertFile + `
  key_file: ` + TLSKeyFile + `
  min_version: TLS12
`
)

func TLSResourceName(name string) string { return SanitizeName(name + "-tls") }

// TLSResourceNames returns the leaf resource names for the expected workloads.
func TLSResourceNames(workloads []string) []string {
	names := make([]string, len(workloads))
	for i, name := range workloads {
		names[i] = TLSResourceName(name)
	}
	return names
}

func ServiceDNSName(service, namespace string) string {
	return service + "." + namespace + ".svc"
}

func PodTemplate(obj client.Object) *corev1.PodTemplateSpec {
	switch obj := obj.(type) {
	case *appsv1.Deployment:
		return &obj.Spec.Template
	case *appsv1.StatefulSet:
		return &obj.Spec.Template
	default:
		return nil
	}
}

// BuildNamespaceTLSResources defines the shared CA and its bootstrap and signing issuers.
func BuildNamespaceTLSResources(namespace string) []client.Object {
	metadata := func(name string) metav1.ObjectMeta {
		return metav1.ObjectMeta{Name: name, Namespace: namespace, Labels: map[string]string{TLSLabel: "true"}}
	}
	return []client.Object{
		&cmv1.Issuer{
			ObjectMeta: metadata(TLSBootstrapIssuerName),
			Spec:       cmv1.IssuerSpec{IssuerConfig: cmv1.IssuerConfig{SelfSigned: &cmv1.SelfSignedIssuer{}}},
		},
		&cmv1.Certificate{
			ObjectMeta: metadata(featuregate.TLSCAName),
			Spec: cmv1.CertificateSpec{
				SecretName: featuregate.TLSCAName, IsCA: true,
				CommonName:     SanitizeName("thanos-ca-" + namespace),
				Duration:       &metav1.Duration{Duration: 10 * 365 * 24 * time.Hour},
				RenewBefore:    &metav1.Duration{Duration: 365 * 24 * time.Hour},
				IssuerRef:      cmmeta.ObjectReference{Name: TLSBootstrapIssuerName, Kind: "Issuer", Group: "cert-manager.io"},
				PrivateKey:     &cmv1.CertificatePrivateKey{Algorithm: cmv1.ECDSAKeyAlgorithm, Size: 256, RotationPolicy: cmv1.RotationPolicyNever},
				SecretTemplate: &cmv1.CertificateSecretTemplate{Labels: map[string]string{TLSLabel: "true"}},
			},
		},
		&cmv1.Issuer{
			ObjectMeta: metadata(featuregate.TLSCAName),
			Spec:       cmv1.IssuerSpec{IssuerConfig: cmv1.IssuerConfig{CA: &cmv1.CAIssuer{SecretName: featuregate.TLSCAName}}},
		},
	}
}

// AppendTLSResources adds the Certificate and HTTP configuration for each workload.
func AppendTLSResources(objects []client.Object, cfg featuregate.Config) []client.Object {
	issuer := cfg.ServerTLS.Issuer()
	for _, obj := range objects {
		template := PodTemplate(obj)
		if template == nil {
			continue
		}
		name := TLSResourceName(obj.GetName())
		dnsName := ServiceDNSName(obj.GetName(), obj.GetNamespace())
		dnsNames := []string{dnsName}
		// Receive forwards to individual pods through their headless service.
		if template.Spec.Containers[0].Args[0] == receiveCommand {
			dnsNames = append(dnsNames, "*."+dnsName)
		}
		objects = append(objects,
			&cmv1.Certificate{
				ObjectMeta: metav1.ObjectMeta{Name: name, Namespace: obj.GetNamespace(), Labels: MergeMaps(obj.GetLabels(), map[string]string{TLSLabel: "true"})},
				Spec: cmv1.CertificateSpec{
					SecretName: name,
					DNSNames:   dnsNames,
					IssuerRef:  cmmeta.ObjectReference{Name: issuer.Name, Kind: issuer.Kind, Group: issuer.Group},
					Usages:     []cmv1.KeyUsage{cmv1.UsageDigitalSignature, cmv1.UsageServerAuth},
					PrivateKey: &cmv1.CertificatePrivateKey{
						Algorithm: cmv1.ECDSAKeyAlgorithm, Size: 256, RotationPolicy: cmv1.RotationPolicyAlways,
					},
					SecretTemplate: &cmv1.CertificateSecretTemplate{Labels: MergeMaps(obj.GetLabels(), map[string]string{TLSLabel: "true"})},
				},
			},
			&corev1.ConfigMap{
				ObjectMeta: metav1.ObjectMeta{Name: name, Namespace: obj.GetNamespace(), Labels: MergeMaps(obj.GetLabels(), map[string]string{TLSLabel: "true"})},
				Data:       map[string]string{"http.yaml": TLSWebConfig},
			},
		)
	}
	return objects
}

// TLSVolumes contains the leaf key pair, public trust, and HTTP configuration.
func TLSVolumes(name string, cfg featuregate.Config) []corev1.Volume {
	ca := cfg.ServerTLS.CABundle()
	return []corev1.Volume{
		{Name: "thanos-tls-server", VolumeSource: corev1.VolumeSource{Secret: &corev1.SecretVolumeSource{SecretName: TLSResourceName(name), DefaultMode: ptr.To(int32(420))}}},
		{Name: "thanos-tls-ca", VolumeSource: corev1.VolumeSource{ConfigMap: &corev1.ConfigMapVolumeSource{LocalObjectReference: corev1.LocalObjectReference{Name: ca.Name}, DefaultMode: ptr.To(int32(420)), Items: []corev1.KeyToPath{{Key: ca.Key, Path: "ca.crt"}}}}},
		{Name: "thanos-tls-web", VolumeSource: corev1.VolumeSource{ConfigMap: &corev1.ConfigMapVolumeSource{LocalObjectReference: corev1.LocalObjectReference{Name: TLSResourceName(name)}, DefaultMode: ptr.To(int32(420))}}},
	}
}

func TLSVolumeMounts() []corev1.VolumeMount {
	return []corev1.VolumeMount{
		{Name: "thanos-tls-server", MountPath: TLSMountPath + "/server", ReadOnly: true},
		{Name: "thanos-tls-ca", MountPath: TLSMountPath + "/ca", ReadOnly: true},
		{Name: "thanos-tls-web", MountPath: TLSMountPath + "/web", ReadOnly: true},
	}
}

func TLSProbeScheme(defaultScheme corev1.URIScheme, cfg featuregate.Config) corev1.URIScheme {
	if cfg.ServerTLSEnabled() {
		return corev1.URISchemeHTTPS
	}
	return defaultScheme
}
