package compact

import (
	"testing"

	"gotest.tools/v3/assert"

	"github.com/thanos-community/thanos-operator/internal/pkg/manifests"
	"github.com/thanos-community/thanos-operator/test/utils"

	"gotest.tools/v3/golden"

	"k8s.io/utils/ptr"

	"sigs.k8s.io/controller-runtime/pkg/client"
	"sigs.k8s.io/yaml"
)

const (
	someCustomLabelValue string = "xyz"
	someOtherLabelValue  string = "abc"
)

func TestNewService(t *testing.T) {
	for _, tc := range []struct {
		name   string
		golden string
		opts   Options
	}{
		{
			name:   "standalone service",
			golden: "service-standalone.golden.yaml",
			opts: Options{
				Options: manifests.Options{
					Namespace: "any",
					Owner:     "standalone",
					Annotations: map[string]string{
						"test":    "annotation",
						"another": "annotation",
					},
				},
			},
		},
		{
			name:   "shard service",
			golden: "service-shard.golden.yaml",
			opts: Options{
				Options: manifests.Options{
					Namespace: "any",
					Owner:     "shard",
					Annotations: map[string]string{
						"test":    "annotation",
						"another": "annotation",
					},
				},
			},
		},
	} {
		t.Run(tc.name, func(t *testing.T) {
			obj := NewService(tc.opts)

			// Test against golden file
			yamlBytes, err := yaml.Marshal(obj)
			if err != nil {
				t.Fatalf("failed to marshal service to YAML: %v", err)
			}
			golden.Assert(t, string(yamlBytes), tc.golden)
		})
	}
}

func TestNewStatefulSet(t *testing.T) {

	for _, tc := range []struct {
		name   string
		opts   Options
		golden string
	}{
		{
			name:   "test compact sts correctness with no shard name",
			golden: "statefulset-no-shard.golden.yaml",
			opts: Options{
				Options: manifests.Options{
					Owner:     "test",
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
			},
		},
		{
			name:   "test compact sts correctness with shard name",
			golden: "statefulset-with-shard.golden.yaml",
			opts: Options{
				Options: manifests.Options{
					Owner:     "some-shard",
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
			},
		},
	} {
		t.Run(tc.name, func(t *testing.T) {
			compact := NewStatefulSet(tc.opts)

			// Test against golden file
			yamlBytes, err := yaml.Marshal(compact)
			if err != nil {
				t.Fatalf("failed to marshal statefulset to YAML: %v", err)
			}
			golden.Assert(t, string(yamlBytes), tc.golden)
		})
	}
}

func TestBuild(t *testing.T) {
	opts := Options{
		Options: manifests.Options{
			Owner:     "test",
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
			// should ignore this setting
			PodDisruptionConfig: &manifests.PodDisruptionBudgetOptions{},
		},
	}

	objs := opts.Build()
	if len(objs) != 3 {
		t.Fatalf("expected 3 objects, got %d", len(objs))
	}

	for _, obj := range objs {
		assert.Assert(t, obj.GetAnnotations()["test"] == "annotation")
	}

	utils.ValidateIsNamedServiceAccount(t, objs[0], opts, opts.Namespace)
	utils.ValidateObjectsEqual(t, objs[1], NewStatefulSet(opts))
	utils.ValidateObjectsEqual(t, objs[2], NewService(opts))

	wantLabels := opts.GetSelectorLabels()
	wantLabels["some-custom-label"] = someCustomLabelValue
	wantLabels["some-other-label"] = someOtherLabelValue
	utils.ValidateObjectLabelsEqual(t, wantLabels, []client.Object{objs[1], objs[2]}...)
}

func TestOptions_GetName(t *testing.T) {
	tests := []struct {
		name     string
		opts     Options
		expected string
	}{
		{
			name: "Test not sharded",
			opts: Options{
				Options: manifests.Options{
					Owner: "valid-owner",
				},
			},
			expected: "thanos-compact-valid-owner",
		},
		{
			name: "Test sharded",
			opts: Options{
				Options: manifests.Options{
					Owner: "valid-owner",
				},
				ShardName: ptr.To("shard"),
			},
			expected: "thanos-compact-valid-owner-shard",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result := tt.opts.GetGeneratedResourceName()
			if result != tt.expected {
				t.Errorf("expected %s, got %s", tt.expected, result)
			}
		})
	}
}
