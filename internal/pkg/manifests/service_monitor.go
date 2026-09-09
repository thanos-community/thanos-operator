package manifests

import (
	monitoringv1 "github.com/prometheus-operator/prometheus-operator/pkg/apis/monitoring/v1"

	"github.com/thanos-community/thanos-operator/internal/pkg/featuregate"

	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
)

func BuildServiceMonitor(name, namespace string, objectMetaLabels, selectorLabels map[string]string, cfg featuregate.ServiceMonitorConfig, port string) *monitoringv1.ServiceMonitor {
	endpoint := monitoringv1.Endpoint{
		Port:     port,
		Path:     "/metrics",
		Interval: monitoringv1.Duration(cfg.Interval),
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
