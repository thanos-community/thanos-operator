package manifests

import (
	"context"

	"k8s.io/apimachinery/pkg/types"

	monitoringv1 "github.com/prometheus-operator/prometheus-operator/pkg/apis/monitoring/v1"
	"k8s.io/apimachinery/pkg/api/errors"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"sigs.k8s.io/controller-runtime/pkg/client"
)

func BuildServiceMonitor(opts Options, port string) *monitoringv1.ServiceMonitor {
	labels := make(map[string]string)
	for k, v := range opts.Labels {
		labels[k] = v
	}
	labels["service-monitor-selector"] = "thanos"
	return &monitoringv1.ServiceMonitor{
		TypeMeta: metav1.TypeMeta{
			Kind:       "ServiceMonitor",
			APIVersion: monitoringv1.SchemeGroupVersion.String(),
		},
		ObjectMeta: metav1.ObjectMeta{
			Name:      opts.Name,
			Namespace: opts.Namespace,
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

func DeleteServiceMonitor(ctx context.Context, client client.Client, name string, namespace string) error {
	sm := &monitoringv1.ServiceMonitor{}
	if err := client.Get(ctx, types.NamespacedName{
		Name:      name,
		Namespace: namespace}, sm); err != nil {
		if errors.IsNotFound(err) {
			return nil
		}
	}
	if err := client.Delete(ctx, sm); err != nil {
		return err
	}
	return nil
}
