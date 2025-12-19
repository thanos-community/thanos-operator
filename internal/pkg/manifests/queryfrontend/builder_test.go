package queryfrontend

import (
	"testing"

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

func TestBuildQueryFrontend(t *testing.T) {
	opts := Options{
		Options: manifests.Options{
			Namespace: "ns",
			Owner:     "any",
			Image:     ptr.To("some-custom-image"),
			Labels: map[string]string{
				"some-custom-label":      someCustomLabelValue,
				"some-other-label":       someOtherLabelValue,
				"app.kubernetes.io/name": "expect-to-be-discarded",
			},
			PodDisruptionConfig: &manifests.PodDisruptionBudgetOptions{},
		},
		QueryService:         "thanos-query",
		LogQueriesLongerThan: "5s",
		CompressResponses:    true,
		RangeSplitInterval:   "1h",
		LabelsSplitInterval:  "30m",
		RangeMaxRetries:      5,
		LabelsMaxRetries:     3,
	}

	objs := opts.Build()
	if len(objs) != 4 {
		t.Fatalf("expected 4 objects, got %d", len(objs))
	}

	utils.ValidateIsNamedServiceAccount(t, objs[0], opts, opts.Namespace)
	utils.ValidateObjectsEqual(t, objs[1], NewQueryFrontendDeployment(opts))
	utils.ValidateObjectsEqual(t, objs[2], NewQueryFrontendService(opts))
	if objs[3].GetObjectKind().GroupVersionKind().Kind != "PodDisruptionBudget" {
		t.Errorf("expected object to be a PodDisruptionBudget, got %v", objs[3].GetObjectKind().GroupVersionKind().Kind)
	}
	utils.ValidateLabelsMatch(t, objs[3], objs[1])

	wantLabels := opts.GetSelectorLabels()
	wantLabels["some-custom-label"] = someCustomLabelValue
	wantLabels["some-other-label"] = someOtherLabelValue
	utils.ValidateObjectLabelsEqual(t, wantLabels, []client.Object{objs[1], objs[2]}...)
}

func TestNewQueryFrontendDeployment(t *testing.T) {

	for _, tc := range []struct {
		name   string
		golden string
		opts   Options
	}{
		{
			name:   "test query frontend deployment correctness",
			golden: "deployment-basic.golden.yaml",
			opts: Options{
				Options: manifests.Options{
					Owner:     "test-qf",
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
					Replicas: 2,
				},
				QueryService:         "thanos-query",
				LogQueriesLongerThan: "5s",
				CompressResponses:    true,
				RangeSplitInterval:   "1h",
				LabelsSplitInterval:  "30m",
				RangeMaxRetries:      5,
				LabelsMaxRetries:     3,
			},
		},
		{
			name:   "test additional volumemount",
			golden: "deployment-with-volumemount.golden.yaml",
			opts: Options{
				Options: manifests.Options{
					Owner:     "test-qf",
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
					Replicas: 2,
					Additional: manifests.Additional{
						VolumeMounts: []corev1.VolumeMount{
							{
								Name:      "test-sd",
								MountPath: "/test-sd-file",
							},
						},
					},
				},
				QueryService:         "thanos-query",
				LogQueriesLongerThan: "5s",
				CompressResponses:    true,
				RangeSplitInterval:   "1h",
				LabelsSplitInterval:  "30m",
				RangeMaxRetries:      5,
				LabelsMaxRetries:     3,
			},
		},
		{
			name:   "test additional container",
			golden: "deployment-with-container.golden.yaml",
			opts: Options{
				Options: manifests.Options{
					Owner:     "test-qf",
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
					Replicas: 2,
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
				QueryService:         "thanos-query",
				LogQueriesLongerThan: "5s",
				CompressResponses:    true,
				RangeSplitInterval:   "1h",
				LabelsSplitInterval:  "30m",
				RangeMaxRetries:      5,
				LabelsMaxRetries:     3,
			},
		},
		{
			name:   "test with external cache",
			golden: "deployment-with-cache.golden.yaml",
			opts: Options{
				Options: manifests.Options{
					Owner:     "test-qf",
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
					Replicas: 2,
				},
				QueryService:         "thanos-query",
				LogQueriesLongerThan: "5s",
				CompressResponses:    true,
				RangeSplitInterval:   "1h",
				LabelsSplitInterval:  "30m",
				RangeMaxRetries:      5,
				LabelsMaxRetries:     3,
				ResponseCacheConfig: manifests.CacheConfig{
					InMemoryCacheConfig: nil,
					FromSecret:          &corev1.SecretKeySelector{LocalObjectReference: corev1.LocalObjectReference{Name: "test-secret"}},
				},
			},
		},
	} {
		t.Run(tc.name, func(t *testing.T) {
			deployment := NewQueryFrontendDeployment(tc.opts)

			// Test against golden file
			yamlBytes, err := yaml.Marshal(deployment)
			if err != nil {
				t.Fatalf("failed to marshal deployment to YAML: %v", err)
			}
			golden.Assert(t, string(yamlBytes), tc.golden)
		})
	}
}

func TestNewQueryFrontendService(t *testing.T) {

	opts := Options{
		Options: manifests.Options{
			Owner:     "test-qf",
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
		opts   Options
	}{
		{
			name:   "test query frontend service correctness",
			golden: "service-basic.golden.yaml",
			opts:   opts,
		},
	} {
		t.Run(tc.name, func(t *testing.T) {
			service := NewQueryFrontendService(tc.opts)

			// Test against golden file
			yamlBytes, err := yaml.Marshal(service)
			if err != nil {
				t.Fatalf("failed to marshal service to YAML: %v", err)
			}
			golden.Assert(t, string(yamlBytes), tc.golden)
		})
	}
}

// TestBuildQueryFrontendGolden uses golden files to validate complete query frontend manifest generation
func TestBuildQueryFrontendGolden(t *testing.T) {
	tests := []struct {
		name string
		opts Options
	}{
		{
			name: "queryfrontend-basic",
			opts: Options{
				Options: manifests.Options{
					Owner:     "test-owner",
					Namespace: "test-namespace",
					Image:     ptr.To("quay.io/thanos/thanos:v0.40.1"),
					Labels: map[string]string{
						"app.kubernetes.io/version": "v0.40.1",
					},
					PodDisruptionConfig: &manifests.PodDisruptionBudgetOptions{},
				},
				QueryService:         "thanos-query",
				LogQueriesLongerThan: "5s",
				CompressResponses:    true,
				RangeSplitInterval:   "1h",
				LabelsSplitInterval:  "30m",
				RangeMaxRetries:      5,
				LabelsMaxRetries:     3,
			},
		},
		{
			name: "queryfrontend-with-cache",
			opts: Options{
				Options: manifests.Options{
					Owner:     "test-owner",
					Namespace: "test-namespace",
					Image:     ptr.To("quay.io/thanos/thanos:v0.40.1"),
					Labels: map[string]string{
						"app.kubernetes.io/version": "v0.40.1",
					},
					Annotations: map[string]string{
						"test-annotation": "test-value",
					},
					PodDisruptionConfig: &manifests.PodDisruptionBudgetOptions{},
				},
				QueryService:         "thanos-query",
				LogQueriesLongerThan: "10s",
				CompressResponses:    true,
				RangeSplitInterval:   "2h",
				LabelsSplitInterval:  "1h",
				RangeMaxRetries:      3,
				LabelsMaxRetries:     2,
				ResponseCacheConfig: manifests.CacheConfig{
					InMemoryCacheConfig: &manifests.InMemoryCacheConfig{
						MaxSize:     "256MB",
						MaxItemSize: "32MB",
					},
				},
			},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			objs := tt.opts.Build()

			yamlBytes, err := yaml.Marshal(objs)
			if err != nil {
				t.Fatalf("failed to marshal objects to YAML: %v", err)
			}
			golden.Assert(t, string(yamlBytes), tt.name+".golden.yaml")
		})
	}
}
