package featuregate

import (
	"encoding/json"
	"fmt"
	"os"

	"sigs.k8s.io/yaml"
)

const DefaultConfigFilePath = "/etc/thanos-operator/feature-gates.yaml"

// LoadAndApplyConfig reads and applies a YAML config file on top of the current Config.
// A missing file returns the current Config unchanged and no error.
// Blocks for disabled features are never decoded, so invalid content there will not cause an error.
func LoadAndApplyConfig(path string, current Config) (Config, error) {
	content, err := os.ReadFile(path)
	if err != nil {
		if os.IsNotExist(err) {
			return current, nil
		}
		return current, fmt.Errorf("failed to read feature gate config file %q: %w", path, err)
	}

	// Convert YAML to JSON for semantic parsing
	jsonData, err := yaml.YAMLToJSON(content)
	if err != nil {
		return current, fmt.Errorf("failed to parse feature gate config file %q: %w", path, err)
	}

	// Unmarshal into map[string]json.RawMessage to selectively decode only enabled features
	var rawConfig map[string]json.RawMessage
	if err := json.Unmarshal(jsonData, &rawConfig); err != nil {
		return current, fmt.Errorf("failed to decode feature gate config file %q: %w", path, err)
	}

	// Only decode kube-resource-sync block if the feature is enabled
	if current.KubeResourceSyncEnabled() {
		if raw, exists := rawConfig["kube-resource-sync"]; exists {
			var krsConfig struct {
				Image string `json:"image,omitempty"`
			}
			if err := json.Unmarshal(raw, &krsConfig); err != nil {
				return current, fmt.Errorf("failed to decode kube-resource-sync config in %q: %w", path, err)
			}
			if krsConfig.Image != "" {
				current.KubeResourceSync.Image = krsConfig.Image
			}
		}
	}

	return current, nil
}
