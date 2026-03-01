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

	// DefaultStoreAPILabel is used to identify StoreAPIs and will be set on the resources created by the operator.
	DefaultStoreAPILabel = "operator.thanos.io/store-api"
	DefaultStoreAPIValue = "true"

	// DefaultQueryAPILabel is used to identify Query APIs and will be set on the resources created by the operator.
	DefaultQueryAPILabel = "operator.thanos.io/query-api"
	DefaultQueryAPIValue = "true"

	// DefaultPrometheusRuleLabel is the default label key for PrometheusRule CRDs
	DefaultPrometheusRuleLabel = "operator.thanos.io/prometheus-rule"
	// DefaultPrometheusRuleValue is the default label value for PrometheusRule CRDs
	DefaultPrometheusRuleValue = "true"

	// PromRuleDerivedConfigMapLabel is the label key for PrometheusRule-derived ConfigMaps
	PromRuleDerivedConfigMapLabel = "operator.thanos.io/prometheus-rule-derived-configmap"
	// PromRuleDerivedConfigMapValue is the label value for PrometheusRule-derived ConfigMaps
	PromRuleDerivedConfigMapValue = "true"

	// UserConfigMapSourceLabel is the label key for user-provided ConfigMap-derived bucketed ConfigMaps
	UserConfigMapSourceLabel = "operator.thanos.io/user-configmap-source"
	// UserConfigMapSourceValue is the label value for user-provided ConfigMap-derived bucketed ConfigMaps
	UserConfigMapSourceValue = "true"

	// OwnerLabel is the label used to identify the owner of the object.
	// This relates to the CustomResource or entity that created the object.
	OwnerLabel = "operator.thanos.io/owner"

	HashringLabel = "operator.thanos.io/hashring"
	ShardLabel    = "operator.thanos.io/shard"
)

// MergeMaps merges the provided labels with the default labels for a component.
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
		maps.Copy(ls.MatchLabels, requiredLabels)
	}
	return metav1.LabelSelectorAsSelector(ls)
}

// EndpointType is the type of endpoint flag to be used for attaching the StoreAPI to Querier.
type EndpointType string

const (
	RegularLabel     EndpointType = "operator.thanos.io/endpoint"
	StrictLabel      EndpointType = "operator.thanos.io/endpoint-strict"
	GroupLabel       EndpointType = "operator.thanos.io/endpoint-group"
	GroupStrictLabel EndpointType = "operator.thanos.io/endpoint-group-strict"
)

// SanitizeStoreAPIEndpointLabels ensures StoreAPI has only a single type of endpoint label.
// If multiple endpoint types are set, the function will remove the conflicting labels based on priority.
func SanitizeStoreAPIEndpointLabels(baseLabels map[string]string) map[string]string {
	priorityOrder := []EndpointType{
		GroupStrictLabel,
		GroupLabel,
		StrictLabel,
		RegularLabel,
	}

	var foundLabel EndpointType
	for _, label := range priorityOrder {
		if _, exists := baseLabels[string(label)]; exists {
			foundLabel = label
			break
		}
	}

	for _, label := range priorityOrder {
		if label != foundLabel {
			delete(baseLabels, string(label))
		}
	}

	return baseLabels
}
