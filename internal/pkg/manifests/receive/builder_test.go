package receive

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

func TestBuildIngesters(t *testing.T) {
	opts := IngesterOptions{
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

	objs := BuildIngester(opts)
	if len(objs) != 3 {
		t.Fatalf("expected 3 objects, got %d", len(objs))
	}

	validateIngesterServiceAccount(t, opts, objs[0])
	utils.ValidateObjectsEqual(t, objs[1], NewIngestorService(opts))
	utils.ValidateObjectsEqual(t, objs[2], NewIngestorStatefulSet(opts))

	wantLabels := GetIngesterSelectorLabels(opts)
	wantLabels["some-custom-label"] = someCustomLabelValue
	wantLabels["some-other-label"] = someOtherLabelValue
	utils.ValidateObjectLabelsEqual(t, wantLabels, []client.Object{objs[1], objs[2]}...)
}

func TestBuildRouter(t *testing.T) {
	opts := RouterOptions{
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

	objs := BuildRouter(opts)
	if len(objs) != 4 {
		t.Fatalf("expected 3 objects, got %d", len(objs))
	}

	validateRouterServiceAccount(t, opts, objs[0])
	utils.ValidateObjectsEqual(t, objs[1], NewRouterService(opts))
	utils.ValidateObjectsEqual(t, objs[2], NewRouterDeployment(opts))

	wantLabels := GetRouterLabels(opts)
	wantLabels["some-custom-label"] = someCustomLabelValue
	wantLabels["some-other-label"] = someOtherLabelValue
	utils.ValidateObjectLabelsEqual(t, wantLabels, []client.Object{objs[1], objs[2]}...)
}

func TestNewIngestorStatefulSet(t *testing.T) {
	extraLabels := map[string]string{
		"some-custom-label": someCustomLabelValue,
		"some-other-label":  someOtherLabelValue,
	}

	for _, tc := range []struct {
		name string
		opts IngesterOptions
	}{
		{
			name: "test ingester statefulset correctness",
			opts: IngesterOptions{
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
			},
		},
		{
			name: "test additional volumemount",
			opts: IngesterOptions{
				Options: manifests.Options{
					Name:      "test",
					Namespace: "ns",
					Image:     ptr.To("some-custom-image"),
					Labels: map[string]string{
						"some-custom-label":      someCustomLabelValue,
						"some-other-label":       someOtherLabelValue,
						"app.kubernetes.io/name": "expect-to-be-discarded",
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
			name: "test additional container",
			opts: IngesterOptions{
				Options: manifests.Options{
					Name:      "test",
					Namespace: "ns",
					Image:     ptr.To("some-custom-image"),
					Labels: map[string]string{
						"some-custom-label":      someCustomLabelValue,
						"some-other-label":       someOtherLabelValue,
						"app.kubernetes.io/name": "expect-to-be-discarded",
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
			objectMetaLabels := GetIngesterLabels(tc.opts)
			utils.ValidateNameNamespaceAndLabels(t, ingester, tc.opts.Name, tc.opts.Namespace, objectMetaLabels)
			utils.ValidateHasLabels(t, ingester, GetIngesterSelectorLabels(tc.opts))
			utils.ValidateHasLabels(t, ingester, extraLabels)

			if ingester.Spec.ServiceName != GetIngesterServiceName(tc.opts) {
				t.Errorf("expected ingester statefulset to have serviceName %s, got %s", GetIngesterServiceAccountName(tc.opts), ingester.Spec.ServiceName)
			}

			if ingester.Spec.Template.Spec.ServiceAccountName != GetIngesterServiceAccountName(tc.opts) {
				t.Errorf("expected ingester statefulset to have service account name %s, got %s", GetIngesterServiceAccountName(tc.opts), ingester.Spec.Template.Spec.ServiceAccountName)
			}

			if len(ingester.Spec.Template.Spec.Containers) != (len(tc.opts.Additional.Containers) + 1) {
				t.Errorf("expected ingester statefulset to have %d containers, got %d", len(tc.opts.Additional.Containers)+1, len(ingester.Spec.Template.Spec.Containers))
			}

			expectArgs := ingestorArgsFrom(tc.opts)
			var found bool
			for _, c := range ingester.Spec.Template.Spec.Containers {
				if c.Name == IngestComponentName {
					found = true
					if c.Image != tc.opts.GetContainerImage() {
						t.Errorf("expected ingester statefulset to have image %s, got %s", tc.opts.GetContainerImage(), c.Image)
					}

					if !reflect.DeepEqual(c.Args, expectArgs) {
						t.Errorf("expected ingester statefulset to have args %v, got %v", expectArgs, c.Args)
					}

					if len(c.VolumeMounts) != len(tc.opts.Additional.VolumeMounts)+1 {
						if c.VolumeMounts[1].Name != "http-config" {
							t.Errorf("expected ingester to have volumemount named http-config, got %s", c.VolumeMounts[0].Name)
						}
						if c.VolumeMounts[1].MountPath != "/http-config" {
							t.Errorf("expected ingester to have volumemount mounted at /http-config, got %s", c.VolumeMounts[0].MountPath)
						}
					}
				}
			}
			if !found {
				t.Errorf("expected ingester to have container named %s", IngestComponentName)
			}
		})
	}
}

func TestNewRouterDeployment(t *testing.T) {
	extraLabels := map[string]string{
		"some-custom-label": someCustomLabelValue,
		"some-other-label":  someOtherLabelValue,
	}

	for _, tc := range []struct {
		name string
		opts RouterOptions
	}{
		{
			name: "test router deployment correctness",
			opts: RouterOptions{
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
			},
		},
		{
			name: "test additional volumemount",
			opts: RouterOptions{
				Options: manifests.Options{
					Name:      "test",
					Namespace: "ns",
					Image:     ptr.To("some-custom-image"),
					Labels: map[string]string{
						"some-custom-label":      someCustomLabelValue,
						"some-other-label":       someOtherLabelValue,
						"app.kubernetes.io/name": "expect-to-be-discarded",
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
			name: "test additional container",
			opts: RouterOptions{
				Options: manifests.Options{
					Name:      "test",
					Namespace: "ns",
					Image:     ptr.To("some-custom-image"),
					Labels: map[string]string{
						"some-custom-label":      someCustomLabelValue,
						"some-other-label":       someOtherLabelValue,
						"app.kubernetes.io/name": "expect-to-be-discarded",
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
			objectMetaLabels := GetRouterLabels(tc.opts)
			utils.ValidateNameNamespaceAndLabels(t, router, tc.opts.Name, tc.opts.Namespace, objectMetaLabels)
			utils.ValidateHasLabels(t, router, GetRouterSelectorLabels(tc.opts))
			utils.ValidateHasLabels(t, router, extraLabels)

			if router.Spec.Template.Spec.ServiceAccountName != GetRouterServiceAccountName(tc.opts) {
				t.Errorf("expected deployment to use service account %s, got %s", GetRouterServiceAccountName(tc.opts), router.Spec.Template.Spec.ServiceAccountName)
			}
			if len(router.Spec.Template.Spec.Containers) != (len(tc.opts.Additional.Containers) + 1) {
				t.Errorf("expected deployment to have %d containers, got %d", len(tc.opts.Additional.Containers)+1, len(router.Spec.Template.Spec.Containers))
			}

			expectArgs := routerArgsFrom(tc.opts)
			var found bool
			for _, c := range router.Spec.Template.Spec.Containers {
				if c.Name == RouterComponentName {
					found = true
					if c.Image != tc.opts.GetContainerImage() {
						t.Errorf("expected router deployment to have image %s, got %s", tc.opts.GetContainerImage(), c.Image)
					}

					if !reflect.DeepEqual(c.Args, expectArgs) {
						t.Errorf("expected router deployment to have args %v, got %v", expectArgs, c.Args)
					}

					if len(c.VolumeMounts) != len(tc.opts.Additional.VolumeMounts)+1 {
						if c.VolumeMounts[1].Name != "http-config" {
							t.Errorf("expected router deployment to have volumemount named http-config, got %s", c.VolumeMounts[0].Name)
						}
						if c.VolumeMounts[1].MountPath != "/http-config" {
							t.Errorf("expected router deployment to have volumemount mounted at /http-config, got %s", c.VolumeMounts[0].MountPath)
						}
					}
				}
			}
			if !found {
				t.Errorf("expected router deployment to have container named %s", RouterComponentName)
			}
		})
	}
}

func TestNewIngestorService(t *testing.T) {
	extraLabels := map[string]string{
		"some-custom-label": someCustomLabelValue,
		"some-other-label":  someOtherLabelValue,
	}

	opts := IngesterOptions{
		Options: manifests.Options{
			Name:      "test",
			Namespace: "ns",
			Labels: map[string]string{
				"some-custom-label":      someCustomLabelValue,
				"some-other-label":       someOtherLabelValue,
				"app.kubernetes.io/name": "expect-to-be-discarded",
			},
		},
	}

	for _, tc := range []struct {
		name string
		opts IngesterOptions
	}{
		{
			name: "test ingester service correctness",
			opts: opts,
		},
	} {
		t.Run(tc.name, func(t *testing.T) {
			ingester := NewIngestorService(tc.opts)
			objectMetaLabels := GetIngesterLabels(tc.opts)
			utils.ValidateNameNamespaceAndLabels(t, ingester, GetIngesterServiceName(opts), opts.Namespace, objectMetaLabels)
			utils.ValidateHasLabels(t, ingester, extraLabels)
			utils.ValidateHasLabels(t, ingester, GetIngesterSelectorLabels(tc.opts))

			if ingester.Spec.ClusterIP != corev1.ClusterIPNone {
				t.Errorf("expected store service to have ClusterIP 'None', got %s", ingester.Spec.ClusterIP)
			}
		})
	}
}

func TestNewRouterService(t *testing.T) {
	extraLabels := map[string]string{
		"some-custom-label": someCustomLabelValue,
		"some-other-label":  someOtherLabelValue,
	}
	opts := RouterOptions{
		Options: manifests.Options{
			Name:      "test",
			Namespace: "ns",
			Labels: map[string]string{
				"some-custom-label":      someCustomLabelValue,
				"some-other-label":       someOtherLabelValue,
				"app.kubernetes.io/name": "expect-to-be-discarded",
			},
		},
	}

	for _, tc := range []struct {
		name string
		opts RouterOptions
	}{
		{
			name: "test router service correctness",
			opts: opts,
		},
	} {
		t.Run(tc.name, func(t *testing.T) {
			router := NewRouterService(tc.opts)
			objectMetaLabels := GetRouterLabels(opts)
			utils.ValidateNameNamespaceAndLabels(t, router, opts.Name, opts.Namespace, objectMetaLabels)
			utils.ValidateHasLabels(t, router, extraLabels)
			utils.ValidateHasLabels(t, router, GetRouterSelectorLabels(opts))
		})
	}
}

func validateIngesterServiceAccount(t *testing.T, opts IngesterOptions, expectSA client.Object) {
	t.Helper()
	if expectSA.GetObjectKind().GroupVersionKind().Kind != "ServiceAccount" {
		t.Errorf("expected object to be a service account, got %v", expectSA.GetObjectKind().GroupVersionKind().Kind)
	}

	utils.ValidateNameNamespaceAndLabels(t, expectSA, GetIngesterServiceAccountName(opts), opts.Namespace, GetIngesterSelectorLabels(opts))
}

func validateRouterServiceAccount(t *testing.T, opts RouterOptions, expectSA client.Object) {
	t.Helper()
	if expectSA.GetObjectKind().GroupVersionKind().Kind != "ServiceAccount" {
		t.Errorf("expected object to be a service account, got %v", expectSA.GetObjectKind().GroupVersionKind().Kind)
	}

	utils.ValidateNameNamespaceAndLabels(t, expectSA, GetRouterServiceAccountName(opts), opts.Namespace, GetRouterSelectorLabels(opts))
}
