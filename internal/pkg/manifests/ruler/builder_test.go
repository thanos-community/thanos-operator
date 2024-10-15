package ruler

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

func TestBuildRuler(t *testing.T) {
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
		Endpoints: []Endpoint{
			{
				ServiceName: "test-query",
				Namespace:   "ns",
				Port:        19101,
			},
		},
		RuleFiles: []corev1.ConfigMapKeySelector{
			{
				LocalObjectReference: corev1.LocalObjectReference{
					Name: "test-rules",
				},
				Key: "rules.yaml",
			},
		},
		ObjStoreSecret: corev1.SecretKeySelector{
			LocalObjectReference: corev1.LocalObjectReference{
				Name: "test-secret",
			},
			Key: "thanos.yaml",
		},
		Retention:       "15d",
		AlertmanagerURL: "http://test-alertmanager.com:9093",
		ExternalLabels: map[string]string{
			"rule_replica": "0",
		},
	}

	objs := opts.Build()
	if len(objs) != 4 {
		t.Fatalf("expected 3 objects, got %d", len(objs))
	}

	utils.ValidateIsNamedServiceAccount(t, objs[0], opts, opts.Namespace)
	expectStateful := NewRulerStatefulSet(opts)
	utils.ValidateObjectsEqual(t, objs[1], expectStateful)
	utils.ValidateObjectsEqual(t, objs[2], NewRulerService(opts))
	if objs[3].GetObjectKind().GroupVersionKind().Kind != "PodDisruptionBudget" {
		t.Errorf("expected object to be a PodDisruptionBudget, got %v", objs[3].GetObjectKind().GroupVersionKind().Kind)
	}
	utils.ValidateLabelsMatch(t, objs[3], objs[1])

	wantLabels := opts.GetSelectorLabels()
	wantLabels["some-custom-label"] = someCustomLabelValue
	wantLabels["some-other-label"] = someOtherLabelValue
	utils.ValidateObjectLabelsEqual(t, wantLabels, []client.Object{objs[1], objs[2]}...)
}

func TestNewRulerStatefulSet(t *testing.T) {
	extraLabels := map[string]string{
		"some-custom-label": someCustomLabelValue,
		"some-other-label":  someOtherLabelValue,
	}

	for _, tc := range []struct {
		name string
		opts Options
	}{
		{
			name: "test ruler statefulset correctness",
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
				Endpoints: []Endpoint{
					{
						ServiceName: "test-query",
						Namespace:   "ns",
						Port:        19101,
					},
				},
				RuleFiles: []corev1.ConfigMapKeySelector{
					{
						LocalObjectReference: corev1.LocalObjectReference{
							Name: "test-rules",
						},
						Key: "rules.yaml",
					},
				},
				ObjStoreSecret: corev1.SecretKeySelector{
					LocalObjectReference: corev1.LocalObjectReference{
						Name: "test-secret",
					},
					Key: "thanos.yaml",
				},
				Retention:       "15d",
				AlertmanagerURL: "http://test-alertmanager.com:9093",
				ExternalLabels: map[string]string{
					"rule_replica": "0",
				},
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
								Name:      "some-rule",
								MountPath: "/some-rule",
							},
						},
					},
				},
				Endpoints: []Endpoint{
					{
						ServiceName: "test-query",
						Namespace:   "ns",
						Port:        19101,
					},
				},
				RuleFiles: []corev1.ConfigMapKeySelector{
					{
						LocalObjectReference: corev1.LocalObjectReference{
							Name: "test-rules",
						},
						Key: "rules.yaml",
					},
				},
				ObjStoreSecret: corev1.SecretKeySelector{
					LocalObjectReference: corev1.LocalObjectReference{
						Name: "test-secret",
					},
					Key: "thanos.yaml",
				},
				Retention:       "15d",
				AlertmanagerURL: "http://test-alertmanager.com:9093",
				ExternalLabels: map[string]string{
					"rule_replica": "0",
				},
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
				Endpoints: []Endpoint{
					{
						ServiceName: "test-query",
						Namespace:   "ns",
						Port:        19101,
					},
				},
				RuleFiles: []corev1.ConfigMapKeySelector{
					{
						LocalObjectReference: corev1.LocalObjectReference{
							Name: "test-rules",
						},
						Key: "rules.yaml",
					},
				},
				ObjStoreSecret: corev1.SecretKeySelector{
					LocalObjectReference: corev1.LocalObjectReference{
						Name: "test-secret",
					},
					Key: "thanos.yaml",
				},
				Retention:       "15d",
				AlertmanagerURL: "http://test-alertmanager.com:9093",
				ExternalLabels: map[string]string{
					"rule_replica": "0",
				},
			},
		},
	} {
		t.Run(tc.name, func(t *testing.T) {
			name := tc.opts.GetGeneratedResourceName()
			ruler := NewRulerStatefulSet(tc.opts)
			objectMetaLabels := GetLabels(tc.opts)
			utils.ValidateNameNamespaceAndLabels(t, ruler, name, tc.opts.Namespace, objectMetaLabels)
			utils.ValidateHasLabels(t, ruler, tc.opts.GetSelectorLabels())
			utils.ValidateHasLabels(t, ruler, extraLabels)

			if ruler.Spec.ServiceName != name {
				t.Errorf("expected ruler statefulset to have serviceName %s, got %s", name, ruler.Spec.ServiceName)
			}

			if ruler.Spec.Template.Spec.ServiceAccountName != name {
				t.Errorf("expected ruler statefulset to have service account name %s, got %s", name, ruler.Spec.Template.Spec.ServiceAccountName)
			}

			if len(ruler.Spec.Template.Spec.Containers) != (len(tc.opts.Additional.Containers) + 1) {
				t.Errorf("expected ruler statefulset to have %d containers, got %d", len(tc.opts.Additional.Containers)+1, len(ruler.Spec.Template.Spec.Containers))
			}

			if ruler.Annotations["test"] != "annotation" {
				t.Errorf("expected ruler statefulset annotation test to be annotation, got %s", ruler.Annotations["test"])
			}

			expectArgs := rulerArgs(tc.opts)
			var found bool
			for _, c := range ruler.Spec.Template.Spec.Containers {
				if c.Name == Name {
					found = true
					if c.Image != tc.opts.GetContainerImage() {
						t.Errorf("expected ruler statefulset to have image %s, got %s", tc.opts.GetContainerImage(), c.Image)
					}

					if !reflect.DeepEqual(c.Args, expectArgs) {
						t.Errorf("expected ruler statefulset to have args %v, got %v", expectArgs, c.Args)
					}

					if len(c.VolumeMounts) != len(tc.opts.Additional.VolumeMounts)+1 {
						if c.VolumeMounts[0].Name != dataVolumeName {
							t.Errorf("expected ruler statefulset to have volumemount named data, got %s", c.VolumeMounts[0].Name)
						}
						if c.VolumeMounts[0].MountPath != dataVolumeMountPath {
							t.Errorf("expected ruler statefulset to have volumemount mounted at var/thanos/ruler, got %s", c.VolumeMounts[0].MountPath)
						}
					}
				}
			}
			if !found {
				t.Errorf("expected ruler statfulset to have container named %s", Name)
			}
		})
	}
}

func TestNewRulerService(t *testing.T) {
	extraLabels := map[string]string{
		"some-custom-label": someCustomLabelValue,
		"some-other-label":  someOtherLabelValue,
	}

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
		Endpoints: []Endpoint{
			{
				ServiceName: "test-query",
				Namespace:   "ns",
				Port:        19101,
			},
		},
		RuleFiles: []corev1.ConfigMapKeySelector{
			{
				LocalObjectReference: corev1.LocalObjectReference{
					Name: "test-rules",
				},
				Key: "rules.yaml",
			},
		},
		ObjStoreSecret: corev1.SecretKeySelector{
			LocalObjectReference: corev1.LocalObjectReference{
				Name: "test-secret",
			},
			Key: "thanos.yaml",
		},
		Retention:       "15d",
		AlertmanagerURL: "http://test-alertmanager.com:9093",
		ExternalLabels: map[string]string{
			"rule_replica": "0",
		},
	}

	for _, tc := range []struct {
		name string
		opts Options
	}{
		{
			name: "test ruler service correctness",
			opts: opts,
		},
	} {
		t.Run(tc.name, func(t *testing.T) {
			ruler := NewRulerService(tc.opts)
			objectMetaLabels := GetLabels(tc.opts)
			utils.ValidateNameNamespaceAndLabels(t, ruler, opts.GetGeneratedResourceName(), opts.Namespace, objectMetaLabels)
			utils.ValidateHasLabels(t, ruler, extraLabels)
			utils.ValidateHasLabels(t, ruler, tc.opts.GetSelectorLabels())

			if ruler.Spec.ClusterIP != corev1.ClusterIPNone {
				t.Errorf("expected ruler service to have ClusterIP 'None', got %s", ruler.Spec.ClusterIP)
			}
		})
	}
}
