package featuregate

import (
	"encoding/json"
	"fmt"
	"os"

	"github.com/prometheus/common/model"
	"k8s.io/apimachinery/pkg/util/validation"
	"sigs.k8s.io/yaml"
)

const DefaultConfigFilePath = "/etc/thanos-operator/feature-gates.yaml"

type serviceMonitorFileConfig struct {
	AdditionalLabels map[string]string `json:"additionalLabels,omitempty"`
	Interval         string            `json:"interval,omitempty"`
}

func (c serviceMonitorFileConfig) validate() error {
	if c.Interval != "" {
		interval, err := model.ParseDuration(c.Interval)
		if err != nil || interval <= 0 {
			return fmt.Errorf("interval %q must be a positive Prometheus duration", c.Interval)
		}
	}
	for key, value := range c.AdditionalLabels {
		if errs := validation.IsQualifiedName(key); len(errs) > 0 {
			return fmt.Errorf("invalid label key %q: %v", key, errs)
		}
		if errs := validation.IsValidLabelValue(value); len(errs) > 0 {
			return fmt.Errorf("invalid value for label %q: %v", key, errs)
		}
	}
	return nil
}

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

	if current.ServiceMonitorEnabled() {
		if raw, exists := rawConfig[ServiceMonitor]; exists {
			var smConfig serviceMonitorFileConfig
			if err := json.Unmarshal(raw, &smConfig); err != nil {
				return current, fmt.Errorf("failed to decode service-monitor config in %q: %w", path, err)
			}
			if err := smConfig.validate(); err != nil {
				return current, fmt.Errorf("invalid service-monitor config in %q: %w", path, err)
			}
			if smConfig.AdditionalLabels != nil {
				current.ServiceMonitor.AdditionalLabels = smConfig.AdditionalLabels
			}
			if smConfig.Interval != "" {
				current.ServiceMonitor.Interval = smConfig.Interval
			}
		}
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
