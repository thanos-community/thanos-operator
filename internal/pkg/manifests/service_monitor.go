package manifests

import (
	monitoringv1 "github.com/prometheus-operator/prometheus-operator/pkg/apis/monitoring/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
)

func BuildServiceMonitor(opts Options, port string) *monitoringv1.ServiceMonitor {
	labels := make(map[string]string)
	for k, v := range opts.Labels {
		labels[k] = v
	}
	for k, v := range opts.ServiceMonitorConfig.Labels {
		labels[k] = v
	}
	labels["thanos-self-monitoring"] = opts.Name
	return &monitoringv1.ServiceMonitor{
		TypeMeta: metav1.TypeMeta{
			Kind:       "ServiceMonitor",
			APIVersion: monitoringv1.SchemeGroupVersion.String(),
		},
		ObjectMeta: metav1.ObjectMeta{
			Name:      opts.Name,
			Namespace: opts.Namespace, // Future: use namespace from ServiceMonitorConfig
			Labels:    labels,
		},
		Spec: monitoringv1.ServiceMonitorSpec{
			Selector: metav1.LabelSelector{
				MatchLabels: map[string]string{"thanos-self-monitoring": opts.Name},
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
