package featuregate

import (
	"reflect"
	"testing"
)

func TestAllFeatures(t *testing.T) {
	expected := []string{
		"prometheus-operator-crds",
		"service-monitor",
		"prometheus-rule",
	}
	
	got := AllFeatures()
	if !reflect.DeepEqual(got, expected) {
		t.Errorf("AllFeatures() = %v, want %v", got, expected)
	}
}

func TestIsValidFeature(t *testing.T) {
	tests := []struct {
		name    string
		feature string
		want    bool
	}{
		{
			name:    "valid prometheus-operator-crds",
			feature: PrometheusOperatorCRDs,
			want:    true,
		},
		{
			name:    "valid service-monitor",
			feature: ServiceMonitor,
			want:    true,
		},
		{
			name:    "valid prometheus-rule",
			feature: PrometheusRule,
			want:    true,
		},
		{
			name:    "invalid feature",
			feature: "invalid-feature",
			want:    false,
		},
		{
			name:    "empty feature",
			feature: "",
			want:    false,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if got := IsValidFeature(tt.feature); got != tt.want {
				t.Errorf("IsValidFeature(%q) = %v, want %v", tt.feature, got, tt.want)
			}
		})
	}
}