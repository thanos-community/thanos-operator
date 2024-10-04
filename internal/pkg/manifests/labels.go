package manifests

import (
	"maps"

	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/labels"
)

const (
	// The following labels are used to identify the components and will be set on the resources created by the operator.
	// These labels cannot be overridden by the user via additional labels configuration.
	// This is ensured by updates to components.
	// See https://kubernetes.io/docs/concepts/overview/working-with-objects/common-labels/#labels

	NameLabel      = "app.kubernetes.io/name"
	ComponentLabel = "app.kubernetes.io/component"
	PartOfLabel    = "app.kubernetes.io/part-of"
	ManagedByLabel = "app.kubernetes.io/managed-by"
	InstanceLabel  = "app.kubernetes.io/instance"

	DefaultPartOfLabel    = "thanos"
	DefaultManagedByLabel = "thanos-operator"

	// The following label is used to identify StoreAPIs and will be set on the resources created by the operator.
	DefaultStoreAPILabel = "operator.thanos.io/store-api"
	DefaultStoreAPIValue = "true"

	// The following label is used to identify Query APIs and will be set on the resources created by the operator.
	DefaultQueryAPILabel = "operator.thanos.io/query-api"
	DefaultQueryAPIValue = "true"

	// The following label is used to identify Rule Configs and will be set on the resources created by the operator.
	DefaultRuleConfigLabel = "operator.thanos.io/rule-file"
	DefaultRuleConfigValue = "true"

	// OwnerLabel is the label used to identify the owner of the object.
	// This relates to the CustomResource or entity that created the object.
	OwnerLabel = "operator.thanos.io/owner"
)

// MergeLabels merges the provided labels with the default labels for a component.
// Returns a new map with the merged labels leaving the original maps unchanged.
func MergeLabels(baseLabels map[string]string, mergeWithPriority map[string]string) map[string]string {
	if baseLabels == nil {
		labelCopy := make(map[string]string, len(mergeWithPriority))
		maps.Copy(labelCopy, mergeWithPriority)
		return labelCopy
	}

	mergedLabels := make(map[string]string, len(baseLabels)+len(mergeWithPriority))
	maps.Copy(mergedLabels, baseLabels)
	maps.Copy(mergedLabels, mergeWithPriority)
	return mergedLabels
}

// BuildLabelSelectorFrom builds a label selector from the provided label selector and required labels.
// The required labels will be added to the MatchLabels of the provided label selector.
// labelSelector is DeepCopied to avoid modifying the original object.
func BuildLabelSelectorFrom(labelSelector *metav1.LabelSelector, requiredLabels map[string]string) (labels.Selector, error) {
	ls := labelSelector.DeepCopy()

	if ls == nil {
		ls = &metav1.LabelSelector{MatchLabels: requiredLabels}
	} else {
		for k, v := range requiredLabels {
			ls.MatchLabels[k] = v
		}
	}
	return metav1.LabelSelectorAsSelector(ls)
}
