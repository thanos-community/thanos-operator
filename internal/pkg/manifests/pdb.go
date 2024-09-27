package manifests

import (
	policyv1 "k8s.io/api/policy/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/util/intstr"
)

// NewPodDisruptionBudget creates a new PodDisruptionBudget object.
// It sets the object name, namespace, selector labels, object meta labels, and maxUnavailable.
// The maxUnavailable is a pointer to an int32 value.
// If the maxUnavailable is nil, it defaults to 1.
func NewPodDisruptionBudget(name, namespace string, selectorLabels, objectMetaLabels map[string]string, maxUnavailable *int) *policyv1.PodDisruptionBudget {
	value := 1
	if maxUnavailable != nil {
		value = *maxUnavailable
	}
	mu := intstr.FromInt32(int32(value))
	return &policyv1.PodDisruptionBudget{
		TypeMeta: metav1.TypeMeta{
			Kind:       "PodDisruptionBudget",
			APIVersion: policyv1.SchemeGroupVersion.String(),
		},
		ObjectMeta: metav1.ObjectMeta{
			Labels:    objectMetaLabels,
			Name:      name,
			Namespace: namespace,
		},
		Spec: policyv1.PodDisruptionBudgetSpec{
			Selector: &metav1.LabelSelector{
				MatchLabels: selectorLabels,
			},
			MaxUnavailable: &mu,
		},
	}
}
