package compact

import (
	"testing"

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
						"test": "annotation",
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
						"test": "annotation",
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
			// should ignore this setting
			PodDisruptionConfig: &manifests.PodDisruptionBudgetOptions{},
		},
	}

	objs := opts.Build()
	if len(objs) != 3 {
		t.Fatalf("expected 3 objects, got %d", len(objs))
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

func TestCompactionOptions_toArgs(t *testing.T) {
	tests := []struct {
		name     string
		opts     *CompactionOptions
		expected []string
	}{
		{
			name:     "nil options",
			opts:     nil,
			expected: []string{},
		},
		{
			name: "vertical compaction enabled",
			opts: &CompactionOptions{
				EnableVerticalCompaction: ptr.To(true),
			},
			expected: []string{"--compact.enable-vertical-compaction"},
		},
		{
			name: "vertical compaction disabled",
			opts: &CompactionOptions{
				EnableVerticalCompaction: ptr.To(false),
			},
			expected: []string{},
		},
		{
			name: "vertical compaction with other options",
			opts: &CompactionOptions{
				CompactConcurrency:       ptr.To(int32(2)),
				EnableVerticalCompaction: ptr.To(true),
			},
			expected: []string{"--compact.concurrency=2", "--compact.enable-vertical-compaction"},
		},
		{
			name: "deduplication replica labels",
			opts: &CompactionOptions{
				DeduplicationReplicaLabels: []string{"replica", "prometheus_replica"},
			},
			expected: []string{"--deduplication.replica-label=replica", "--deduplication.replica-label=prometheus_replica"},
		},
		{
			name: "deduplication func penalty",
			opts: &CompactionOptions{
				DeduplicationFunc: ptr.To("penalty"),
			},
			expected: []string{"--deduplication.func=penalty"},
		},
		{
			name: "deduplication func empty string ignored",
			opts: &CompactionOptions{
				DeduplicationFunc: ptr.To(""),
			},
			expected: []string{},
		},
		{
			name: "vertical compaction with deduplication",
			opts: &CompactionOptions{
				EnableVerticalCompaction:   ptr.To(true),
				DeduplicationReplicaLabels: []string{"replica"},
				DeduplicationFunc:          ptr.To("penalty"),
			},
			expected: []string{"--compact.enable-vertical-compaction", "--deduplication.replica-label=replica", "--deduplication.func=penalty"},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result := tt.opts.toArgs()
			if len(result) != len(tt.expected) {
				t.Errorf("expected %d args, got %d: %v", len(tt.expected), len(result), result)
				return
			}
			for i, arg := range result {
				if arg != tt.expected[i] {
					t.Errorf("expected arg[%d] to be %s, got %s", i, tt.expected[i], arg)
				}
			}
		})
	}
}
