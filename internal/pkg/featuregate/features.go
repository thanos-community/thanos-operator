package featuregate

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
