package query

import (
	"testing"

	monitoringthanosiov1alpha1 "github.com/thanos-community/thanos-operator/api/v1alpha1"
	"github.com/thanos-community/thanos-operator/internal/pkg/manifests"

	corev1 "k8s.io/api/core/v1"
	"k8s.io/apimachinery/pkg/api/equality"
	"k8s.io/utils/ptr"
)

const (
	someCustomLabelValue string = "xyz"
	someOtherLabelValue  string = "abc"
)

func TestBuildQuerier(t *testing.T) {
	opts := QuerierOptions{
		Options: manifests.Options{
			Name:      "test",
			Namespace: "ns",
			Image:     ptr.To("some-custom-image"),
			Labels: map[string]string{
				"some-custom-label":      someCustomLabelValue,
				"some-other-label":       someOtherLabelValue,
				"app.kubernetes.io/name": "expect-to-be-discarded",
			},
		}.ApplyDefaults(),
		Timeout:       "15m",
		LookbackDelta: "5m",
		MaxConcurrent: 20,
	}

	expectSA := manifests.BuildServiceAccount(opts.Options)
	expectService := NewQuerierService(opts)
	expectDeployment := NewQuerierDeployment(opts)

	objs := BuildQuerier(opts)
	if len(objs) != 3 {
		t.Fatalf("expected 3 objects, got %d", len(objs))
	}

	if !equality.Semantic.DeepEqual(objs[0], expectSA) {
		t.Errorf("expected first object to be a service account, wanted \n%v\n got \n%v\n", expectSA, objs[0])
	}

	if !equality.Semantic.DeepEqual(objs[1], expectDeployment) {
		t.Errorf("expected second object to be a deployment, wanted \n%v\n got \n%v\n", expectDeployment, objs[2])
	}

	if !equality.Semantic.DeepEqual(objs[2], expectService) {
		t.Errorf("expected third object to be a service, wanted \n%v\n got \n%v\n", expectService, objs[1])
	}

	wantLabels := labelsForQuerier(opts)
	wantLabels["some-custom-label"] = someCustomLabelValue
	wantLabels["some-other-label"] = someOtherLabelValue

	for _, obj := range objs {
		if !equality.Semantic.DeepEqual(obj.GetLabels(), wantLabels) {
			t.Errorf("expected object to have labels %v, got %v", wantLabels, obj.GetLabels())
		}
	}
}

func TestNewQuerierDeployment(t *testing.T) {
	for _, tc := range []struct {
		name string
		opts QuerierOptions
	}{
		{
			name: "test query deployment correctness",
			opts: QuerierOptions{
				Options: manifests.Options{
					Name:      "test",
					Namespace: "ns",
					Image:     ptr.To("some-custom-image"),
					Labels: map[string]string{
						"some-custom-label":      someCustomLabelValue,
						"some-other-label":       someOtherLabelValue,
						"app.kubernetes.io/name": "expect-to-be-discarded",
					},
				}.ApplyDefaults(),
				Timeout:       "15m",
				LookbackDelta: "5m",
				MaxConcurrent: 20,
			},
		},
		{
			name: "test additional volumemount",
			opts: QuerierOptions{
				Options: manifests.Options{
					Name:      "test",
					Namespace: "ns",
					Image:     ptr.To("some-custom-image"),
					Labels: map[string]string{
						"some-custom-label":      someCustomLabelValue,
						"some-other-label":       someOtherLabelValue,
						"app.kubernetes.io/name": "expect-to-be-discarded",
					},
				}.ApplyDefaults(),
				Timeout:       "15m",
				LookbackDelta: "5m",
				MaxConcurrent: 20,
				Additional: monitoringthanosiov1alpha1.Additional{
					VolumeMounts: []corev1.VolumeMount{
						{
							Name:      "test-sd",
							MountPath: "/test-sd-file",
						},
					},
				},
			},
		},
		{
			name: "test additional container",
			opts: QuerierOptions{
				Options: manifests.Options{
					Name:      "test",
					Namespace: "ns",
					Image:     ptr.To("some-custom-image"),
					Labels: map[string]string{
						"some-custom-label":      someCustomLabelValue,
						"some-other-label":       someOtherLabelValue,
						"app.kubernetes.io/name": "expect-to-be-discarded",
					},
				}.ApplyDefaults(),
				Timeout:       "15m",
				LookbackDelta: "5m",
				MaxConcurrent: 20,
				Additional: monitoringthanosiov1alpha1.Additional{
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
	} {
		t.Run(tc.name, func(t *testing.T) {
			tc.opts.Options = tc.opts.ApplyDefaults()
			query := NewQuerierDeployment(tc.opts)
			if query.GetName() != tc.opts.Name {
				t.Errorf("expected query deployment name to be %s, got %s", tc.opts.Name, query.GetName())
			}
			if query.GetNamespace() != tc.opts.Namespace {
				t.Errorf("expected query deployment namespace to be %s, got %s", tc.opts.Namespace, query.GetNamespace())
			}
			// ensure we inherit the labels from the Options struct and that the strict labels cannot be overridden
			if len(query.GetLabels()) != 7 {
				t.Errorf("expected query deployment to have 7 labels, got %d", len(query.GetLabels()))
			}
			// ensure custom labels are set
			if query.GetLabels()["some-custom-label"] != someCustomLabelValue {
				t.Errorf("expected query deployment to have label 'some-custom-label' with value 'xyz', got %s", query.GetLabels()["some-custom-label"])
			}
			if query.GetLabels()["some-other-label"] != someOtherLabelValue {
				t.Errorf("expected query deployment to have label 'some-other-label' with value 'abc', got %s", query.GetLabels()["some-other-label"])
			}
			// ensure default labels are set
			expect := labelsForQuerier(tc.opts)
			for k, v := range expect {
				if query.GetLabels()[k] != v {
					t.Errorf("expected query deployment to have label %s with value %s, got %s", k, v, query.GetLabels()[k])
				}
			}

			if tc.name == "test additional container" && len(query.Spec.Template.Spec.Containers) != 2 {
				t.Errorf("expected query deployment to have 2 containers, got %d", len(query.Spec.Template.Spec.Containers))
			}

			expectArgs := querierArgs(tc.opts)
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

					if tc.name == "test additional volumemount" {
						if len(c.VolumeMounts) != 1 {
							t.Errorf("expected query deployment to have 1 volumemount, got %d", len(c.VolumeMounts))
						}
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

func TestNewQuerierService(t *testing.T) {
	opts := QuerierOptions{
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
		Timeout:       "15m",
		LookbackDelta: "5m",
		MaxConcurrent: 20,
	}

	for _, tc := range []struct {
		name string
		opts QuerierOptions
	}{
		{
			name: "test query service correctness",
			opts: opts,
		},
	} {
		t.Run(tc.name, func(t *testing.T) {
			tc.opts.Options = tc.opts.ApplyDefaults()
			querier := NewQuerierService(tc.opts)
			if querier.GetName() != tc.opts.Name {
				t.Errorf("expected querier service name to be %s, got %s", tc.opts.Name, querier.GetName())
			}
			if querier.GetNamespace() != tc.opts.Namespace {
				t.Errorf("expected querier service namespace to be %s, got %s", tc.opts.Namespace, querier.GetNamespace())
			}
			// ensure we inherit the labels from the Options struct and that the strict labels cannot be overridden
			if len(querier.GetLabels()) != 7 {
				t.Errorf("expected querier service to have 7 labels, got %d", len(querier.GetLabels()))
			}
			// ensure custom labels are set
			if querier.GetLabels()["some-custom-label"] != someCustomLabelValue {
				t.Errorf("expected query service to have label 'some-custom-label' with value 'xyz', got %s", querier.GetLabels()["some-custom-label"])
			}
			if querier.GetLabels()["some-other-label"] != someOtherLabelValue {
				t.Errorf("expected query service to have label 'some-other-label' with value 'abc', got %s", querier.GetLabels()["some-other-label"])
			}
			// ensure default labels are set
			expect := labelsForQuerier(tc.opts)
			for k, v := range expect {
				if querier.GetLabels()[k] != v {
					t.Errorf("expected query service to have label %s with value %s, got %s", k, v, querier.GetLabels()[k])
				}
			}

			if querier.Spec.ClusterIP != "None" {
				t.Errorf("expected query service to have ClusterIP 'None', got %s", querier.Spec.ClusterIP)
			}
		})
	}
}
