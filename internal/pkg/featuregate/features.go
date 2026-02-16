package featuregate

import (
	"k8s.io/apimachinery/pkg/runtime/schema"
)

// Feature flag names for use with --enable-feature flag.
// These follow Prometheus convention of kebab-case feature names.
const (
	// ServiceMonitor enables management of ServiceMonitor objects.
	// See https://prometheus-operator.dev/docs/api-reference/api/#monitoring.coreos.com/v1.ServiceMonitor
	ServiceMonitor = "service-monitor"

	// PrometheusRule enables discovery of PrometheusRule objects to set on Thanos Ruler.
	// See https://prometheus-operator.dev/docs/api-reference/api/#monitoring.coreos.com/v1.PrometheusRule
	PrometheusRule = "prometheus-rule"

	// KubeResourceSync enables the kube-resource-sync sidecar for immediate ConfigMap/Secret synchronization.
	// See https://github.com/philipgough/kube-resource-sync
	KubeResourceSync = "kube-resource-sync"
)

// AllFeatures returns a slice of all available feature flag names.
// This is useful for validation and help text generation.
func AllFeatures() []string {
	return []string{
		ServiceMonitor,
		PrometheusRule,
		KubeResourceSync,
	}
}

// IsValidFeature checks if a given feature name is valid.
func IsValidFeature(feature string) bool {
	for _, f := range AllFeatures() {
		if f == feature {
			return true
		}
	}
	return false
}

// Config holds information about globally enabled features.
// This represents the actual feature state used by controllers and manifest builders.
type Config struct {
	// EnableServiceMonitor enables the management of ServiceMonitor objects.
	EnableServiceMonitor bool
	// EnablePrometheusRuleDiscovery enables the discovery of PrometheusRule objects.
	EnablePrometheusRuleDiscovery bool
	// EnableKubeResourceSync enables the kube-resource-sync sidecar container.
	EnableKubeResourceSync bool
	// KubeResourceSyncImage specifies the image to use for the kube-resource-sync sidecar.
	KubeResourceSyncImage string
}

// ServiceMonitorEnabled returns true if ServiceMonitor management is enabled.
func (c Config) ServiceMonitorEnabled() bool {
	return c.EnableServiceMonitor
}

// PrometheusRuleEnabled returns true if PrometheusRule discovery is enabled.
func (c Config) PrometheusRuleEnabled() bool {
	return c.EnablePrometheusRuleDiscovery
}

// KubeResourceSyncEnabled returns true if KubeResourceSync sidecar is enabled.
func (c Config) KubeResourceSyncEnabled() bool {
	return c.EnableKubeResourceSync
}

// GetKubeResourceSyncImage returns the image used for the kube-resource-sync sidecar.
func (c Config) GetKubeResourceSyncImage() string {
	return c.KubeResourceSyncImage
}

// ToFeatureGate converts a Flag to a Config struct for use by controllers.
func (f *Flag) ToFeatureGate() Config {
	return Config{
		EnableServiceMonitor:          f.EnablesServiceMonitor(),
		EnablePrometheusRuleDiscovery: f.EnablesPrometheusRule(),
		EnableKubeResourceSync:        f.EnablesKubeResourceSync(),
	}
}

// ToGVK returns the GroupVersionKind for all enabled features.
func (c Config) ToGVK() []schema.GroupVersionKind {
	var gvk []schema.GroupVersionKind
	if !c.EnableServiceMonitor {
		gvk = append(gvk, schema.GroupVersionKind{
			Group:   "monitoring.coreos.com",
			Version: "v1",
			Kind:    "ServiceMonitor",
		})
	}
	if !c.EnablePrometheusRuleDiscovery {
		gvk = append(gvk, schema.GroupVersionKind{
			Group:   "monitoring.coreos.com",
			Version: "v1",
			Kind:    "PrometheusRule",
		})
	}
	return gvk
}
