package receive

import (
	"testing"

	"github.com/thanos-community/thanos-operator/internal/pkg/manifests"
	"github.com/thanos-community/thanos-operator/test/utils"

	corev1 "k8s.io/api/core/v1"
	"k8s.io/utils/ptr"

	"gotest.tools/v3/golden"
	"sigs.k8s.io/controller-runtime/pkg/client"
	"sigs.k8s.io/yaml"
)

const (
	someCustomLabelValue string = "xyz"
	someOtherLabelValue  string = "abc"
)

func TestBuildIngesters(t *testing.T) {
	opts := IngesterOptions{
		Options: manifests.Options{
			Owner:     "any",
			Namespace: "ns",
			Image:     ptr.To("some-custom-image"),
			Labels: map[string]string{
				"some-custom-label":      someCustomLabelValue,
				"some-other-label":       someOtherLabelValue,
				"app.kubernetes.io/name": "expect-to-be-discarded",
			},
			PodDisruptionConfig: &manifests.PodDisruptionBudgetOptions{},
		},
		HashringName: "test-hashring",
	}

	objs := opts.Build()
	if len(objs) != 4 {
		t.Fatalf("expected 4 objects, got %d", len(objs))
	}

	utils.ValidateIsNamedServiceAccount(t, objs[0], opts, opts.Namespace)
	utils.ValidateObjectsEqual(t, objs[1], NewIngestorService(opts))
	utils.ValidateObjectsEqual(t, objs[2], NewIngestorStatefulSet(opts))
	utils.ValidateIsNamedPodDisruptionBudget(t, objs[3], opts, opts.Namespace, objs[1])

	wantLabels := opts.GetSelectorLabels()
	wantLabels["some-custom-label"] = someCustomLabelValue
	wantLabels["some-other-label"] = someOtherLabelValue
	utils.ValidateObjectLabelsEqual(t, wantLabels, []client.Object{objs[1], objs[2]}...)
}

func TestBuildRouter(t *testing.T) {
	opts := RouterOptions{
		Options: manifests.Options{
			Owner:     "any",
			Namespace: "ns",
			Image:     ptr.To("some-custom-image"),
			Labels: map[string]string{
				"some-custom-label":      someCustomLabelValue,
				"some-other-label":       someOtherLabelValue,
				"app.kubernetes.io/name": "expect-to-be-discarded",
			},
			PodDisruptionConfig: &manifests.PodDisruptionBudgetOptions{},
		},
	}

	objs := opts.Build()
	if len(objs) != 5 {
		t.Fatalf("expected 5 objects, got %d", len(objs))
	}

	utils.ValidateIsNamedServiceAccount(t, objs[0], opts, opts.Namespace)
	utils.ValidateObjectsEqual(t, objs[1], NewRouterService(opts))
	utils.ValidateObjectsEqual(t, objs[2], NewRouterDeployment(opts))
	utils.ValidateIsNamedPodDisruptionBudget(t, objs[4], opts, opts.Namespace, objs[2])

	wantLabels := GetRouterLabels(opts)
	wantLabels["some-custom-label"] = someCustomLabelValue
	wantLabels["some-other-label"] = someOtherLabelValue
	utils.ValidateObjectLabelsEqual(t, wantLabels, []client.Object{objs[1], objs[2]}...)
}

func TestNewIngestorStatefulSet(t *testing.T) {

	for _, tc := range []struct {
		name   string
		golden string
		opts   IngesterOptions
	}{
		{
			name:   "test ingester statefulset correctness",
			golden: "ingester-statefulset-basic.golden.yaml",
			opts: IngesterOptions{
				Options: manifests.Options{
					Namespace: "ns",
					Image:     ptr.To("some-custom-image"),
					Labels: map[string]string{
						"some-custom-label":      someCustomLabelValue,
						"some-other-label":       someOtherLabelValue,
						"app.kubernetes.io/name": "expect-to-be-discarded",
					},
					Annotations: map[string]string{
						"test": "annotation",
					},
				},
			},
		},
		{
			name:   "test additional volumemount",
			golden: "ingester-statefulset-with-volumemount.golden.yaml",
			opts: IngesterOptions{
				Options: manifests.Options{
					Namespace: "ns",
					Image:     ptr.To("some-custom-image"),
					Labels: map[string]string{
						"some-custom-label":      someCustomLabelValue,
						"some-other-label":       someOtherLabelValue,
						"app.kubernetes.io/name": "expect-to-be-discarded",
					},
					Annotations: map[string]string{
						"test": "annotation",
					},
					Additional: manifests.Additional{
						VolumeMounts: []corev1.VolumeMount{
							{
								Name:      "http-config",
								MountPath: "/http-config",
							},
						},
					},
				},
			},
		},
		{
			name:   "test additional container",
			golden: "ingester-statefulset-with-container.golden.yaml",
			opts: IngesterOptions{
				Options: manifests.Options{
					Namespace: "ns",
					Image:     ptr.To("some-custom-image"),
					Labels: map[string]string{
						"some-custom-label":      someCustomLabelValue,
						"some-other-label":       someOtherLabelValue,
						"app.kubernetes.io/name": "expect-to-be-discarded",
					},
					Annotations: map[string]string{
						"test": "annotation",
					},
					Additional: manifests.Additional{
						Containers: []corev1.Container{
							{
								Name:  "test-container",
								Image: "test-image:latest",
								Args:  []string{"--test-arg"},
								Env: []corev1.EnvVar{{
									Name:  "TEST_ENV",
									Value: "test",
								}},
							},
						},
					},
				},
			},
		},
	} {
		t.Run(tc.name, func(t *testing.T) {
			ingester := NewIngestorStatefulSet(tc.opts)

			// Test against golden file
			yamlBytes, err := yaml.Marshal(ingester)
			if err != nil {
				t.Fatalf("failed to marshal statefulset to YAML: %v", err)
			}
			golden.Assert(t, string(yamlBytes), tc.golden)
		})
	}
}

func TestNewRouterDeployment(t *testing.T) {

	for _, tc := range []struct {
		name   string
		golden string
		opts   RouterOptions
	}{
		{
			name:   "test router deployment correctness",
			golden: "router-deployment-basic.golden.yaml",
			opts: RouterOptions{
				Options: manifests.Options{
					Namespace: "ns",
					Image:     ptr.To("some-custom-image"),
					Labels: map[string]string{
						"some-custom-label":      someCustomLabelValue,
						"some-other-label":       someOtherLabelValue,
						"app.kubernetes.io/name": "expect-to-be-discarded",
					},
					Annotations: map[string]string{
						"test": "annotation",
					},
				},
			},
		},
		{
			name:   "test additional volumemount",
			golden: "router-deployment-with-volumemount.golden.yaml",
			opts: RouterOptions{
				Options: manifests.Options{
					Namespace: "ns",
					Image:     ptr.To("some-custom-image"),
					Labels: map[string]string{
						"some-custom-label":      someCustomLabelValue,
						"some-other-label":       someOtherLabelValue,
						"app.kubernetes.io/name": "expect-to-be-discarded",
					},
					Annotations: map[string]string{
						"test": "annotation",
					},
					Additional: manifests.Additional{
						VolumeMounts: []corev1.VolumeMount{
							{
								Name:      "http-config",
								MountPath: "/http-config",
							},
						},
					},
				},
			},
		},
		{
			name:   "test additional container",
			golden: "router-deployment-with-container.golden.yaml",
			opts: RouterOptions{
				Options: manifests.Options{
					Namespace: "ns",
					Image:     ptr.To("some-custom-image"),
					Labels: map[string]string{
						"some-custom-label":      someCustomLabelValue,
						"some-other-label":       someOtherLabelValue,
						"app.kubernetes.io/name": "expect-to-be-discarded",
					},
					Annotations: map[string]string{
						"test": "annotation",
					},
					Additional: manifests.Additional{
						Containers: []corev1.Container{
							{
								Name:  "test-container",
								Image: "test-image:latest",
								Args:  []string{"--test-arg"},
								Env: []corev1.EnvVar{{
									Name:  "TEST_ENV",
									Value: "test",
								}},
							},
						},
					},
				},
			},
		},
	} {
		t.Run(tc.name, func(t *testing.T) {
			router := NewRouterDeployment(tc.opts)

			// Test against golden file
			yamlBytes, err := yaml.Marshal(router)
			if err != nil {
				t.Fatalf("failed to marshal deployment to YAML: %v", err)
			}
			golden.Assert(t, string(yamlBytes), tc.golden)
		})
	}
}

func TestNewIngestorService(t *testing.T) {

	opts := IngesterOptions{
		Options: manifests.Options{
			Namespace: "ns",
			Labels: map[string]string{
				"some-custom-label":      someCustomLabelValue,
				"some-other-label":       someOtherLabelValue,
				"app.kubernetes.io/name": "expect-to-be-discarded",
			},
		},
	}

	for _, tc := range []struct {
		name   string
		golden string
		opts   IngesterOptions
	}{
		{
			name:   "test ingester service correctness",
			golden: "ingester-service-basic.golden.yaml",
			opts:   opts,
		},
	} {
		t.Run(tc.name, func(t *testing.T) {
			ingester := NewIngestorService(tc.opts)

			// Test against golden file
			yamlBytes, err := yaml.Marshal(ingester)
			if err != nil {
				t.Fatalf("failed to marshal service to YAML: %v", err)
			}
			golden.Assert(t, string(yamlBytes), tc.golden)
		})
	}
}

func TestNewRouterService(t *testing.T) {

	opts := RouterOptions{
		Options: manifests.Options{
			Namespace: "ns",
			Labels: map[string]string{
				"some-custom-label":      someCustomLabelValue,
				"some-other-label":       someOtherLabelValue,
				"app.kubernetes.io/name": "expect-to-be-discarded",
			},
		},
	}

	for _, tc := range []struct {
		name   string
		golden string
		opts   RouterOptions
	}{
		{
			name:   "test router service correctness",
			golden: "router-service-basic.golden.yaml",
			opts:   opts,
		},
	} {
		t.Run(tc.name, func(t *testing.T) {
			router := NewRouterService(tc.opts)

			// Test against golden file
			yamlBytes, err := yaml.Marshal(router)
			if err != nil {
				t.Fatalf("failed to marshal service to YAML: %v", err)
			}
			golden.Assert(t, string(yamlBytes), tc.golden)
		})
	}
}

// TestBuildRouterGolden demonstrates using golden files for router manifest testing
// This test shows how to validate the complete structure of generated manifests
// Run with -update to regenerate golden files
func TestBuildRouterGolden(t *testing.T) {
	opts := RouterOptions{
		Options: manifests.Options{
			Owner:     "test-owner",
			Namespace: "test-namespace",
			Image:     ptr.To("quay.io/thanos/thanos:v0.40.1"),
			Labels: map[string]string{
				"app.kubernetes.io/version": "v0.40.1",
			},
			PodDisruptionConfig: &manifests.PodDisruptionBudgetOptions{},
		},
		HashringConfig: `[{"hashring":"test","endpoints":["test:19291"]}]`,
	}

	objs := opts.Build()

	// Validate against golden file containing all router resources
	yamlBytes, err := yaml.Marshal(objs)
	if err != nil {
		t.Fatalf("failed to marshal objects to YAML: %v", err)
	}
	golden.Assert(t, string(yamlBytes), "router-complete.golden.yaml")
}
