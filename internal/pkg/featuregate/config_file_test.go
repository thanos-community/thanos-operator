package featuregate

import (
	"os"
	"path/filepath"
	"testing"

	"gotest.tools/v3/assert"
)

func TestLoadAndApplyConfig(t *testing.T) {
	tests := []struct {
		name          string
		fileContent   string
		baseConfig    Config
		expectedImage string
		expectError   bool
		errorContains string
	}{
		{
			name:          "missing file returns base config unchanged",
			fileContent:   "", // no file written
			baseConfig:    Config{KubeResourceSync: &KubeResourceSyncConfig{FeatureConfig: FeatureConfig{Enabled: true}, Image: "default"}},
			expectedImage: "default",
			expectError:   false,
		},
		{
			name:          "empty file applies to base unchanged",
			fileContent:   "",
			baseConfig:    Config{KubeResourceSync: &KubeResourceSyncConfig{FeatureConfig: FeatureConfig{Enabled: true}, Image: "default"}},
			expectedImage: "default",
			expectError:   false,
		},
		{
			name:          "kube-resource-sync enabled, file sets image",
			fileContent:   "kube-resource-sync:\n  image: custom-image:v1.0",
			baseConfig:    Config{KubeResourceSync: &KubeResourceSyncConfig{FeatureConfig: FeatureConfig{Enabled: true}, Image: "default"}},
			expectedImage: "custom-image:v1.0",
			expectError:   false,
		},
		{
			name:          "kube-resource-sync disabled, file block is ignored even if invalid structure",
			fileContent:   "kube-resource-sync:\n  unknownField: true\n  image: 123",
			baseConfig:    Config{}, // feature not enabled
			expectedImage: "",
			expectError:   false,
		},
		{
			name:          "kube-resource-sync enabled, file block with invalid field type returns error",
			fileContent:   "kube-resource-sync:\n  image: 123",
			baseConfig:    Config{KubeResourceSync: &KubeResourceSyncConfig{FeatureConfig: FeatureConfig{Enabled: true}}},
			expectError:   true,
			errorContains: "failed to decode kube-resource-sync config",
		},
		{
			name:          "kube-resource-sync enabled, file has empty block",
			fileContent:   "kube-resource-sync: {}",
			baseConfig:    Config{KubeResourceSync: &KubeResourceSyncConfig{FeatureConfig: FeatureConfig{Enabled: true}, Image: "default"}},
			expectedImage: "default",
			expectError:   false,
		},
		{
			name:          "file with multiple features, only enabled ones decoded",
			fileContent:   "kube-resource-sync:\n  image: my-image:v2\nservice-monitor:\n  someField: true",
			baseConfig:    Config{KubeResourceSync: &KubeResourceSyncConfig{FeatureConfig: FeatureConfig{Enabled: true}, Image: "default"}},
			expectedImage: "my-image:v2",
			expectError:   false,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			tmpDir := t.TempDir()
			var filePath string

			if tt.fileContent != "" {
				filePath = filepath.Join(tmpDir, "feature-gates.yaml")
				err := os.WriteFile(filePath, []byte(tt.fileContent), 0644)
				assert.NilError(t, err)
			} else {
				filePath = filepath.Join(tmpDir, "nonexistent.yaml")
			}

			cfg, err := LoadAndApplyConfig(filePath, tt.baseConfig)

			if tt.expectError {
				assert.ErrorContains(t, err, tt.errorContains)
			} else {
				assert.NilError(t, err)
				if tt.expectedImage != "" {
					assert.Assert(t, cfg.KubeResourceSync != nil)
					assert.Equal(t, cfg.KubeResourceSync.Image, tt.expectedImage)
				}
			}
		})
	}
}

func TestLoadServiceMonitorConfig(t *testing.T) {
	for _, tc := range []struct {
		name          string
		content       string
		disabled      bool
		missingFile   bool
		wantLabels    map[string]string
		wantInterval  string
		errorContains string
	}{
		{name: "missing file", missingFile: true},
		{name: "empty file"},
		{name: "missing block", content: "kube-resource-sync: {}"},
		{name: "empty block", content: "service-monitor: {}"},
		{name: "explicit empty settings", content: "service-monitor:\n  additionalLabels: {}\n  interval: ''", wantLabels: map[string]string{}},
		{name: "labels and interval", content: "service-monitor:\n  additionalLabels:\n    prometheus: platform\n  interval: 30s", wantLabels: map[string]string{"prometheus": "platform"}, wantInterval: "30s"},
		{name: "compound interval", content: "service-monitor:\n  interval: 1m30s", wantInterval: "1m30s"},
		{name: "enablement stays in flags", content: "service-monitor:\n  enabled: false\n  interval: 1m", wantInterval: "1m"},
		{name: "disabled block ignored", disabled: true, content: "service-monitor:\n  enabled: true\n  additionalLabels: wrong\n  interval: invalid"},
		{name: "invalid labels type", content: "service-monitor:\n  additionalLabels: wrong", errorContains: "failed to decode service-monitor config"},
		{name: "invalid label key", content: "service-monitor:\n  additionalLabels:\n    invalid/key/name: value", errorContains: "invalid label key"},
		{name: "invalid label value", content: "service-monitor:\n  additionalLabels:\n    prometheus: invalid value", errorContains: "invalid value for label"},
		{name: "invalid interval type", content: "service-monitor:\n  interval: 30", errorContains: "failed to decode service-monitor config"},
		{name: "invalid duration", content: "service-monitor:\n  interval: invalid", errorContains: "must be a positive Prometheus duration"},
		{name: "zero duration", content: "service-monitor:\n  interval: 0s", errorContains: "must be a positive Prometheus duration"},
		{name: "negative duration", content: "service-monitor:\n  interval: -1s", errorContains: "must be a positive Prometheus duration"},
	} {
		t.Run(tc.name, func(t *testing.T) {
			path := filepath.Join(t.TempDir(), "feature-gates.yaml")
			if !tc.missingFile {
				assert.NilError(t, os.WriteFile(path, []byte(tc.content), 0600))
			}
			base := Config{}
			if !tc.disabled {
				base.ServiceMonitor = &ServiceMonitorConfig{FeatureConfig: FeatureConfig{Enabled: true}}
			}
			cfg, err := LoadAndApplyConfig(path, base)
			if tc.errorContains != "" {
				assert.ErrorContains(t, err, tc.errorContains)
				return
			}
			assert.NilError(t, err)
			assert.Equal(t, cfg.ServiceMonitorEnabled(), !tc.disabled)
			if tc.disabled {
				assert.Assert(t, cfg.ServiceMonitor == nil)
				return
			}
			assert.DeepEqual(t, cfg.ServiceMonitor.AdditionalLabels, tc.wantLabels)
			assert.Equal(t, cfg.ServiceMonitor.Interval, tc.wantInterval)
		})
	}
}
