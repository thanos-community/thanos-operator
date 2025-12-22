package controller

import (
	monitoringv1 "github.com/prometheus-operator/prometheus-operator/pkg/apis/monitoring/v1"

	"github.com/thanos-community/thanos-operator/internal/pkg/featuregate"

	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"

	"sigs.k8s.io/controller-runtime/pkg/client"
)

// getDisabledFeatureGatedResources returns resources that should be deleted based on feature gates.
func getDisabledFeatureGatedResources(fg featuregate.Config, expectResourceNames []string, namespace string) []client.Object {
	var objs []client.Object

	// ServiceMonitor is now only controlled globally
	if !featuregate.HasServiceMonitorEnabled(fg) {
		for _, resource := range expectResourceNames {
			objs = append(objs, &monitoringv1.ServiceMonitor{ObjectMeta: metav1.ObjectMeta{Name: resource, Namespace: namespace}})
		}
	}

	return objs
}
