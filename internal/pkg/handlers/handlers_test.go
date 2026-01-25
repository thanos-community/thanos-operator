package handlers

import (
	"context"
	"fmt"
	"slices"
	"testing"

	"github.com/go-logr/logr"

	appsv1 "k8s.io/api/apps/v1"
	corev1 "k8s.io/api/core/v1"
	discoveryv1 "k8s.io/api/discovery/v1"
	"k8s.io/apimachinery/pkg/api/resource"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/runtime"
	"k8s.io/client-go/kubernetes/scheme"

	"sigs.k8s.io/controller-runtime/pkg/client"
	"sigs.k8s.io/controller-runtime/pkg/client/fake"
	"sigs.k8s.io/controller-runtime/pkg/log"
)

type fakeClientWithError struct {
	client.Client
	shouldError bool
}

func (fc *fakeClientWithError) List(ctx context.Context, objs client.ObjectList, opts ...client.ListOption) error {
	if fc.shouldError {
		// after we hit the first error we should reset the flag
		fc.shouldError = false
		return fmt.Errorf("error")
	}
	return fc.Client.List(ctx, objs)
}

func (fc *fakeClientWithError) Update(ctx context.Context, obj client.Object, opts ...client.UpdateOption) error {
	if fc.shouldError {
		return fmt.Errorf("error")
	}
	return fc.Client.Update(ctx, obj)
}

func TestHandler_CreateOrUpdate(t *testing.T) {
	ctx := context.Background()
	const (
		namespace = "test"
		name      = "test"
	)

	owner := &appsv1.StatefulSet{}
	runTimeSts := &appsv1.StatefulSet{
		ObjectMeta: metav1.ObjectMeta{
			Name:      name,
			Namespace: namespace,
		},
	}

	baseObjects := []client.Object{
		runTimeSts,
		&corev1.Service{
			ObjectMeta: metav1.ObjectMeta{
				Name:      name,
				Namespace: namespace,
			},
		},
	}

	for _, tc := range []struct {
		name           string
		h              func() *Handler
		objs           []client.Object
		expectErrCount int
	}{
		{
			name: "test no errors on create fresh objects",
			h: func() *Handler {
				return &Handler{
					handler: &handler{
						client: fake.NewFakeClient(),
						scheme: scheme.Scheme,
						logger: logr.New(log.NullLogSink{}),
					},
				}
			},
			objs: baseObjects,
		},
		{
			name: "test error on update returns correct error count",
			h: func() *Handler {
				return &Handler{
					handler: &handler{
						client: &fakeClientWithError{
							Client:      fake.NewFakeClient(runTimeSts),
							shouldError: true,
						},
						scheme: scheme.Scheme,
						logger: logr.New(log.NullLogSink{}),
					},
				}
			},
			objs:           baseObjects,
			expectErrCount: 1,
		},
	} {
		t.Run(tc.name, func(t *testing.T) {
			h := tc.h()
			errCount := h.CreateOrUpdate(ctx, namespace, owner, tc.objs)
			if errCount != tc.expectErrCount {
				t.Errorf("expected %d errors, got %d", tc.expectErrCount, errCount)
			}
		})
	}

}

func TestHandler_GetEndpointSlices(t *testing.T) {
	ctx := context.Background()
	const (
		namespace = "test"
		name      = "test"
		svcName   = "svc"
	)

	for _, tc := range []struct {
		name           string
		objs           []runtime.Object
		expectErrCount int
	}{
		{
			name: "test extract correct endpoint slice",
			objs: []runtime.Object{
				&discoveryv1.EndpointSlice{
					TypeMeta: metav1.TypeMeta{
						APIVersion: "discovery.k8s.io/v1",
						Kind:       "EndpointSlice",
					},
					ObjectMeta: metav1.ObjectMeta{
						Name:      name,
						Namespace: namespace,
						Labels: map[string]string{
							"explain":                    "should be included",
							discoveryv1.LabelServiceName: svcName,
						},
					},
					AddressType: "",
					Endpoints:   nil,
					Ports:       nil,
				},
				&discoveryv1.EndpointSlice{
					TypeMeta: metav1.TypeMeta{
						APIVersion: "discovery.k8s.io/v1",
						Kind:       "EndpointSlice",
					},
					ObjectMeta: metav1.ObjectMeta{
						Name:      name,
						Namespace: "some-other-namespace",
						Labels: map[string]string{
							"explain":                    "should be excluded due to namespace",
							discoveryv1.LabelServiceName: svcName,
						},
					},
					AddressType: "",
					Endpoints:   nil,
					Ports:       nil,
				},
			},
		},
	} {
		t.Run(tc.name, func(t *testing.T) {
			h := &Handler{
				handler: &handler{
					client: fake.NewFakeClient(tc.objs...),
					scheme: scheme.Scheme,
					logger: logr.New(log.NullLogSink{}),
				},
			}
			eps, err := h.GetEndpointSlices(ctx, svcName, namespace)
			if err != nil {
				t.Errorf("unexpected error: %v", err)
			}

			if len(eps.Items) != 1 {
				t.Fatalf("expected 1 endpoint slice, got %d", len(eps.Items))
			}

			if eps.Items[0].Name != name && eps.Items[0].Namespace != namespace {
				t.Errorf("unexpected endpoint slice: %v", eps.Items[0])
			}

			if eps.Items[0].Labels[discoveryv1.LabelServiceName] != svcName && eps.Items[0].Labels["explain"] != "should be included" {
				t.Errorf("unexpected endpoint slice: %v", eps.Items[0])
			}
		})
	}
}

func TestPrune(t *testing.T) {
	tests := []struct {
		name              string
		keepResourceNames []string
		from              client.ListOption
		setup             func() *resourcePruner
		expectedError     bool
		expectedRemaining []string
	}{
		{
			name:              "DeletesOrphanedResources",
			keepResourceNames: []string{"keep-me"},
			from:              client.InNamespace("test-namespace"),
			setup: func() *resourcePruner {
				return &resourcePruner{
					handler: &handler{
						client: fake.NewFakeClient(
							&corev1.ServiceAccount{ObjectMeta: metav1.ObjectMeta{Name: "delete-me", Namespace: "test-namespace"}},
							&corev1.ServiceAccount{ObjectMeta: metav1.ObjectMeta{Name: "keep-me", Namespace: "test-namespace"}},
						),
						scheme: scheme.Scheme,
						logger: logr.New(log.NullLogSink{}),
					},
					sa: true,
				}
			},
			expectedError:     false,
			expectedRemaining: []string{"keep-me"},
		},
		{
			name:              "NoResourcesToDelete",
			keepResourceNames: []string{"keep-me"},
			from:              client.InNamespace("test-namespace"),
			setup: func() *resourcePruner {
				return &resourcePruner{
					handler: &handler{
						client: fake.NewFakeClient(
							&corev1.ServiceAccount{ObjectMeta: metav1.ObjectMeta{Name: "keep-me", Namespace: "test-namespace"}},
						),
						scheme: scheme.Scheme,
						logger: logr.New(log.NullLogSink{}),
					},
					sa: true,
				}
			},
			expectedError:     false,
			expectedRemaining: []string{"keep-me"},
		},
		{
			name:              "HandlesListError",
			keepResourceNames: []string{"keep-me"},
			from:              client.InNamespace("test-namespace"),
			setup: func() *resourcePruner {
				return &resourcePruner{
					handler: &handler{
						client: &fakeClientWithError{Client: fake.NewFakeClient(), shouldError: true},
						scheme: scheme.Scheme,
						logger: logr.New(log.NullLogSink{}),
					},
					sa: true,
				}
			},
			expectedError:     true,
			expectedRemaining: []string{},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			r := tt.setup()
			errs := r.Prune(context.Background(), tt.keepResourceNames, tt.from)
			if tt.expectedError {
				if errs == 0 {
					t.Errorf("expected error, got none")
				}
			} else {
				if errs != 0 {
					t.Fatalf("unexpected error count: %v ", errs)
				}
			}

			saList := &corev1.ServiceAccountList{}
			if err := r.client.List(context.Background(), saList, tt.from); err != nil {
				t.Fatalf("unexpected error: %v", err)
			}

			var remaining []string
			for _, sa := range saList.Items {
				remaining = append(remaining, sa.Name)
			}

			if len(remaining) != len(tt.expectedRemaining) {
				t.Errorf("expected %d remaining resources, got %d", len(tt.expectedRemaining), len(remaining))
			}

			for _, rem := range remaining {
				if !slices.Contains(tt.expectedRemaining, rem) {
					t.Errorf("unexpected remaining resource %s", rem)
				}
			}
		})
	}
}

func TestHandler_ExpandPVCsForStatefulSet(t *testing.T) {
	ctx := context.Background()
	const (
		namespace = "test"
		stsName   = "test-sts"
		pvcName   = "data-test-sts-0"
	)

	for _, tc := range []struct {
		name           string
		setup          func() (*Handler, *appsv1.StatefulSet, corev1.ResourceList)
		expectErrCount int
		validate       func(t *testing.T, h *Handler)
	}{
		{
			name: "expands PVC when desired size is larger",
			setup: func() (*Handler, *appsv1.StatefulSet, corev1.ResourceList) {
				currentSize := corev1.ResourceList{
					corev1.ResourceStorage: parseQuantity(t, "10Gi"),
				}
				desiredSize := corev1.ResourceList{
					corev1.ResourceStorage: parseQuantity(t, "20Gi"),
				}

				pvc := &corev1.PersistentVolumeClaim{
					ObjectMeta: metav1.ObjectMeta{
						Name:      pvcName,
						Namespace: namespace,
						Labels:    map[string]string{"app": "test"},
					},
					Spec: corev1.PersistentVolumeClaimSpec{
						Resources: corev1.VolumeResourceRequirements{
							Requests: currentSize,
						},
					},
				}

				sts := &appsv1.StatefulSet{
					ObjectMeta: metav1.ObjectMeta{
						Name:      stsName,
						Namespace: namespace,
					},
					Spec: appsv1.StatefulSetSpec{
						Selector: &metav1.LabelSelector{
							MatchLabels: map[string]string{"app": "test"},
						},
						VolumeClaimTemplates: []corev1.PersistentVolumeClaim{
							{
								Spec: corev1.PersistentVolumeClaimSpec{
									Resources: corev1.VolumeResourceRequirements{
										Requests: desiredSize,
									},
								},
							},
						},
					},
				}

				h := &Handler{
					handler: &handler{
						client: fake.NewFakeClient(pvc, sts),
						scheme: scheme.Scheme,
						logger: logr.New(log.NullLogSink{}),
					},
				}

				return h, sts, desiredSize
			},
			expectErrCount: 0,
			validate: func(t *testing.T, h *Handler) {
				pvc := &corev1.PersistentVolumeClaim{}
				if err := h.client.Get(ctx, client.ObjectKey{Name: pvcName, Namespace: namespace}, pvc); err != nil {
					t.Fatalf("failed to get PVC: %v", err)
				}

				actualSize := pvc.Spec.Resources.Requests[corev1.ResourceStorage]
				expectedSize := parseQuantity(t, "20Gi")

				if actualSize.Cmp(expectedSize) != 0 {
					t.Errorf("expected PVC size %s, got %s", expectedSize.String(), actualSize.String())
				}
			},
		},
		{
			name: "does not modify PVC when desired size is same",
			setup: func() (*Handler, *appsv1.StatefulSet, corev1.ResourceList) {
				currentSize := corev1.ResourceList{
					corev1.ResourceStorage: parseQuantity(t, "10Gi"),
				}

				pvc := &corev1.PersistentVolumeClaim{
					ObjectMeta: metav1.ObjectMeta{
						Name:      pvcName,
						Namespace: namespace,
						Labels:    map[string]string{"app": "test"},
					},
					Spec: corev1.PersistentVolumeClaimSpec{
						Resources: corev1.VolumeResourceRequirements{
							Requests: currentSize,
						},
					},
				}

				sts := &appsv1.StatefulSet{
					ObjectMeta: metav1.ObjectMeta{
						Name:      stsName,
						Namespace: namespace,
					},
					Spec: appsv1.StatefulSetSpec{
						Selector: &metav1.LabelSelector{
							MatchLabels: map[string]string{"app": "test"},
						},
						VolumeClaimTemplates: []corev1.PersistentVolumeClaim{
							{
								Spec: corev1.PersistentVolumeClaimSpec{
									Resources: corev1.VolumeResourceRequirements{
										Requests: currentSize,
									},
								},
							},
						},
					},
				}

				h := &Handler{
					handler: &handler{
						client: fake.NewFakeClient(pvc, sts),
						scheme: scheme.Scheme,
						logger: logr.New(log.NullLogSink{}),
					},
				}

				return h, sts, currentSize
			},
			expectErrCount: 0,
			validate: func(t *testing.T, h *Handler) {
				pvc := &corev1.PersistentVolumeClaim{}
				if err := h.client.Get(ctx, client.ObjectKey{Name: pvcName, Namespace: namespace}, pvc); err != nil {
					t.Fatalf("failed to get PVC: %v", err)
				}

				actualSize := pvc.Spec.Resources.Requests[corev1.ResourceStorage]
				expectedSize := parseQuantity(t, "10Gi")

				if actualSize.Cmp(expectedSize) != 0 {
					t.Errorf("expected PVC size %s, got %s", expectedSize.String(), actualSize.String())
				}
			},
		},
		{
			name: "does not modify PVC when desired size is smaller",
			setup: func() (*Handler, *appsv1.StatefulSet, corev1.ResourceList) {
				currentSize := corev1.ResourceList{
					corev1.ResourceStorage: parseQuantity(t, "20Gi"),
				}
				desiredSize := corev1.ResourceList{
					corev1.ResourceStorage: parseQuantity(t, "10Gi"),
				}

				pvc := &corev1.PersistentVolumeClaim{
					ObjectMeta: metav1.ObjectMeta{
						Name:      pvcName,
						Namespace: namespace,
						Labels:    map[string]string{"app": "test"},
					},
					Spec: corev1.PersistentVolumeClaimSpec{
						Resources: corev1.VolumeResourceRequirements{
							Requests: currentSize,
						},
					},
				}

				sts := &appsv1.StatefulSet{
					ObjectMeta: metav1.ObjectMeta{
						Name:      stsName,
						Namespace: namespace,
					},
					Spec: appsv1.StatefulSetSpec{
						Selector: &metav1.LabelSelector{
							MatchLabels: map[string]string{"app": "test"},
						},
						VolumeClaimTemplates: []corev1.PersistentVolumeClaim{
							{
								Spec: corev1.PersistentVolumeClaimSpec{
									Resources: corev1.VolumeResourceRequirements{
										Requests: desiredSize,
									},
								},
							},
						},
					},
				}

				h := &Handler{
					handler: &handler{
						client: fake.NewFakeClient(pvc, sts),
						scheme: scheme.Scheme,
						logger: logr.New(log.NullLogSink{}),
					},
				}

				return h, sts, desiredSize
			},
			expectErrCount: 0,
			validate: func(t *testing.T, h *Handler) {
				pvc := &corev1.PersistentVolumeClaim{}
				if err := h.client.Get(ctx, client.ObjectKey{Name: pvcName, Namespace: namespace}, pvc); err != nil {
					t.Fatalf("failed to get PVC: %v", err)
				}

				// Should remain at original size
				actualSize := pvc.Spec.Resources.Requests[corev1.ResourceStorage]
				expectedSize := parseQuantity(t, "20Gi")

				if actualSize.Cmp(expectedSize) != 0 {
					t.Errorf("expected PVC size %s (unchanged), got %s", expectedSize.String(), actualSize.String())
				}
			},
		},
	} {
		t.Run(tc.name, func(t *testing.T) {
			h, sts, desiredSize := tc.setup()
			errCount := h.ExpandPVCsForStatefulSet(ctx, sts, desiredSize)

			if errCount != tc.expectErrCount {
				t.Errorf("expected %d errors, got %d", tc.expectErrCount, errCount)
			}

			if tc.validate != nil {
				tc.validate(t, h)
			}
		})
	}
}

func parseQuantity(t *testing.T, s string) resource.Quantity {
	t.Helper()
	q, err := resource.ParseQuantity(s)
	if err != nil {
		t.Fatalf("failed to parse quantity %s: %v", s, err)
	}
	return q
}
