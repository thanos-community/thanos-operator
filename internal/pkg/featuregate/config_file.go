package featuregate

import (
	"encoding/json"
	"fmt"
	"os"

	"sigs.k8s.io/yaml"
)

const DefaultConfigFilePath = "/etc/thanos-operator/feature-gates.yaml"

// FileConfig represents the on-disk YAML schema for feature gate configuration.
// It carries only per-feature settings (not enablement); enablement is CLI-only.
// All fields are optional.
type FileConfig struct {
	KubeResourceSync *KubeResourceSyncFileConfig `json:"kube-resource-sync,omitempty"`
}

// KubeResourceSyncFileConfig carries settings for the kube-resource-sync feature.
type KubeResourceSyncFileConfig struct {
	Image string `json:"image,omitempty"`
}

// LoadFileConfig reads and parses a YAML config file, decoding only the blocks
// for features that are enabled in `enabled`. A missing file returns a zero-value
// FileConfig and no error. Blocks for disabled features are never decoded, so
// invalid content there will not cause an error.
func LoadFileConfig(path string, enabled Config) (FileConfig, error) {
	content, err := os.ReadFile(path)
	if err != nil {
		if os.IsNotExist(err) {
			return FileConfig{}, nil
		}
		return FileConfig{}, fmt.Errorf("failed to read feature gate config file %q: %w", path, err)
	}

	// Convert YAML to JSON for semantic parsing
	jsonData, err := yaml.YAMLToJSON(content)
	if err != nil {
		return FileConfig{}, fmt.Errorf("failed to parse feature gate config file %q: %w", path, err)
	}

	// Unmarshal into map[string]json.RawMessage to selectively decode only enabled features
	var rawConfig map[string]json.RawMessage
	if err := json.Unmarshal(jsonData, &rawConfig); err != nil {
		return FileConfig{}, fmt.Errorf("failed to decode feature gate config file %q: %w", path, err)
	}

	cfg := FileConfig{}

	// Only decode kube-resource-sync block if the feature is enabled
	if enabled.KubeResourceSyncEnabled() {
		if raw, exists := rawConfig["kube-resource-sync"]; exists {
			var krsConfig KubeResourceSyncFileConfig
			if err := json.Unmarshal(raw, &krsConfig); err != nil {
				return FileConfig{}, fmt.Errorf("failed to decode kube-resource-sync config in %q: %w", path, err)
			}
			cfg.KubeResourceSync = &krsConfig
		}
	}

	return cfg, nil
}

// ApplyFileConfig overlays settings from fc onto c for features that are already enabled.
// Returns the updated Config. Never changes which features are enabled.
func (c Config) ApplyFileConfig(fc FileConfig) Config {
	if c.KubeResourceSyncEnabled() && fc.KubeResourceSync != nil && fc.KubeResourceSync.Image != "" {
		c.KubeResourceSync.Image = fc.KubeResourceSync.Image
	}
	return c
}
