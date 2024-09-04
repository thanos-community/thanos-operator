package manifests

import (
	monitoringv1 "github.com/prometheus-operator/prometheus-operator/pkg/apis/monitoring/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
)

func BuildServiceMonitor(opts Options, port string) *monitoringv1.ServiceMonitor {
	opts.Labels["service-monitor"] = "thanos"
	return &monitoringv1.ServiceMonitor{
		TypeMeta: metav1.TypeMeta{
			Kind:       "ServiceMonitor",
			APIVersion: monitoringv1.SchemeGroupVersion.String(),
		},
		ObjectMeta: metav1.ObjectMeta{
			Name:      opts.Name,
			Namespace: opts.Namespace,
			Labels:    opts.Labels,
		},
		Spec: monitoringv1.ServiceMonitorSpec{
			Selector: metav1.LabelSelector{
				MatchLabels: map[string]string{"thanos-self-monitoring": "true"},
			},
			NamespaceSelector: monitoringv1.NamespaceSelector{
				MatchNames: []string{opts.Namespace},
			},
			Endpoints: []monitoringv1.Endpoint{
				{
					Interval: "30s",
					Port:     port,
					Path:     "/metrics",
				},
			},
		},
	}
}
