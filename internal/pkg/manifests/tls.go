package manifests

import (
	"fmt"
	"strings"
	"time"

	cmv1 "github.com/cert-manager/cert-manager/pkg/apis/certmanager/v1"
	cmmeta "github.com/cert-manager/cert-manager/pkg/apis/meta/v1"
	monitoringv1 "github.com/prometheus-operator/prometheus-operator/pkg/apis/monitoring/v1"
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
	if !cfg.ServerTLSEnabled() {
		return objects
	}
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

// ValidateTLSWorkload rejects Receive transports that cannot provide TLS.
func ValidateTLSWorkload(template *corev1.PodTemplateSpec) error {
	c := template.Spec.Containers[0]
	for _, arg := range c.Args {
		if strings.HasPrefix(arg, "--receive.replication-protocol=capnproto") || strings.HasPrefix(arg, "--receive.capnproto-address=") {
			return fmt.Errorf("TLS requires Receive's gRPC replication protocol")
		}
	}
	return nil
}

func augmentTLS(obj client.Object, opts Options) {
	if !opts.ServerTLSEnabled() {
		return
	}
	t := PodTemplate(obj)
	if t == nil {
		return
	}
	c := &t.Spec.Containers[0]
	name := TLSResourceName(obj.GetName())
	ca := opts.ServerTLS.CABundle()
	t.Labels = MergeMaps(t.Labels, map[string]string{TLSLabel: "true"})
	t.Spec.Volumes = append(t.Spec.Volumes,
		corev1.Volume{Name: "thanos-tls-server", VolumeSource: corev1.VolumeSource{Secret: &corev1.SecretVolumeSource{SecretName: name, DefaultMode: ptr.To(int32(420))}}},
		corev1.Volume{Name: "thanos-tls-ca", VolumeSource: corev1.VolumeSource{ConfigMap: &corev1.ConfigMapVolumeSource{LocalObjectReference: corev1.LocalObjectReference{Name: ca.Name}, DefaultMode: ptr.To(int32(420)), Items: []corev1.KeyToPath{{Key: ca.Key, Path: "ca.crt"}}}}},
		corev1.Volume{Name: "thanos-tls-web", VolumeSource: corev1.VolumeSource{ConfigMap: &corev1.ConfigMapVolumeSource{LocalObjectReference: corev1.LocalObjectReference{Name: name}, DefaultMode: ptr.To(int32(420))}}},
	)
	c.VolumeMounts = append(c.VolumeMounts,
		corev1.VolumeMount{Name: "thanos-tls-server", MountPath: TLSMountPath + "/server", ReadOnly: true},
		corev1.VolumeMount{Name: "thanos-tls-ca", MountPath: TLSMountPath + "/ca", ReadOnly: true},
		corev1.VolumeMount{Name: "thanos-tls-web", MountPath: TLSMountPath + "/web", ReadOnly: true},
	)
	args := []string{"--http.config=" + TLSWebConfigFile}
	switch c.Args[0] {
	case "query", "store", receiveCommand, "rule":
		args = append(args, "--grpc-server-tls-cert="+TLSCertFile, "--grpc-server-tls-key="+TLSKeyFile)
	}
	if c.Args[0] == receiveCommand {
		args = append(args, "--remote-write.server-tls-cert="+TLSCertFile, "--remote-write.server-tls-key="+TLSKeyFile,
			"--remote-write.client-tls-secure", "--remote-write.client-tls-ca="+TLSCAFile)
	}
	c.Args = MergeArgs(c.Args, args)
	for _, probe := range []*corev1.Probe{c.StartupProbe, c.ReadinessProbe, c.LivenessProbe} {
		if probe != nil && probe.HTTPGet != nil {
			probe.HTTPGet.Scheme = corev1.URISchemeHTTPS
		}
	}
	// The existing reloader's HTTP client supports the pod-local TLS endpoint.
	if c.Args[0] == "rule" {
		for i := range t.Spec.Containers {
			if t.Spec.Containers[i].Name != "config-reloader" {
				continue
			}
			for j, arg := range t.Spec.Containers[i].Args {
				if strings.HasPrefix(arg, "--reload-url=http://localhost:") {
					t.Spec.Containers[i].Args[j] = strings.Replace(arg, "http://", "https://", 1)
				}
			}
		}
	}
}

// ConfigureTLSMonitors uses the public trust bundle for Prometheus scrapes.
func ConfigureTLSMonitors(objects []client.Object, cfg featuregate.Config, port string) []client.Object {
	if !cfg.ServerTLSEnabled() {
		return objects
	}
	for _, obj := range objects {
		sm, ok := obj.(*monitoringv1.ServiceMonitor)
		if !ok {
			continue
		}
		ca := cfg.ServerTLS.CABundle()
		for i := range sm.Spec.Endpoints {
			ep := &sm.Spec.Endpoints[i]
			if ep.Port != port {
				continue
			}
			ep.Scheme = ptr.To(monitoringv1.SchemeHTTPS)
			ep.TLSConfig = &monitoringv1.TLSConfig{SafeTLSConfig: monitoringv1.SafeTLSConfig{
				CA:         monitoringv1.SecretOrConfigMap{ConfigMap: &corev1.ConfigMapKeySelector{LocalObjectReference: corev1.LocalObjectReference{Name: ca.Name}, Key: ca.Key}},
				ServerName: new(ServiceDNSName(sm.Name, sm.Namespace)),
			}}
		}
	}
	return objects
}
