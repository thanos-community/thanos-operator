package controller

import (
	monitoringv1 "github.com/prometheus-operator/prometheus-operator/pkg/apis/monitoring/v1"

	"github.com/thanos-community/thanos-operator/internal/pkg/featuregate"

	v1 "k8s.io/api/policy/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"

	"sigs.k8s.io/controller-runtime/pkg/client"
)

// getDisabledFeatureGatedResourcesGlobal returns resources that should be deleted when features are globally disabled.
func getDisabledFeatureGatedResourcesGlobal(fg featuregate.Config, expectResourceNames []string, namespace string) []client.Object {
	var objs []client.Object
	if !featuregate.HasServiceMonitorEnabled(fg) {
		for _, resource := range expectResourceNames {
			objs = append(objs, &monitoringv1.ServiceMonitor{ObjectMeta: metav1.ObjectMeta{Name: resource, Namespace: namespace}})
		}
	}

	if !featuregate.HasPodDisruptionBudgetEnabled(fg) {
		for _, resource := range expectResourceNames {
			objs = append(objs, &v1.PodDisruptionBudget{ObjectMeta: metav1.ObjectMeta{Name: resource, Namespace: namespace}})
		}
	}
	return objs
}
