package manifests

import (
	monitoringv1 "github.com/prometheus-operator/prometheus-operator/pkg/apis/monitoring/v1"
	corev1 "k8s.io/api/core/v1"
	"k8s.io/utils/ptr"

	"github.com/thanos-community/thanos-operator/internal/pkg/featuregate"

	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
)

func BuildServiceMonitor(name, namespace string, objectMetaLabels, selectorLabels map[string]string, cfg featuregate.ServiceMonitorConfig, port string, tlsConfig *featuregate.ServerTLSConfig) *monitoringv1.ServiceMonitor {
	endpoint := monitoringv1.Endpoint{
		Port:     port,
		Path:     "/metrics",
		Interval: monitoringv1.Duration(cfg.Interval),
	}
	if tlsConfig != nil {
		ca := tlsConfig.CABundle()
		endpoint.Scheme = ptr.To(monitoringv1.SchemeHTTPS)
		endpoint.TLSConfig = &monitoringv1.TLSConfig{SafeTLSConfig: monitoringv1.SafeTLSConfig{
			CA:         monitoringv1.SecretOrConfigMap{ConfigMap: &corev1.ConfigMapKeySelector{LocalObjectReference: corev1.LocalObjectReference{Name: ca.Name}, Key: ca.Key}},
			ServerName: new(ServiceDNSName(name, namespace)),
		}}
	}

	return &monitoringv1.ServiceMonitor{
		TypeMeta: metav1.TypeMeta{
			Kind:       "ServiceMonitor",
			APIVersion: monitoringv1.SchemeGroupVersion.String(),
		},
		ObjectMeta: metav1.ObjectMeta{
			Name:      name,
			Namespace: namespace,
			Labels:    MergeMaps(cfg.AdditionalLabels, objectMetaLabels),
		},
		Spec: monitoringv1.ServiceMonitorSpec{
			Selector: metav1.LabelSelector{
				MatchLabels: selectorLabels,
			},
			NamespaceSelector: monitoringv1.NamespaceSelector{
				MatchNames: []string{namespace},
			},
			Endpoints: []monitoringv1.Endpoint{endpoint},
		},
	}
}
