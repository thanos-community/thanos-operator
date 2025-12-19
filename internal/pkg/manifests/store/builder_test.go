package store

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

func TestBuildStore(t *testing.T) {
	opts := Options{
		Options: manifests.Options{
			Namespace: "ns",
			Image:     ptr.To("some-custom-image"),
			Labels: map[string]string{
				"some-custom-label":      someCustomLabelValue,
				"some-other-label":       someOtherLabelValue,
				"app.kubernetes.io/name": "expect-to-be-discarded",
			},
			Owner:               "any",
			PodDisruptionConfig: &manifests.PodDisruptionBudgetOptions{},
		},
	}

	objs := opts.Build()
	if len(objs) != 4 {
		t.Fatalf("expected 4 objects, got %d", len(objs))
	}

	utils.ValidateIsNamedServiceAccount(t, objs[0], opts, opts.Namespace)
	utils.ValidateObjectsEqual(t, objs[1], NewStoreService(opts))
	utils.ValidateObjectsEqual(t, objs[2], NewStoreStatefulSet(opts))
	utils.ValidateIsNamedPodDisruptionBudget(t, objs[3], opts, opts.Namespace, objs[2])

	wantLabels := opts.GetSelectorLabels()
	wantLabels["some-custom-label"] = someCustomLabelValue
	wantLabels["some-other-label"] = someOtherLabelValue
	utils.ValidateObjectLabelsEqual(t, wantLabels, []client.Object{objs[1], objs[2]}...)
}

func TestNewStoreStatefulSet(t *testing.T) {
	const (
		owner = "test"
		ns    = "ns"
	)

	buildDefaultOpts := func() Options {
		return Options{
			Options: manifests.Options{
				Owner:     owner,
				Namespace: ns,
				Image:     ptr.To("some-custom-image"),
				Labels: map[string]string{
					"some-custom-label":       someCustomLabelValue,
					"some-other-label":        someOtherLabelValue,
					"app.kubernetes.io/owner": "expect-to-be-discarded",
				},
				Annotations: map[string]string{
					"test": "annotation",
				},
			},
		}
	}

	for _, tc := range []struct {
		name   string
		golden string
		opts   func() Options
	}{
		{
			name:   "test store stateful correctness",
			golden: "statefulset-basic.golden.yaml",
			opts: func() Options {
				return buildDefaultOpts()
			},
		},
		{
			name:   "test additional volumemount",
			golden: "statefulset-with-volumemount.golden.yaml",
			opts: func() Options {
				opts := buildDefaultOpts()
				opts.Additional.VolumeMounts = []corev1.VolumeMount{
					{
						Name:      "test-sd",
						MountPath: "/test-sd-file",
					},
				}
				return opts
			},
		},
		{
			name:   "test additional container",
			golden: "statefulset-with-container.golden.yaml",
			opts: func() Options {
				opts := buildDefaultOpts()
				opts.Additional.Containers = []corev1.Container{
					{
						Name:  "test-container",
						Image: "test-image:latest",
						Args:  []string{"--test-arg"},
						Env: []corev1.EnvVar{{
							Name:  "TEST_ENV",
							Value: "test",
						}},
					},
				}
				return opts
			},
		},
	} {
		t.Run(tc.name, func(t *testing.T) {
			builtOpts := tc.opts()
			store := NewStoreStatefulSet(builtOpts)

			// Test against golden file
			yamlBytes, err := yaml.Marshal(store)
			if err != nil {
				t.Fatalf("failed to marshal statefulset to YAML: %v", err)
			}
			golden.Assert(t, string(yamlBytes), tc.golden)
		})
	}
}

func TestNewStoreService(t *testing.T) {
	const (
		ns = "ns"
	)

	opts := Options{
		Options: manifests.Options{
			Namespace: ns,
			Image:     ptr.To("some-custom-image"),
			Labels: map[string]string{
				"some-custom-label":      someCustomLabelValue,
				"some-other-label":       someOtherLabelValue,
				"app.kubernetes.io/name": "expect-to-be-discarded",
			},
			Replicas: int32(3),
		},
	}

	for _, tc := range []struct {
		name   string
		golden string
		opts   func() Options
	}{
		{
			name:   "test store service correctness",
			golden: "service-basic.golden.yaml",
			opts: func() Options {
				return opts
			},
		},
	} {
		t.Run(tc.name, func(t *testing.T) {
			builtOpts := tc.opts()
			storeSvc := NewStoreService(builtOpts)

			// Test against golden file
			yamlBytes, err := yaml.Marshal(storeSvc)
			if err != nil {
				t.Fatalf("failed to marshal service to YAML: %v", err)
			}
			golden.Assert(t, string(yamlBytes), tc.golden)
		})
	}
}
