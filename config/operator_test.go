package config

import (
	"testing"

	"gotest.tools/v3/golden"
	"sigs.k8s.io/yaml"
)

func TestControllerManagerDeployment(t *testing.T) {
	for _, tc := range []struct {
		name   string
		golden string
		opts   []DeploymentOption
	}{
		{
			name:   "basic deployment with no options",
			golden: "deployment-basic.golden.yaml",
			opts:   []DeploymentOption{},
		},
		{
			name:   "deployment with auth proxy",
			golden: "deployment-auth-proxy.golden.yaml",
			opts:   []DeploymentOption{WithAuthProxy()},
		},
		{
			name:   "deployment with service monitor",
			golden: "deployment-service-monitor.golden.yaml",
			opts:   []DeploymentOption{WithServiceMonitor()},
		},
		{
			name:   "deployment with prometheus rule",
			golden: "deployment-prometheus-rule.golden.yaml",
			opts:   []DeploymentOption{WithPrometheusRule()},
		},
		{
			name:   "deployment with multiple features",
			golden: "deployment-multiple-features.golden.yaml",
			opts:   []DeploymentOption{WithServiceMonitor(), WithPrometheusRule()},
		},
		{
			name:   "deployment with auth proxy and features",
			golden: "deployment-auth-proxy-features.golden.yaml",
			opts:   []DeploymentOption{WithAuthProxy(), WithServiceMonitor(), WithPrometheusRule()},
		},
		{
			name:   "deployment with features using WithFeatures helper",
			golden: "deployment-with-features-helper.golden.yaml",
			opts:   []DeploymentOption{WithFeatures("service-monitor", "prometheus-rule")},
		},
	} {
		t.Run(tc.name, func(t *testing.T) {
			deployment := ControllerManagerDeployment(tc.opts...)

			// Test against golden file
			yamlBytes, err := yaml.Marshal(deployment)
			if err != nil {
				t.Fatalf("failed to marshal deployment to YAML: %v", err)
			}
			golden.Assert(t, string(yamlBytes), tc.golden)
		})
	}
}

func TestManagerServiceAccount(t *testing.T) {
	serviceAccount := ManagerServiceAccount()

	yamlBytes, err := yaml.Marshal(serviceAccount)
	if err != nil {
		t.Fatalf("failed to marshal serviceaccount to YAML: %v", err)
	}
	golden.Assert(t, string(yamlBytes), "serviceaccount-basic.golden.yaml")
}

func TestControllerManagerServiceMonitor(t *testing.T) {
	serviceMonitor := ControllerManagerServiceMonitor()

	yamlBytes, err := yaml.Marshal(serviceMonitor)
	if err != nil {
		t.Fatalf("failed to marshal servicemonitor to YAML: %v", err)
	}
	golden.Assert(t, string(yamlBytes), "servicemonitor-basic.golden.yaml")
}

func TestControllerManagerNamespace(t *testing.T) {
	namespace := ControllerManagerNamespace()

	yamlBytes, err := yaml.Marshal(namespace)
	if err != nil {
		t.Fatalf("failed to marshal namespace to YAML: %v", err)
	}
	golden.Assert(t, string(yamlBytes), "namespace-basic.golden.yaml")
}
