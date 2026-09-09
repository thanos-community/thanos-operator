package manifests

import (
	"testing"

	"github.com/thanos-community/thanos-operator/internal/pkg/featuregate"

	"gotest.tools/v3/golden"
	"sigs.k8s.io/yaml"
)

func TestBuildServiceMonitor(t *testing.T) {
	const (
		name = "thanos-stack"
		ns   = "ns"
	)

	randObjMeta := map[string]string{
		"some-random-label": "some-random",
	}
	randSelectorLabels := map[string]string{
		"some-random-selector-label": "some-random",
	}

	for _, tc := range []struct {
		name   string
		golden string
		config featuregate.ServiceMonitorConfig
	}{
		{
			name:   "test service monitor correctness with defaults",
			golden: "servicemonitor-basic.golden.yaml",
			config: featuregate.ServiceMonitorConfig{},
		},
		{
			name:   "test service monitor with custom interval",
			golden: "servicemonitor-custom-interval.golden.yaml",
			config: featuregate.ServiceMonitorConfig{
				Interval: "60s",
			},
		},
	} {
		t.Run(tc.name, func(t *testing.T) {
			sm := BuildServiceMonitor(name, ns, randObjMeta, randSelectorLabels, tc.config, "http")

			// Test against golden file
			yamlBytes, err := yaml.Marshal(sm)
			if err != nil {
				t.Fatalf("failed to marshal ServiceMonitor to YAML: %v", err)
			}
			golden.Assert(t, string(yamlBytes), tc.golden)
		})
	}
}
