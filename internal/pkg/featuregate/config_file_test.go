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
