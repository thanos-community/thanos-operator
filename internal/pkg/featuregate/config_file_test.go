package featuregate

import (
	"os"
	"path/filepath"
	"testing"

	"gotest.tools/v3/assert"
)

func TestLoadFileConfig(t *testing.T) {
	tests := []struct {
		name             string
		fileContent      string
		enabledFeatures  Config
		expectedImage    string
		expectError      bool
		errorContains    string
	}{
		{
			name:            "missing file returns empty config",
			fileContent:     "", // no file written
			enabledFeatures: Config{},
			expectedImage:   "",
			expectError:     false,
		},
		{
			name:            "empty file",
			fileContent:     "",
			enabledFeatures: Config{KubeResourceSync: &KubeResourceSyncConfig{FeatureConfig: FeatureConfig{Enabled: true}}},
			expectedImage:   "",
			expectError:     false,
		},
		{
			name:            "kube-resource-sync enabled, file sets image",
			fileContent:     "kube-resource-sync:\n  image: custom-image:v1.0",
			enabledFeatures: Config{KubeResourceSync: &KubeResourceSyncConfig{FeatureConfig: FeatureConfig{Enabled: true}}},
			expectedImage:   "custom-image:v1.0",
			expectError:     false,
		},
		{
			name:            "kube-resource-sync disabled, file block is ignored even if invalid structure",
			fileContent:     "kube-resource-sync:\n  unknownField: true\n  image: 123",
			enabledFeatures: Config{}, // feature not enabled
			expectedImage:   "",
			expectError:     false,
		},
		{
			name:            "kube-resource-sync enabled, file block with invalid field type returns error",
			fileContent:     "kube-resource-sync:\n  image: 123",
			enabledFeatures: Config{KubeResourceSync: &KubeResourceSyncConfig{FeatureConfig: FeatureConfig{Enabled: true}}},
			expectError:     true,
			errorContains:   "failed to decode kube-resource-sync config",
		},
		{
			name:            "kube-resource-sync enabled, file has empty block",
			fileContent:     "kube-resource-sync: {}",
			enabledFeatures: Config{KubeResourceSync: &KubeResourceSyncConfig{FeatureConfig: FeatureConfig{Enabled: true}}},
			expectedImage:   "",
			expectError:     false,
		},
		{
			name:            "file with multiple features, only enabled ones decoded",
			fileContent:     "kube-resource-sync:\n  image: my-image:v2\nservice-monitor:\n  someField: true",
			enabledFeatures: Config{KubeResourceSync: &KubeResourceSyncConfig{FeatureConfig: FeatureConfig{Enabled: true}}},
			expectedImage:   "my-image:v2",
			expectError:     false,
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

			cfg, err := LoadFileConfig(filePath, tt.enabledFeatures)

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

func TestApplyFileConfig(t *testing.T) {
	tests := []struct {
		name            string
		baseConfig      Config
		fileConfig      FileConfig
		expectedImage   string
		expectedEnabled bool
	}{
		{
			name:            "apply image to enabled kube-resource-sync",
			baseConfig:      Config{KubeResourceSync: &KubeResourceSyncConfig{FeatureConfig: FeatureConfig{Enabled: true}, Image: "default-image"}},
			fileConfig:      FileConfig{KubeResourceSync: &KubeResourceSyncFileConfig{Image: "file-image"}},
			expectedImage:   "file-image",
			expectedEnabled: true,
		},
		{
			name:            "apply image to enabled feature with empty default",
			baseConfig:      Config{KubeResourceSync: &KubeResourceSyncConfig{FeatureConfig: FeatureConfig{Enabled: true}, Image: ""}},
			fileConfig:      FileConfig{KubeResourceSync: &KubeResourceSyncFileConfig{Image: "file-image"}},
			expectedImage:   "file-image",
			expectedEnabled: true,
		},
		{
			name:            "empty file config leaves base unchanged",
			baseConfig:      Config{KubeResourceSync: &KubeResourceSyncConfig{FeatureConfig: FeatureConfig{Enabled: true}, Image: "original"}},
			fileConfig:      FileConfig{},
			expectedImage:   "original",
			expectedEnabled: true,
		},
		{
			name:            "file config with empty image string skipped",
			baseConfig:      Config{KubeResourceSync: &KubeResourceSyncConfig{FeatureConfig: FeatureConfig{Enabled: true}, Image: "original"}},
			fileConfig:      FileConfig{KubeResourceSync: &KubeResourceSyncFileConfig{Image: ""}},
			expectedImage:   "original",
			expectedEnabled: true,
		},
		{
			name:            "disabled feature block in file ignored",
			baseConfig:      Config{},
			fileConfig:      FileConfig{KubeResourceSync: &KubeResourceSyncFileConfig{Image: "file-image"}},
			expectedImage:   "",
			expectedEnabled: false,
		},
		{
			name:            "nil kube-resource-sync in base, file config not applied",
			baseConfig:      Config{},
			fileConfig:      FileConfig{KubeResourceSync: &KubeResourceSyncFileConfig{Image: "file-image"}},
			expectedImage:   "",
			expectedEnabled: false,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result := tt.baseConfig.ApplyFileConfig(tt.fileConfig)

			if tt.expectedEnabled {
				assert.Assert(t, result.KubeResourceSync != nil)
				assert.Equal(t, result.KubeResourceSync.Image, tt.expectedImage)
			} else {
				assert.Assert(t, result.KubeResourceSync == nil || !result.KubeResourceSyncEnabled())
			}
		})
	}
}
