/*
Copyright 2024.

Licensed under the Apache License, Version 2.0 (the "License");
you may not use this file except in compliance with the License.
You may obtain a copy of the License at

    http://www.apache.org/licenses/LICENSE-2.0

Unless required by applicable law or agreed to in writing, software
distributed under the License is distributed on an "AS IS" BASIS,
WITHOUT WARRANTIES OR CONDITIONS OF ANY KIND, either express or implied.
See the License for the specific language governing permissions and
limitations under the License.
*/

package controller

import (
	"context"
	"testing"

	"github.com/go-logr/logr"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/thanos-community/thanos-operator/internal/pkg/manifests"

	appsv1 "k8s.io/api/apps/v1"
	corev1 "k8s.io/api/core/v1"
	"k8s.io/apimachinery/pkg/api/resource"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/runtime"
	"k8s.io/apimachinery/pkg/types"
	"k8s.io/client-go/tools/events"
	"sigs.k8s.io/controller-runtime/pkg/client"
	"sigs.k8s.io/controller-runtime/pkg/client/fake"
	"sigs.k8s.io/controller-runtime/pkg/reconcile"
)

func TestVolumeResizeReconciler_Reconcile(t *testing.T) {
	scheme := runtime.NewScheme()
	require.NoError(t, appsv1.AddToScheme(scheme))
	require.NoError(t, corev1.AddToScheme(scheme))

	tests := []struct {
		name         string
		objects      []runtime.Object
		request      reconcile.Request
		wantErr      bool
		wantResult   reconcile.Result
		validateFunc func(t *testing.T, fakeClient client.Client)
	}{
		{
			name: "should resize PVC when StatefulSet has storage annotation",
			objects: []runtime.Object{
				&appsv1.StatefulSet{
					ObjectMeta: metav1.ObjectMeta{
						Name:      "test-store",
						Namespace: "test-ns",
						Labels: map[string]string{
							manifests.ManagedByLabel:     manifests.DefaultManagedByLabel,
							manifests.PartOfLabel:        manifests.DefaultPartOfLabel,
							"app.kubernetes.io/instance": "test-instance",
						},
						Annotations: map[string]string{
							manifests.StorageSizeAnnotation: "20Gi",
						},
					},
					Spec: appsv1.StatefulSetSpec{
						ServiceName: "test-store",
						Replicas:    &[]int32{1}[0],
					},
				},
				&corev1.PersistentVolumeClaim{
					ObjectMeta: metav1.ObjectMeta{
						Name:      "data-test-store-0",
						Namespace: "test-ns",
						Labels: map[string]string{
							"app.kubernetes.io/instance": "test-instance",
						},
					},
					Spec: corev1.PersistentVolumeClaimSpec{
						Resources: corev1.VolumeResourceRequirements{
							Requests: corev1.ResourceList{
								corev1.ResourceStorage: resource.MustParse("10Gi"),
							},
						},
					},
				},
			},
			request: reconcile.Request{
				NamespacedName: types.NamespacedName{
					Name:      "test-store",
					Namespace: "test-ns",
				},
			},
			wantErr:    false,
			wantResult: reconcile.Result{},
			validateFunc: func(t *testing.T, fakeClient client.Client) {
				// After reconciliation, the PVC should be resized to 20Gi
				pvc := &corev1.PersistentVolumeClaim{}
				err := fakeClient.Get(context.Background(), types.NamespacedName{
					Name:      "data-test-store-0",
					Namespace: "test-ns",
				}, pvc)
				require.NoError(t, err)

				expectedSize := resource.MustParse("20Gi")
				actualSize := pvc.Spec.Resources.Requests[corev1.ResourceStorage]
				assert.True(t, expectedSize.Equal(actualSize),
					"Expected PVC size %s, got %s", expectedSize.String(), actualSize.String())
			},
		},
		{
			name: "should skip StatefulSet without thanos operator labels",
			objects: []runtime.Object{
				&appsv1.StatefulSet{
					ObjectMeta: metav1.ObjectMeta{
						Name:      "other-sts",
						Namespace: "test-ns",
						Labels: map[string]string{
							"app": "other-app",
						},
						Annotations: map[string]string{
							manifests.StorageSizeAnnotation: "20Gi",
						},
					},
				},
			},
			request: reconcile.Request{
				NamespacedName: types.NamespacedName{
					Name:      "other-sts",
					Namespace: "test-ns",
				},
			},
			wantErr:    false,
			wantResult: reconcile.Result{},
		},
		{
			name: "should skip StatefulSet without storage annotation",
			objects: []runtime.Object{
				&appsv1.StatefulSet{
					ObjectMeta: metav1.ObjectMeta{
						Name:      "test-store",
						Namespace: "test-ns",
						Labels: map[string]string{
							manifests.ManagedByLabel:     manifests.DefaultManagedByLabel,
							manifests.PartOfLabel:        manifests.DefaultPartOfLabel,
							"app.kubernetes.io/instance": "test-instance",
						},
					},
				},
			},
			request: reconcile.Request{
				NamespacedName: types.NamespacedName{
					Name:      "test-store",
					Namespace: "test-ns",
				},
			},
			wantErr:    false,
			wantResult: reconcile.Result{},
		},
		{
			name:    "should handle StatefulSet not found",
			objects: []runtime.Object{},
			request: reconcile.Request{
				NamespacedName: types.NamespacedName{
					Name:      "non-existent",
					Namespace: "test-ns",
				},
			},
			wantErr:    false,
			wantResult: reconcile.Result{},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			clientBuilder := fake.NewClientBuilder().WithScheme(scheme).WithRuntimeObjects(tt.objects...)
			fakeClient := clientBuilder.Build()

			reconciler := &VolumeResizeReconciler{
				Client:   fakeClient,
				Scheme:   scheme,
				logger:   logr.Discard(),
				recorder: events.NewFakeRecorder(10),
			}

			result, err := reconciler.Reconcile(context.Background(), tt.request)

			if tt.wantErr {
				assert.Error(t, err)
			} else {
				assert.NoError(t, err)
			}
			assert.Equal(t, tt.wantResult, result)

			if tt.validateFunc != nil {
				tt.validateFunc(t, fakeClient)
			}
		})
	}
}

func TestParseStorageSize(t *testing.T) {
	tests := []struct {
		name    string
		sizeStr string
		want    string
		wantErr bool
	}{
		{
			name:    "valid size with Gi",
			sizeStr: "10Gi",
			want:    "10Gi",
			wantErr: false,
		},
		{
			name:    "valid size with Mi",
			sizeStr: "1024Mi",
			want:    "1Gi",
			wantErr: false,
		},
		{
			name:    "invalid size format",
			sizeStr: "invalid",
			wantErr: true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got, err := parseStorageSize(tt.sizeStr)
			if tt.wantErr {
				assert.Error(t, err)
				return
			}
			require.NoError(t, err)
			assert.Equal(t, tt.want, got.String())
		})
	}
}
