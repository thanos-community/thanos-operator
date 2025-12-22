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
)

// AllFeatures returns a slice of all available feature flag names.
// This is useful for validation and help text generation.
func AllFeatures() []string {
	return []string{
		ServiceMonitor,
		PrometheusRule,
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
}

// ServiceMonitorEnabled returns true if ServiceMonitor management is enabled.
func (c Config) ServiceMonitorEnabled() bool {
	return c.EnableServiceMonitor
}

// PrometheusRuleEnabled returns true if PrometheusRule discovery is enabled.
func (c Config) PrometheusRuleEnabled() bool {
	return c.EnablePrometheusRuleDiscovery
}

// ToFeatureGate converts a Flag to a Config struct for use by controllers.
func (f *Flag) ToFeatureGate() Config {
	return Config{
		EnableServiceMonitor:          f.EnablesServiceMonitor(),
		EnablePrometheusRuleDiscovery: f.EnablesPrometheusRule(),
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

// HasServiceMonitorEnabled checks if ServiceMonitor is enabled using global feature gate.
func HasServiceMonitorEnabled(fg Config) bool {
	return fg.ServiceMonitorEnabled()
}

// HasPrometheusRuleEnabled checks if PrometheusRule is enabled using global feature gate.
func HasPrometheusRuleEnabled(fg Config) bool {
	return fg.PrometheusRuleEnabled()
}
