package manifests

import (
	policyv1 "k8s.io/api/policy/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/util/intstr"
	"k8s.io/utils/ptr"
)

// PodDisruptionBudgetOptions defines the available options for creating a PodDisruptionBudget object.
type PodDisruptionBudgetOptions struct {
	// MaxUnavailable is the maximum number of pods that can be unavailable during the disruption.
	// Defaults to 1 if not specified.
	MaxUnavailable *int32
	// MinAvailable is the minimum number of pods that must still be available during the disruption.
	// Defaults to nil if not specified.
	MinAvailable *int32
}

// NewPodDisruptionBudget creates a new PodDisruptionBudget object.
// It sets the object name, namespace, selector labels, object meta labels, and maxUnavailable.
// The maxUnavailable is a pointer to an int32 value.
// If the maxUnavailable is nil, it defaults to 1.
func NewPodDisruptionBudget(name, namespace string, selectorLabels, objectMetaLabels, annotations map[string]string, opts PodDisruptionBudgetOptions) *policyv1.PodDisruptionBudget {
	minValue, maxValue := opts.getMinAndMax()
	return &policyv1.PodDisruptionBudget{
		TypeMeta: metav1.TypeMeta{
			Kind:       "PodDisruptionBudget",
			APIVersion: policyv1.SchemeGroupVersion.String(),
		},
		ObjectMeta: metav1.ObjectMeta{
			Labels:      objectMetaLabels,
			Name:        name,
			Namespace:   namespace,
			Annotations: annotations,
		},
		Spec: policyv1.PodDisruptionBudgetSpec{
			Selector: &metav1.LabelSelector{
				MatchLabels: selectorLabels,
			},
			MinAvailable:   minValue,
			MaxUnavailable: maxValue,
		},
	}
}

func (opts PodDisruptionBudgetOptions) getMinAndMax() (min *intstr.IntOrString, max *intstr.IntOrString) {
	if opts.MinAvailable != nil {
		minValue := intstr.FromInt32(*opts.MinAvailable)
		min = &minValue
	}

	if opts.MaxUnavailable == nil {
		opts.MaxUnavailable = ptr.To(int32(1))
	}

	maxValue := intstr.FromInt32(*opts.MaxUnavailable)
	max = &maxValue
	return min, max
}
