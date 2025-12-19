package featuregate

import (
	"testing"
)

func TestFlag_Set(t *testing.T) {
	tests := []struct {
		name        string
		feature     string
		wantErr     bool
		errContains string
	}{
		{
			name:    "valid service-monitor",
			feature: ServiceMonitor,
			wantErr: false,
		},
		{
			name:    "valid prometheus-rule",
			feature: PrometheusRule,
			wantErr: false,
		},
		{
			name:        "invalid feature",
			feature:     "invalid-feature",
			wantErr:     true,
			errContains: "unknown feature",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			f := &Flag{}
			err := f.Set(tt.feature)

			if tt.wantErr {
				if err == nil {
					t.Errorf("Flag.Set(%q) expected error but got none", tt.feature)
					return
				}
				if tt.errContains != "" && !containsString(err.Error(), tt.errContains) {
					t.Errorf("Flag.Set(%q) error %q should contain %q", tt.feature, err.Error(), tt.errContains)
				}
			} else {
				if err != nil {
					t.Errorf("Flag.Set(%q) unexpected error: %v", tt.feature, err)
					return
				}
				if !f.Contains(tt.feature) {
					t.Errorf("Flag.Set(%q) feature not found in flag after setting", tt.feature)
				}
			}
		})
	}
}

func TestFlag_EnablesServiceMonitor(t *testing.T) {
	tests := []struct {
		name     string
		features []string
		want     bool
	}{
		{
			name:     "no features",
			features: []string{},
			want:     false,
		},
		{
			name:     "service-monitor enables service monitor",
			features: []string{ServiceMonitor},
			want:     true,
		},
		{
			name:     "prometheus-rule does not enable service monitor",
			features: []string{PrometheusRule},
			want:     false,
		},
		{
			name:     "multiple features including service-monitor",
			features: []string{ServiceMonitor, PrometheusRule},
			want:     true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			f := &Flag{}
			for _, feature := range tt.features {
				_ = f.Set(feature)
			}

			if got := f.EnablesServiceMonitor(); got != tt.want {
				t.Errorf("Flag.EnablesServiceMonitor() = %v, want %v", got, tt.want)
			}
		})
	}
}

func TestFlag_EnablesPrometheusRule(t *testing.T) {
	tests := []struct {
		name     string
		features []string
		want     bool
	}{
		{
			name:     "no features",
			features: []string{},
			want:     false,
		},
		{
			name:     "prometheus-rule enables prometheus rule",
			features: []string{PrometheusRule},
			want:     true,
		},
		{
			name:     "service-monitor does not enable prometheus rule",
			features: []string{ServiceMonitor},
			want:     false,
		},
		{
			name:     "multiple features including prometheus-rule",
			features: []string{ServiceMonitor, PrometheusRule},
			want:     true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			f := &Flag{}
			for _, feature := range tt.features {
				_ = f.Set(feature)
			}

			if got := f.EnablesPrometheusRule(); got != tt.want {
				t.Errorf("Flag.EnablesPrometheusRule() = %v, want %v", got, tt.want)
			}
		})
	}
}

func TestFlag_String(t *testing.T) {
	f := &Flag{}
	_ = f.Set(ServiceMonitor)
	_ = f.Set(PrometheusRule)

	got := f.String()
	expected := "service-monitor,prometheus-rule"

	if got != expected {
		t.Errorf("Flag.String() = %q, want %q", got, expected)
	}
}

func containsString(s, substr string) bool {
	return len(s) >= len(substr) && s[len(s)-len(substr):] == substr ||
		len(s) >= len(substr) && s[:len(substr)] == substr ||
		(len(s) > len(substr) && containsStringHelper(s, substr))
}

func containsStringHelper(s, substr string) bool {
	for i := 0; i <= len(s)-len(substr); i++ {
		if s[i:i+len(substr)] == substr {
			return true
		}
	}
	return false
}
