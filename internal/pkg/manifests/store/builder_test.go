package store

import (
	"reflect"
	"testing"

	"github.com/thanos-community/thanos-operator/internal/pkg/manifests"
	"github.com/thanos-community/thanos-operator/test/utils"

	corev1 "k8s.io/api/core/v1"
	"k8s.io/utils/ptr"

	"sigs.k8s.io/controller-runtime/pkg/client"
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

	extraLabels := map[string]string{
		"some-custom-label": someCustomLabelValue,
		"some-other-label":  someOtherLabelValue,
	}

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
		name string
		opts func() Options
	}{
		{
			name: "test store stateful correctness",
			opts: func() Options {
				return buildDefaultOpts()
			},
		},
		{
			name: "test additional volumemount",
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
			name: "test additional container",
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
			t.Parallel()
			builtOpts := tc.opts()
			store := NewStoreStatefulSet(builtOpts)
			objectMetaLabels := GetLabels(builtOpts)
			name := builtOpts.GetGeneratedResourceName()

			utils.ValidateNameNamespaceAndLabels(t, store, name, ns, objectMetaLabels)
			utils.ValidateHasLabels(t, store, builtOpts.GetSelectorLabels())
			utils.ValidateHasLabels(t, store, extraLabels)
			objName := builtOpts.GetGeneratedResourceName()

			if store.Spec.ServiceName != objName {
				t.Errorf("expected store statefulset to have serviceName %s, got %s", objName, store.Spec.ServiceName)
			}

			if store.Spec.Template.Spec.ServiceAccountName != objName {
				t.Errorf("expected store statefulset to have service account owner %s, got %s", objName, store.Spec.Template.Spec.ServiceAccountName)
			}

			if len(store.Spec.Template.Spec.Containers) != (len(builtOpts.Additional.Containers) + 1) {
				t.Errorf("expected store statefulset to have %d containers, got %d", len(builtOpts.Additional.Containers)+1, len(store.Spec.Template.Spec.Containers))
			}

			if store.Annotations["test"] != "annotation" {
				t.Errorf("expected store statefulset annotation test to be annotation, got %s", store.Annotations["test"])
			}

			expectArgs := storeArgsFrom(builtOpts)
			var found bool
			for _, c := range store.Spec.Template.Spec.Containers {
				if c.Name == Name {
					found = true
					if c.Image != builtOpts.GetContainerImage() {
						t.Errorf("expected store statefulset to have image %s, got %s", builtOpts.GetContainerImage(), c.Image)
					}

					if !reflect.DeepEqual(c.Args, expectArgs) {
						t.Errorf("expected store statefulset to have args %v, got %v", expectArgs, c.Args)
					}

					if len(c.VolumeMounts) != len(builtOpts.Additional.VolumeMounts)+1 {
						if c.VolumeMounts[0].Name != dataVolumeName {
							t.Errorf("expected store statefulset to have volumemount named data, got %s", c.VolumeMounts[0].Name)
						}
						if c.VolumeMounts[0].MountPath != dataVolumeMountPath {
							t.Errorf("expected store statefulset to have volumemount mounted at var/thanos/store, got %s", c.VolumeMounts[0].MountPath)
						}
					}
				}
			}
			if !found {
				t.Errorf("expected store statefulset to have container named %s", Name)
			}
		})
	}
}

func TestNewStoreService(t *testing.T) {
	const (
		ns = "ns"
	)

	extraLabels := map[string]string{
		"some-custom-label":          someCustomLabelValue,
		"some-other-label":           someOtherLabelValue,
		string(manifests.GroupLabel): "true",
	}

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
		name string
		opts func() Options
	}{
		{
			name: "test store service correctness",
			opts: func() Options {
				return opts
			},
		},
	} {
		t.Run(tc.name, func(t *testing.T) {
			t.Parallel()
			builtOpts := tc.opts()
			storeSvc := NewStoreService(builtOpts)
			objectMetaLabels := GetLabels(builtOpts)
			utils.ValidateNameNamespaceAndLabels(t, storeSvc, opts.GetGeneratedResourceName(), ns, objectMetaLabels)
			utils.ValidateHasLabels(t, storeSvc, extraLabels)
			utils.ValidateHasLabels(t, storeSvc, opts.GetSelectorLabels())

			if storeSvc.Spec.ClusterIP != corev1.ClusterIPNone {
				t.Errorf("expected store service to have ClusterIP 'None', got %s", storeSvc.Spec.ClusterIP)
			}
		})
	}
}
