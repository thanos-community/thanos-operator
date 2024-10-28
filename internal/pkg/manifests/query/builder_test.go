package query

import (
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
	extraLabels := map[string]string{
		"some-custom-label": someCustomLabelValue,
		"some-other-label":  someOtherLabelValue,
	}

	for _, tc := range []struct {
		name string
		opts Options
	}{
		{
			name: "test query deployment correctness",
			opts: Options{
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
				Timeout:       "15m",
				LookbackDelta: "5m",
				MaxConcurrent: 20,
			},
		},
		{
			name: "test additional volumemount",
			opts: Options{
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
			name: "test additional container",
			opts: Options{
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
				Timeout:       "15m",
				LookbackDelta: "5m",
				MaxConcurrent: 20,
			},
		},
	} {
		t.Run(tc.name, func(t *testing.T) {
			t.Parallel()
			name := tc.opts.GetGeneratedResourceName()
			query := NewQueryDeployment(tc.opts)
			objectMetaLabels := GetLabels(tc.opts)
			utils.ValidateNameNamespaceAndLabels(t, query, name, tc.opts.Namespace, objectMetaLabels)
			utils.ValidateHasLabels(t, query, tc.opts.GetSelectorLabels())
			utils.ValidateHasLabels(t, query, extraLabels)

			if query.Spec.Template.Spec.ServiceAccountName != name {
				t.Errorf("expected deployment to use service account %s, got %s", name, query.Spec.Template.Spec.ServiceAccountName)
			}
			if len(query.Spec.Template.Spec.Containers) != (len(tc.opts.Additional.Containers) + 1) {
				t.Errorf("expected deployment to have %d containers, got %d", len(tc.opts.Additional.Containers)+1, len(query.Spec.Template.Spec.Containers))
			}
			if len(query.Annotations) != 1 {
				t.Errorf("expected deployment to have 1 annotation, got %d", len(query.Annotations))
			}
			if query.Annotations["test"] != "annotation" {
				t.Errorf("expected deployment annotation test to be annotation, got %s", query.Annotations["test"])
			}

			expectArgs := queryArgs(tc.opts)
			var found bool
			for _, c := range query.Spec.Template.Spec.Containers {
				if c.Name == Name {
					found = true
					if c.Image != tc.opts.GetContainerImage() {
						t.Errorf("expected query deployment to have image %s, got %s", tc.opts.GetContainerImage(), c.Image)
					}
					if len(c.Args) != len(expectArgs) {
						t.Errorf("expected query deployment to have %d args, got %d", len(expectArgs), len(c.Args))
					}
					for i, arg := range c.Args {
						if arg != expectArgs[i] {
							t.Errorf("expected query deployment to have arg %s, got %s", expectArgs[i], arg)
						}
					}

					if len(c.VolumeMounts) != len(tc.opts.Additional.VolumeMounts) {
						t.Errorf("expected query deployment to have 1 volumemount, got %d", len(c.VolumeMounts))
					}
					if len(c.VolumeMounts) > 0 {
						if c.VolumeMounts[0].Name != "test-sd" {
							t.Errorf("expected query deployment to have volumemount named test-sd, got %s", c.VolumeMounts[0].Name)
						}
						if c.VolumeMounts[0].MountPath != "/test-sd-file" {
							t.Errorf("expected query deployment to have volumemount mounted at /test-sd-file, got %s", c.VolumeMounts[0].MountPath)
						}
					}

				}
			}
			if !found {
				t.Errorf("expected query deployment to have container named %s", Name)
			}
		})
	}
}

func TestNewQueryService(t *testing.T) {
	opts := Options{
		Options: manifests.Options{
			Namespace: "ns",
			Image:     ptr.To("some-custom-image"),
			Labels: map[string]string{
				"some-custom-label":      someCustomLabelValue,
				"some-other-label":       someOtherLabelValue,
				"app.kubernetes.io/name": "expect-to-be-discarded",
			},
		},
		Timeout:       "15m",
		LookbackDelta: "5m",
		MaxConcurrent: 20,
	}

	extraLabels := map[string]string{
		"some-custom-label": someCustomLabelValue,
		"some-other-label":  someOtherLabelValue,
	}

	for _, tc := range []struct {
		name string
		opts Options
	}{
		{
			name: "test query service correctness",
			opts: opts,
		},
	} {
		t.Run(tc.name, func(t *testing.T) {
			querySvc := NewQueryService(tc.opts)
			objectMetaLabels := GetLabels(tc.opts)
			utils.ValidateNameNamespaceAndLabels(t, querySvc, tc.opts.GetGeneratedResourceName(), tc.opts.Namespace, objectMetaLabels)
			utils.ValidateHasLabels(t, querySvc, extraLabels)
			utils.ValidateHasLabels(t, querySvc, tc.opts.GetSelectorLabels())
		})
	}
}
