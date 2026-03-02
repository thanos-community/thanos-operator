package query

import (
	"testing"

	"gotest.tools/v3/assert"

	"github.com/thanos-community/thanos-operator/internal/pkg/manifests"
	"github.com/thanos-community/thanos-operator/test/utils"

	"gotest.tools/v3/golden"

	corev1 "k8s.io/api/core/v1"
	"k8s.io/utils/ptr"

	"sigs.k8s.io/controller-runtime/pkg/client"
	"sigs.k8s.io/yaml"
)

const (
	someCustomLabelValue string = "xyz"
	someOtherLabelValue  string = "abc"
)

func TestBuildQuery(t *testing.T) {
	opts := Options{
		Options: manifests.Options{
			Owner:     "any",
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
			PodDisruptionConfig: &manifests.PodDisruptionBudgetOptions{},
		},
		Timeout:       "15m",
		LookbackDelta: "5m",
		MaxConcurrent: 20,
	}

	objs := opts.Build()
	if len(objs) != 4 {
		t.Fatalf("expected 4 objects, got %d", len(objs))
	}

	for _, obj := range objs {
		assert.Assert(t, obj.GetAnnotations()["test"] == "annotation")
	}

	utils.ValidateIsNamedServiceAccount(t, objs[0], opts, opts.Namespace)
	utils.ValidateObjectsEqual(t, objs[1], NewQueryDeployment(opts))
	utils.ValidateObjectsEqual(t, objs[2], NewQueryService(opts))
	utils.ValidateIsNamedPodDisruptionBudget(t, objs[3], opts, opts.Namespace, objs[1])

	wantLabels := opts.GetSelectorLabels()
	wantLabels["some-custom-label"] = someCustomLabelValue
	wantLabels["some-other-label"] = someOtherLabelValue
	utils.ValidateObjectLabelsEqual(t, wantLabels, []client.Object{objs[1], objs[2]}...)
}

func TestNewQueryDeployment(t *testing.T) {

	for _, tc := range []struct {
		name   string
		golden string
		opts   Options
	}{
		{
			name:   "test query deployment correctness",
			golden: "deployment-basic.golden.yaml",
			opts: Options{
				Options: manifests.Options{
					Owner:     "test-q",
					Namespace: "ns",
					Image:     ptr.To("some-custom-image"),
					Labels: map[string]string{
						"some-custom-label":      someCustomLabelValue,
						"some-other-label":       someOtherLabelValue,
						"app.kubernetes.io/name": "expect-to-be-discarded",
					},
					Annotations: map[string]string{
						"test":    "annotation",
						"another": "annotation",
					},
				},
				Timeout:       "15m",
				LookbackDelta: "5m",
				MaxConcurrent: 20,
			},
		},
		{
			name:   "test additional volumemount",
			golden: "deployment-with-volumemount.golden.yaml",
			opts: Options{
				Options: manifests.Options{
					Owner:     "test-q",
					Namespace: "ns",
					Image:     ptr.To("some-custom-image"),
					Labels: map[string]string{
						"some-custom-label":      someCustomLabelValue,
						"some-other-label":       someOtherLabelValue,
						"app.kubernetes.io/name": "expect-to-be-discarded",
					},
					Annotations: map[string]string{
						"test":    "annotation",
						"another": "annotation",
					},
					Additional: manifests.Additional{
						VolumeMounts: []corev1.VolumeMount{
							{
								Name:      "test-sd",
								MountPath: "/test-sd-file",
							},
						},
					},
				},
				Timeout:       "15m",
				LookbackDelta: "5m",
				MaxConcurrent: 20,
			},
		},
		{
			name:   "test additional container",
			golden: "deployment-with-container.golden.yaml",
			opts: Options{
				Options: manifests.Options{
					Owner:     "test-q",
					Namespace: "ns",
					Image:     ptr.To("some-custom-image"),
					Labels: map[string]string{
						"some-custom-label":      someCustomLabelValue,
						"some-other-label":       someOtherLabelValue,
						"app.kubernetes.io/name": "expect-to-be-discarded",
					},
					Annotations: map[string]string{
						"test":    "annotation",
						"another": "annotation",
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
				Timeout:       "15m",
				LookbackDelta: "5m",
				MaxConcurrent: 20,
			},
		},
	} {
		t.Run(tc.name, func(t *testing.T) {
			query := NewQueryDeployment(tc.opts)

			// Test against golden file
			yamlBytes, err := yaml.Marshal(query)
			if err != nil {
				t.Fatalf("failed to marshal deployment to YAML: %v", err)
			}
			golden.Assert(t, string(yamlBytes), tc.golden)
		})
	}
}

func TestNewQueryService(t *testing.T) {
	opts := Options{
		Options: manifests.Options{
			Owner:     "test-q",
			Namespace: "ns",
			Image:     ptr.To("some-custom-image"),
			Labels: map[string]string{
				"some-custom-label":      someCustomLabelValue,
				"some-other-label":       someOtherLabelValue,
				"app.kubernetes.io/name": "expect-to-be-discarded",
			},
			Annotations: map[string]string{
				"test":    "annotation",
				"another": "annotation",
			},
		},
		Timeout:       "15m",
		LookbackDelta: "5m",
		MaxConcurrent: 20,
	}

	for _, tc := range []struct {
		name   string
		golden string
		opts   Options
	}{
		{
			name:   "test query service correctness",
			golden: "service-basic.golden.yaml",
			opts:   opts,
		},
	} {
		t.Run(tc.name, func(t *testing.T) {
			querySvc := NewQueryService(tc.opts)

			// Test against golden file
			yamlBytes, err := yaml.Marshal(querySvc)
			if err != nil {
				t.Fatalf("failed to marshal service to YAML: %v", err)
			}
			golden.Assert(t, string(yamlBytes), tc.golden)
		})
	}
}

// TestBuildQueryGolden uses golden files to validate complete manifest generation
func TestBuildQueryGolden(t *testing.T) {
	tests := []struct {
		name string
		opts Options
	}{
		{
			name: "query-basic",
			opts: Options{
				Options: manifests.Options{
					Owner:     "test-owner",
					Namespace: "test-namespace",
					Image:     ptr.To("quay.io/thanos/thanos:v0.40.1"),
					Labels: map[string]string{
						"app.kubernetes.io/version": "v0.40.1",
					},
					Annotations: map[string]string{
						"test":    "annotation",
						"another": "annotation",
					},
					PodDisruptionConfig: &manifests.PodDisruptionBudgetOptions{},
				},
				Timeout:       "15m",
				LookbackDelta: "5m",
				MaxConcurrent: 20,
			},
		},
		{
			name: "query-complete",
			opts: Options{
				Options: manifests.Options{
					Owner:     "test-owner",
					Namespace: "test-namespace",
					Image:     ptr.To("quay.io/thanos/thanos:v0.40.1"),
					Labels: map[string]string{
						"app.kubernetes.io/version": "v0.40.1",
						"custom-label":              "custom-value",
					},
					Annotations: map[string]string{
						"test-annotation": "test-value",
						"another":         "annotation",
					},
					Additional: manifests.Additional{
						Args: []string{"--query.timeout=30m"},
						VolumeMounts: []corev1.VolumeMount{
							{
								Name:      "config",
								MountPath: "/etc/thanos",
							},
						},
						Volumes: []corev1.Volume{
							{
								Name: "config",
								VolumeSource: corev1.VolumeSource{
									ConfigMap: &corev1.ConfigMapVolumeSource{
										LocalObjectReference: corev1.LocalObjectReference{
											Name: "query-config",
										},
									},
								},
							},
						},
						Containers: []corev1.Container{
							{
								Name:  "sidecar",
								Image: "sidecar:latest",
								Args:  []string{"--verbose"},
							},
						},
					},
					PodDisruptionConfig: &manifests.PodDisruptionBudgetOptions{},
				},
				Timeout:       "30m",
				LookbackDelta: "10m",
				MaxConcurrent: 50,
			},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			objs := tt.opts.Build()

			// Validate complete manifest set
			yamlBytes, err := yaml.Marshal(objs)
			if err != nil {
				t.Fatalf("failed to marshal objects to YAML: %v", err)
			}
			golden.Assert(t, string(yamlBytes), tt.name+".golden.yaml")
		})
	}
}
