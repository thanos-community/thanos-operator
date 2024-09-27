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
			Name:      "standalone",
		},
	}
	obj := NewService(opts)
	objectMetaLabels := GetLabels(opts)
	utils.ValidateNameNamespaceAndLabels(t, obj, GetServiceName(opts), opts.Namespace, objectMetaLabels)
	utils.ValidateHasLabels(t, obj, GetSelectorLabels(opts))

	if obj.Spec.Ports[0].Name != HTTPPortName {
		t.Errorf("expected service port name to be 'http', got %s", obj.Spec.Ports[0].Name)
	}
	if obj.Spec.Ports[0].Port != HTTPPort {
		t.Errorf("expected service port to be %d, got %d", HTTPPort, obj.Spec.Ports[0].Port)
	}

	opts = Options{
		Options: manifests.Options{
			Namespace: "any",
			Name:      "shard",
		},
		InstanceName: "test",
	}
	obj = NewService(opts)
	objectMetaLabels = GetLabels(opts)
	utils.ValidateNameNamespaceAndLabels(t, obj, GetServiceName(opts), opts.Namespace, objectMetaLabels)
	utils.ValidateHasLabels(t, obj, GetSelectorLabels(opts))
}

func TestNewStatefulSet(t *testing.T) {
	extraLabels := map[string]string{
		"some-custom-label": someCustomLabelValue,
		"some-other-label":  someOtherLabelValue,
	}

	for _, tc := range []struct {
		name       string
		opts       Options
		expectName string
	}{
		{
			name:       "test compact sts correctness with no shard name",
			expectName: "test",
			opts: Options{
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
			name:       "test compact sts correctness with no shard name",
			expectName: "some-shard",
			opts: Options{
				Options: manifests.Options{
					Name:      "some-shard",
					Namespace: "ns",
					Image:     ptr.To("some-custom-image"),
					Labels: map[string]string{
						"some-custom-label":      someCustomLabelValue,
						"some-other-label":       someOtherLabelValue,
						"app.kubernetes.io/name": "expect-to-be-discarded",
					},
				},
				InstanceName: "test",
			},
		},
	} {
		t.Run(tc.name, func(t *testing.T) {
			compact := NewStatefulSet(tc.opts)
			objectMetaLabels := GetLabels(tc.opts)

			utils.ValidateNameNamespaceAndLabels(t, compact, tc.expectName, tc.opts.Namespace, objectMetaLabels)
			utils.ValidateHasLabels(t, compact, GetSelectorLabels(tc.opts))
			utils.ValidateHasLabels(t, compact, extraLabels)

			if compact.Spec.ServiceName != GetServiceName(tc.opts) {
				t.Errorf("expected compact statefulset to have serviceName %s, got %s", GetServiceName(tc.opts), compact.Spec.ServiceName)
			}

			if compact.Spec.Template.Spec.ServiceAccountName != GetServiceAccountName(tc.opts) {
				t.Errorf("expected compact statefulset to have service account name %s, got %s", GetServiceAccountName(tc.opts), compact.Spec.Template.Spec.ServiceAccountName)
			}

			if *compact.Spec.Replicas != *ptr.To(int32(1)) {
				t.Errorf("expected compact statefulset to have 1 replica, got %d", *compact.Spec.Replicas)
			}

			if len(compact.Spec.Template.Spec.Containers) != (len(tc.opts.Additional.Containers) + 1) {
				t.Errorf("expected compact statefulset to have %d containers, got %d", len(tc.opts.Additional.Containers)+1, len(compact.Spec.Template.Spec.Containers))
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
	if len(objs) != 3 {
		t.Fatalf("expected 3 objects, got %d", len(objs))
	}

	validateServiceAccount(t, opts, objs[0])
	utils.ValidateObjectsEqual(t, objs[1], NewStatefulSet(opts))
	utils.ValidateObjectsEqual(t, objs[2], NewService(opts))

	wantLabels := GetSelectorLabels(opts)
	wantLabels["some-custom-label"] = someCustomLabelValue
	wantLabels["some-other-label"] = someOtherLabelValue
	utils.ValidateObjectLabelsEqual(t, wantLabels, []client.Object{objs[1], objs[2]}...)
}

func validateServiceAccount(t *testing.T, opts Options, expectSA client.Object) {
	t.Helper()
	if expectSA.GetObjectKind().GroupVersionKind().Kind != "ServiceAccount" {
		t.Errorf("expected object to be a service account, got %v", expectSA.GetObjectKind().GroupVersionKind().Kind)
	}

	utils.ValidateNameNamespaceAndLabels(t, expectSA, GetServiceAccountName(opts), opts.Namespace, GetSelectorLabels(opts))
}
