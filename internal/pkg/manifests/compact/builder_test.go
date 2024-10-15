package compact

import (
	"reflect"
	"testing"

	"github.com/thanos-community/thanos-operator/internal/pkg/manifests"
	"github.com/thanos-community/thanos-operator/test/utils"

	"k8s.io/utils/ptr"

	"sigs.k8s.io/controller-runtime/pkg/client"
)

const (
	someCustomLabelValue string = "xyz"
	someOtherLabelValue  string = "abc"
)

func TestNewService(t *testing.T) {
	opts := Options{
		Options: manifests.Options{
			Namespace: "any",
			Owner:     "standalone",
		},
	}
	obj := NewService(opts)
	objectMetaLabels := GetLabels(opts)
	utils.ValidateNameNamespaceAndLabels(t, obj, opts.GetGeneratedResourceName(), opts.Namespace, objectMetaLabels)
	utils.ValidateHasLabels(t, obj, opts.GetSelectorLabels())

	if obj.Spec.Ports[0].Name != HTTPPortName {
		t.Errorf("expected service port name to be 'http', got %s", obj.Spec.Ports[0].Name)
	}
	if obj.Spec.Ports[0].Port != HTTPPort {
		t.Errorf("expected service port to be %d, got %d", HTTPPort, obj.Spec.Ports[0].Port)
	}

	opts = Options{
		Options: manifests.Options{
			Namespace: "any",
			Owner:     "shard",
		},
	}
	obj = NewService(opts)
	objectMetaLabels = GetLabels(opts)
	utils.ValidateNameNamespaceAndLabels(t, obj, opts.GetGeneratedResourceName(), opts.Namespace, objectMetaLabels)
	utils.ValidateHasLabels(t, obj, opts.GetSelectorLabels())
}

func TestNewStatefulSet(t *testing.T) {
	extraLabels := map[string]string{
		"some-custom-label": someCustomLabelValue,
		"some-other-label":  someOtherLabelValue,
	}

	for _, tc := range []struct {
		name string
		opts Options
	}{
		{
			name: "test compact sts correctness with no shard name",
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
			name: "test compact sts correctness with no shard name",
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
			objectMetaLabels := GetLabels(tc.opts)
			name := tc.opts.GetGeneratedResourceName()
			utils.ValidateNameNamespaceAndLabels(t, compact, name, tc.opts.Namespace, objectMetaLabels)
			utils.ValidateHasLabels(t, compact, tc.opts.GetSelectorLabels())
			utils.ValidateHasLabels(t, compact, extraLabels)

			if compact.Spec.ServiceName != name {
				t.Errorf("expected compact statefulset to have serviceName %s, got %s", name, compact.Spec.ServiceName)
			}

			if compact.Spec.Template.Spec.ServiceAccountName != name {
				t.Errorf("expected compact statefulset to have service account name %s, got %s", name, compact.Spec.Template.Spec.ServiceAccountName)
			}

			if *compact.Spec.Replicas != *ptr.To(int32(1)) {
				t.Errorf("expected compact statefulset to have 1 replica, got %d", *compact.Spec.Replicas)
			}

			if len(compact.Spec.Template.Spec.Containers) != (len(tc.opts.Additional.Containers) + 1) {
				t.Errorf("expected compact statefulset to have %d containers, got %d", len(tc.opts.Additional.Containers)+1, len(compact.Spec.Template.Spec.Containers))
			}

			if compact.Annotations["test"] != "annotation" {
				t.Errorf("expected compact statefulset annotation test to be annotation, got %s", compact.Annotations["test"])
			}

			expectArgs := compactorArgsFrom(tc.opts)
			var found bool
			for _, c := range compact.Spec.Template.Spec.Containers {
				if c.Name == Name {
					found = true
					if c.Image != tc.opts.GetContainerImage() {
						t.Errorf("expected compact statefulset to have image %s, got %s", tc.opts.GetContainerImage(), c.Image)
					}

					if !reflect.DeepEqual(c.Args, expectArgs) {
						t.Errorf("expected compact statefulset to have args %v, got %v", expectArgs, c.Args)
					}

					if len(c.VolumeMounts) != len(tc.opts.Additional.VolumeMounts)+1 {
						t.Errorf("expected compact statefulset to have 2 containers, got %d", len(compact.Spec.Template.Spec.Containers))
					}

					if c.VolumeMounts[0].Name != dataVolumeName {
						t.Errorf("expected compact statefulset to have volumemount named data, got %s", c.VolumeMounts[0].Name)
					}
					if c.VolumeMounts[0].MountPath != dataVolumeMountPath {
						t.Errorf("expected compact statefulset to have volumemount mounted at var/thanos/compact, got %s", c.VolumeMounts[0].MountPath)
					}
				}
			}
			if !found {
				t.Errorf("expected compact statefulset to have container named %s", Name)
			}
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
				ShardName:  ptr.To("shard"),
				ShardIndex: ptr.To(1),
			},
			expected: "thanos-compact-valid-owner-shard-1",
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
