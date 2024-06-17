package manifests

const (
	// The following labels are used to identify the components and will be set on the resources created by the operator.
	// These labels cannot be overridden by the user via additional labels configuration.
	// See https://kubernetes.io/docs/concepts/overview/working-with-objects/common-labels/#labels

	NameLabel      = "app.kubernetes.io/name"
	ComponentLabel = "app.kubernetes.io/component"
	PartOfLabel    = "app.kubernetes.io/part-of"
	ManagedByLabel = "app.kubernetes.io/managed-by"
	InstanceLabel  = "app.kubernetes.io/instance"

	DefaultPartOfLabel    = "thanos"
	DefaultManagedByLabel = "thanos-operator"
)

// MergeLabels merges the provided labels with the default labels for a component.
func MergeLabels(baseLabels map[string]string, mergeWithPriority map[string]string) map[string]string {
	if baseLabels == nil {
		return mergeWithPriority
	}

	for k, v := range mergeWithPriority {
		baseLabels[k] = v
	}
	return baseLabels
}
