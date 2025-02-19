package controller

import (
	monitoringv1 "github.com/prometheus-operator/prometheus-operator/pkg/apis/monitoring/v1"
	"github.com/thanos-community/thanos-operator/api/v1alpha1"
	"github.com/thanos-community/thanos-operator/internal/pkg/manifests"

	policyv1 "k8s.io/api/policy/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"

	"sigs.k8s.io/controller-runtime/pkg/client"
)

func getFeatureGatedObjectsToDelete(obj manifests.Buildable, fg *v1alpha1.FeatureGates) []client.Object {
	var objs []client.Object
	if !manifests.HasServiceMonitorEnabled(fg) {
		objs = append(objs, &monitoringv1.ServiceMonitor{
			ObjectMeta: metav1.ObjectMeta{
				Name:      obj.GetGeneratedResourceName(),
				Namespace: obj.GetNamespace(),
			},
		})
	}
	if !manifests.HasPodDisruptionBudgetEnabled(fg) {
		objs = append(objs, &policyv1.PodDisruptionBudget{
			ObjectMeta: metav1.ObjectMeta{
				Name:      obj.GetGeneratedResourceName(),
				Namespace: obj.GetNamespace(),
			},
		})
	}
	return objs
}
