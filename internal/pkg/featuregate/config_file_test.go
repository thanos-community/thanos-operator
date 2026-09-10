package featuregate

import (
	"os"
	"path/filepath"
	"testing"

	"gotest.tools/v3/assert"
)

func TestLoadKubeResourceSyncConfig(t *testing.T) {
	for _, tc := range []struct {
		name      string
		content   string
		wantImage string
	}{
		{
			name: "configured",
			content: `
kube-resource-sync:
  image: custom-image:v1.0
`,
			wantImage: "custom-image:v1.0",
		},
		{
			name: "defaults",
			content: `
kube-resource-sync: {}
`,
			wantImage: "quay.io/philipgough/kube-resource-sync:0.1.0",
		},
	} {
		t.Run(tc.name, func(t *testing.T) {
			path := filepath.Join(t.TempDir(), "feature-gates.yaml")
			assert.NilError(t, os.WriteFile(path, []byte(tc.content), 0600))
			base := Config{
				KubeResourceSync: &KubeResourceSyncConfig{
					FeatureConfig: FeatureConfig{Enabled: true},
				},
			}

			cfg, err := LoadAndApplyConfig(path, base)
			assert.NilError(t, err)
			assert.Assert(t, cfg.KubeResourceSyncEnabled())
			assert.Equal(t, cfg.KubeResourceSync.Image, tc.wantImage)
		})
	}
}

func TestLoadServiceMonitorConfig(t *testing.T) {
	for _, tc := range []struct {
		name         string
		content      string
		wantLabels   map[string]string
		wantInterval string
	}{
		{
			name: "configured",
			content: `
service-monitor:
  additionalLabels:
    prometheus: platform
  interval: 30s
`,
			wantLabels:   map[string]string{"prometheus": "platform"},
			wantInterval: "30s",
		},
		{
			name: "defaults",
			content: `
service-monitor: {}
`,
		},
	} {
		t.Run(tc.name, func(t *testing.T) {
			path := filepath.Join(t.TempDir(), "feature-gates.yaml")
			assert.NilError(t, os.WriteFile(path, []byte(tc.content), 0600))
			base := Config{
				ServiceMonitor: &ServiceMonitorConfig{FeatureConfig: FeatureConfig{Enabled: true}},
			}

			cfg, err := LoadAndApplyConfig(path, base)
			assert.NilError(t, err)
			assert.Assert(t, cfg.ServiceMonitorEnabled())
			assert.DeepEqual(t, cfg.ServiceMonitor.AdditionalLabels, tc.wantLabels)
			assert.Equal(t, cfg.ServiceMonitor.Interval, tc.wantInterval)
		})
	}
}
