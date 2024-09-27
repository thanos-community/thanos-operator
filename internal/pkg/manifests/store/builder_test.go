package store

import (
	"reflect"
	"testing"

	"github.com/thanos-community/thanos-operator/internal/pkg/manifests"
	"github.com/thanos-community/thanos-operator/test/utils"

	appsv1 "k8s.io/api/apps/v1"
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
			Name:      "test",
			Namespace: "ns",
			Image:     ptr.To("some-custom-image"),
			Labels: map[string]string{
				"some-custom-label":      someCustomLabelValue,
				"some-other-label":       someOtherLabelValue,
				"app.kubernetes.io/name": "expect-to-be-discarded",
			},
		},
	}

	objs := Build(opts)
	if len(objs) != 4 {
		t.Fatalf("expected 4 objects, got %d", len(objs))
	}

	validateServiceAccount(t, opts, objs[0], "test")
	utils.ValidateObjectsEqual(t, objs[1], NewStoreService(opts))
	utils.ValidateObjectsEqual(t, objs[2], NewStoreStatefulSet(opts))

	wantLabels := GetSelectorLabels(opts)
	wantLabels["some-custom-label"] = someCustomLabelValue
	wantLabels["some-other-label"] = someOtherLabelValue
	utils.ValidateObjectLabelsEqual(t, wantLabels, []client.Object{objs[1], objs[2]}...)
}

func TestNewStoreStatefulSet(t *testing.T) {
	const (
		name = "test"
		ns   = "ns"
	)

	extraLabels := map[string]string{
		"some-custom-label": someCustomLabelValue,
		"some-other-label":  someOtherLabelValue,
	}

	buildDefaultOpts := func() Options {
		return Options{
			Options: manifests.Options{
				Name:      name,
				Namespace: ns,
				Image:     ptr.To("some-custom-image"),
				Labels: map[string]string{
					"some-custom-label":      someCustomLabelValue,
					"some-other-label":       someOtherLabelValue,
					"app.kubernetes.io/name": "expect-to-be-discarded",
				},
			},
		}
	}

	for _, tc := range []struct {
		name       string
		opts       func() Options
		expectName string
	}{
		{
			name: "test store stateful correctness",
			opts: func() Options {
				return buildDefaultOpts()
			},
			expectName: name,
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
			expectName: name,
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
			expectName: name,
		},
	} {
		t.Run(tc.name, func(t *testing.T) {
			builtOpts := tc.opts()
			store := NewStoreStatefulSet(builtOpts)
			objectMetaLabels := GetLabels(builtOpts)

			utils.ValidateNameNamespaceAndLabels(t, store, tc.expectName, ns, objectMetaLabels)
			utils.ValidateHasLabels(t, store, GetSelectorLabels(builtOpts))
			utils.ValidateHasLabels(t, store, extraLabels)

			storeSts := store.(*appsv1.StatefulSet)
			if storeSts.Spec.ServiceName != GetServiceAccountName(builtOpts) {
				t.Errorf("expected store statefulset to have serviceName %s, got %s", GetServiceAccountName(builtOpts), storeSts.Spec.ServiceName)
			}

			if len(storeSts.Spec.Template.Spec.Containers) != (len(builtOpts.Additional.Containers) + 1) {
				t.Errorf("expected store statefulset to have %d containers, got %d", len(builtOpts.Additional.Containers)+1, len(storeSts.Spec.Template.Spec.Containers))
			}

			expectArgs := storeArgsFrom(builtOpts)
			var found bool
			for _, c := range storeSts.Spec.Template.Spec.Containers {
				if c.Name == Name {
					found = true
					if c.Image != builtOpts.GetContainerImage() {
						t.Errorf("expected store statefulset to have image %s, got %s", builtOpts.GetContainerImage(), c.Image)
					}
					if len(c.Args) != len(expectArgs) {
						t.Errorf("expected store statefulset to have %d args, got %d", len(expectArgs), len(c.Args))
					}
					for i, arg := range c.Args {
						if arg != expectArgs[i] {
							t.Errorf("expected store statefulset to have arg %s, got %s", expectArgs[i], arg)
						}
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
		name = "test"
		ns   = "ns"
	)

	extraLabels := map[string]string{
		"some-custom-label": someCustomLabelValue,
		"some-other-label":  someOtherLabelValue,
	}

	opts := Options{
		Options: manifests.Options{
			Name:      name,
			Namespace: ns,
			Image:     ptr.To("some-custom-image"),
			Labels: map[string]string{
				"some-custom-label":      someCustomLabelValue,
				"some-other-label":       someOtherLabelValue,
				"app.kubernetes.io/name": "expect-to-be-discarded",
			},
		},
	}

	for _, tc := range []struct {
		name       string
		opts       func() Options
		expectName string
	}{
		{
			name: "test store service correctness",
			opts: func() Options {
				return opts
			},
			expectName: name,
		},
		{
			name: "test store service correctness",
			opts: func() Options {
				opts.Name = "test-shard"
				return opts
			},
			expectName: "test-shard",
		},
	} {
		t.Run(tc.name, func(t *testing.T) {
			builtOpts := tc.opts()
			storeSvc := NewStoreService(builtOpts)
			objectMetaLabels := GetLabels(builtOpts)
			utils.ValidateNameNamespaceAndLabels(t, storeSvc, tc.expectName, ns, objectMetaLabels)
			utils.ValidateHasLabels(t, storeSvc, extraLabels)
			utils.ValidateHasLabels(t, storeSvc, GetSelectorLabels(builtOpts))

			svc := storeSvc.(*corev1.Service)
			if svc.Spec.ClusterIP != "None" {
				t.Errorf("expected store service to have ClusterIP 'None', got %s", svc.Spec.ClusterIP)
			}
		})
	}
}

func validateServiceAccount(t *testing.T, opts Options, expectSA client.Object, expectName string) {
	t.Helper()
	if expectSA.GetObjectKind().GroupVersionKind().Kind != "ServiceAccount" {
		t.Errorf("expected object to be a service account, got %v", expectSA.GetObjectKind().GroupVersionKind().Kind)
	}

	if expectSA.GetName() != expectName {
		t.Errorf("expected service account name to be %s, got %s", expectName, expectSA.GetName())
	}

	if expectSA.GetNamespace() != opts.Namespace {
		t.Errorf("expected service account namespace to be %s, got %s", expectSA.GetNamespace(), opts.Namespace)
	}

	if !reflect.DeepEqual(expectSA.GetLabels(), GetSelectorLabels(opts)) {
		t.Errorf("expected service account to have labels %v, got %v", GetSelectorLabels(opts), expectSA.GetLabels())
	}
}
