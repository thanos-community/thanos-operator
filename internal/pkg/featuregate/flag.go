package featuregate

import (
	"fmt"
	"strings"
)

// Flag implements flag.Value for repeatable feature flags.
// It validates feature names and provides convenience methods for checking enabled features.
type Flag []string

// String returns a comma-separated list of enabled features.
func (f *Flag) String() string {
	return strings.Join(*f, ",")
}

// Set adds a feature to the enabled features list after validating it.
func (f *Flag) Set(value string) error {
	if !IsValidFeature(value) {
		return fmt.Errorf("unknown feature %q, available features: %s", value, strings.Join(AllFeatures(), ", "))
	}
	*f = append(*f, value)
	return nil
}

// Contains checks if a specific feature is enabled.
func (f *Flag) Contains(feature string) bool {
	for _, v := range *f {
		if v == feature {
			return true
		}
	}
	return false
}

// EnablesServiceMonitor returns true if ServiceMonitor features should be enabled.
func (f *Flag) EnablesServiceMonitor() bool {
	return f.Contains(ServiceMonitor)
}

// EnablesPrometheusRule returns true if PrometheusRule features should be enabled.
func (f *Flag) EnablesPrometheusRule() bool {
	return f.Contains(PrometheusRule)
}
