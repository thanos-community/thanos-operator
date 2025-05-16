package queryfrontend

import (
	"reflect"
	"testing"

	"github.com/thanos-community/thanos-operator/internal/pkg/manifests"
	"github.com/thanos-community/thanos-operator/test/utils"

	corev1 "k8s.io/api/core/v1"
	"k8s.io/apimachinery/pkg/api/equality"
	"k8s.io/utils/ptr"

	"sigs.k8s.io/controller-runtime/pkg/client"
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
	extraLabels := map[string]string{
		"some-custom-label": someCustomLabelValue,
		"some-other-label":  someOtherLabelValue,
	}

	for _, tc := range []struct {
		name      string
		opts      Options
		expectEnv []corev1.EnvVar
	}{
		{
			name: "test query frontend deployment correctness",
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
			name: "test with external cache",
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
			expectEnv: []corev1.EnvVar{
				{
					Name: externalCacheEnvVarName,
					ValueFrom: &corev1.EnvVarSource{
						SecretKeyRef: &corev1.SecretKeySelector{
							LocalObjectReference: corev1.LocalObjectReference{
								Name: "test-secret",
							},
						},
					},
				},
			},
		},
	} {
		t.Run(tc.name, func(t *testing.T) {
			t.Parallel()
			name := tc.opts.GetGeneratedResourceName()
			deployment := NewQueryFrontendDeployment(tc.opts)
			objectMetaLabels := GetLabels(tc.opts)
			utils.ValidateNameNamespaceAndLabels(t, deployment, name, tc.opts.Namespace, objectMetaLabels)
			utils.ValidateHasLabels(t, deployment, tc.opts.GetSelectorLabels())
			utils.ValidateHasLabels(t, deployment, extraLabels)

			if deployment.Spec.Template.Spec.ServiceAccountName != name {
				t.Errorf("expected deployment to use service account %s, got %s", name, deployment.Spec.Template.Spec.ServiceAccountName)
			}
			if len(deployment.Spec.Template.Spec.Containers) != (len(tc.opts.Additional.Containers) + 1) {
				t.Errorf("expected store statefulset to have %d containers, got %d", len(tc.opts.Additional.Containers)+1, len(deployment.Spec.Template.Spec.Containers))
			}

			if !reflect.DeepEqual(deployment.Spec.Template.Spec.Containers[0].Env, tc.expectEnv) {
				t.Errorf("expected deployment to have env %v, got %v", tc.expectEnv, deployment.Spec.Template.Spec.Containers[0].Env)
			}

			if *deployment.Spec.Replicas != tc.opts.Replicas {
				t.Errorf("expected deployment replicas to be %d, got %d", tc.opts.Replicas, *deployment.Spec.Replicas)
			}

			if deployment.Annotations["test"] != "annotation" {
				t.Errorf("expected deployment annotation test to be annotation, got %s", deployment.Annotations["test"])
			}

			// Check containers
			containers := deployment.Spec.Template.Spec.Containers
			if len(tc.opts.Additional.Containers)+1 != len(containers) {
				t.Errorf("expected deployment to have %d containers, got %d", len(tc.opts.Additional.Containers)+1, len(containers))
			}

			var found bool
			for _, c := range containers {
				if c.Name == Name {
					found = true
					if c.Image != tc.opts.GetContainerImage() {
						t.Errorf("expected container image to be %s, got %s", tc.opts.GetContainerImage(), c.Image)
					}

					expectedArgs := queryFrontendArgs(tc.opts)
					if !equality.Semantic.DeepEqual(c.Args, expectedArgs) {
						t.Errorf("expected container args to be %v, got %v", expectedArgs, c.Args)
					}

					if len(c.Ports) != 1 || c.Ports[0].ContainerPort != HTTPPort {
						t.Errorf("expected container to have 1 port (%d), got %v", HTTPPort, c.Ports)
					}

					if len(c.VolumeMounts) != len(tc.opts.Additional.VolumeMounts) {
						t.Errorf("expected query fe deployment to have 1 volumemount, got %d", len(c.VolumeMounts))
					}
					if len(c.VolumeMounts) > 0 {
						if c.VolumeMounts[0].Name != "test-sd" {
							t.Errorf("expected query fe deployment to have volumemount named test-sd, got %s", c.VolumeMounts[0].Name)
						}
						if c.VolumeMounts[0].MountPath != "/test-sd-file" {
							t.Errorf("expected query fe deployment to have volumemount mounted at /test-sd-file, got %s", c.VolumeMounts[0].MountPath)
						}
					}
				}
			}
			if !found {
				t.Errorf("expected deployment to have container named %s", Name)
			}
		})
	}
}

func TestNewQueryFrontendService(t *testing.T) {
	extraLabels := map[string]string{
		"some-custom-label": someCustomLabelValue,
		"some-other-label":  someOtherLabelValue,
	}

	opts := Options{
		Options: manifests.Options{
			Namespace: "ns",
			Labels: map[string]string{
				"some-custom-label":      someCustomLabelValue,
				"some-other-label":       someOtherLabelValue,
				"app.kubernetes.io/name": "expect-to-be-discarded",
			},
		},
	}

	service := NewQueryFrontendService(opts)
	objectMetaLabels := GetLabels(opts)
	utils.ValidateNameNamespaceAndLabels(t, service, opts.GetGeneratedResourceName(), opts.Namespace, objectMetaLabels)
	utils.ValidateHasLabels(t, service, extraLabels)
	utils.ValidateHasLabels(t, service, opts.GetSelectorLabels())

	if len(service.Spec.Ports) != 1 || service.Spec.Ports[0].Port != HTTPPort {
		t.Errorf("expected service to have 1 port (%d), got %v", HTTPPort, service.Spec.Ports)
	}
}
