package controller

import (
	monitoringv1 "github.com/prometheus-operator/prometheus-operator/pkg/apis/monitoring/v1"

	"github.com/thanos-community/thanos-operator/api/v1alpha1"
	"github.com/thanos-community/thanos-operator/internal/pkg/featuregate"

	v1 "k8s.io/api/policy/v1"
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

// getDisabledFeatureGatedResources returns resources that should be deleted based on both global and per-resource feature gates.
func getDisabledFeatureGatedResources(globalFG featuregate.Config, resourceFG *v1alpha1.FeatureGates, expectResourceNames []string, namespace string) []client.Object {
	var objs []client.Object

	// ServiceMonitor is now only controlled globally
	if !featuregate.HasServiceMonitorEnabled(globalFG) {
		for _, resource := range expectResourceNames {
			objs = append(objs, &monitoringv1.ServiceMonitor{ObjectMeta: metav1.ObjectMeta{Name: resource, Namespace: namespace}})
		}
	}

	// PodDisruptionBudget can be controlled by both global and per-resource feature gates
	// Per-resource setting takes precedence over global setting
	pdbEnabled := featuregate.HasPodDisruptionBudgetEnabled(globalFG) // Start with global setting
	if resourceFG != nil && resourceFG.PodDisruptionBudgetConfig != nil && resourceFG.PodDisruptionBudgetConfig.Enable != nil {
		pdbEnabled = *resourceFG.PodDisruptionBudgetConfig.Enable // Override with per-resource setting
	}

	if !pdbEnabled {
		for _, resource := range expectResourceNames {
			objs = append(objs, &v1.PodDisruptionBudget{ObjectMeta: metav1.ObjectMeta{Name: resource, Namespace: namespace}})
		}
	}

	return objs
}
