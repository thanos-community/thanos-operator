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
	"time"

	"sigs.k8s.io/controller-runtime/pkg/reconcile"

	. "github.com/onsi/ginkgo/v2"
	. "github.com/onsi/gomega"

	monitoringthanosiov1alpha1 "github.com/thanos-community/thanos-operator/api/v1alpha1"

	appsv1 "k8s.io/api/apps/v1"
	corev1 "k8s.io/api/core/v1"
	"k8s.io/apimachinery/pkg/api/errors"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/types"
)

var _ = Describe("ThanosReceive Controller", func() {
	Context("When reconciling a resource", func() {
		const (
			resourceName = "test-resource"
			ns           = "default"

			objStoreSecretName = "test-secret"
			objStoreSecretKey  = "test-key.yaml"
		)

		ctx := context.Background()

		typeNamespacedName := types.NamespacedName{
			Name:      resourceName,
			Namespace: ns,
		}

		BeforeEach(func() {
			By("creating the object store secret")
			secret := &corev1.Secret{
				ObjectMeta: metav1.ObjectMeta{
					Name:      objStoreSecretName,
					Namespace: ns,
				},
				StringData: map[string]string{
					objStoreSecretKey: `type: S3
config:
  bucket: "thanos"
  access_key: "thanos"
  secret_key: "thanos-secret"
  endpoint: "minio.default.svc:9000"
  insecure: true
  trace:
    enable: false`,
				},
			}

			err := k8sClient.Create(ctx, secret)
			if err != nil && !errors.IsAlreadyExists(err) {
				Expect(err).NotTo(HaveOccurred())
			}
		})

		AfterEach(func() {
			resource := &monitoringthanosiov1alpha1.ThanosReceive{}
			err := k8sClient.Get(ctx, typeNamespacedName, resource)
			Expect(err).NotTo(HaveOccurred())

			By("Cleanup the specific resource instance ThanosReceive")
			Expect(k8sClient.Delete(ctx, resource)).To(Succeed())
		})

		It("should error when the spec is invalid due to CEL rules", func() {
			resource := &monitoringthanosiov1alpha1.ThanosReceive{
				ObjectMeta: metav1.ObjectMeta{
					Name:      resourceName,
					Namespace: ns,
				},
				Spec: monitoringthanosiov1alpha1.ThanosReceiveSpec{
					CommonThanosFields: monitoringthanosiov1alpha1.CommonThanosFields{},
					Router: monitoringthanosiov1alpha1.RouterSpec{
						Labels:            map[string]string{"test": "my-router-test"},
						ReplicationFactor: 3,
					},
					Ingester: monitoringthanosiov1alpha1.IngesterSpec{
						DefaultObjectStorageConfig: monitoringthanosiov1alpha1.ObjectStorageConfig{
							LocalObjectReference: corev1.LocalObjectReference{Name: "test-secret"},
							Key:                  "test-key",
						},
						Hashrings: []monitoringthanosiov1alpha1.IngestorHashringSpec{
							{
								Name:        "test-hashring",
								Labels:      map[string]string{"test": "my-ingester-test"},
								StorageSize: "100Mi",
								Tenants:     []string{"test-tenant"},
								Replicas:    2,
							},
						},
					},
				},
			}
			By("failing when the receive replica count is less than the ingester replication factor", func() {
				Expect(k8sClient.Create(context.Background(), resource)).ShouldNot(Succeed())
			})

			By("ensuring hashring name is a singleton across the list", func() {
				resource.Spec.Ingester.Hashrings[0].Replicas = 3
				Expect(k8sClient.Create(context.Background(), resource)).Should(Succeed())
				resource := &monitoringthanosiov1alpha1.ThanosReceive{}
				err := k8sClient.Get(ctx, typeNamespacedName, resource)
				Expect(err).NotTo(HaveOccurred())
				resource.Spec.Ingester.Hashrings = append(
					resource.Spec.Ingester.Hashrings,
					monitoringthanosiov1alpha1.IngestorHashringSpec{
						Name:        "test-hashring",
						Labels:      map[string]string{"test": "my-ingester-test"},
						StorageSize: "100Mi",
						Tenants:     []string{"test-tenant"},
						Replicas:    5,
					},
				)
				Expect(k8sClient.Update(context.Background(), resource)).ShouldNot(Succeed())
				resource.Spec.Ingester.Hashrings[1].Name = "test-hashring-2"
				Expect(k8sClient.Update(context.Background(), resource)).Should(Succeed())
			})
		})

		It("should reconcile correctly", func() {
			resource := &monitoringthanosiov1alpha1.ThanosReceive{
				ObjectMeta: metav1.ObjectMeta{
					Name:      resourceName,
					Namespace: ns,
				},
				Spec: monitoringthanosiov1alpha1.ThanosReceiveSpec{
					CommonThanosFields: monitoringthanosiov1alpha1.CommonThanosFields{},
					Router: monitoringthanosiov1alpha1.RouterSpec{
						Labels:            map[string]string{"test": "my-router-test"},
						ReplicationFactor: 3,
					},
					Ingester: monitoringthanosiov1alpha1.IngesterSpec{
						DefaultObjectStorageConfig: monitoringthanosiov1alpha1.ObjectStorageConfig{
							LocalObjectReference: corev1.LocalObjectReference{Name: "test-secret"},
							Key:                  "test-key",
						},
						Hashrings: []monitoringthanosiov1alpha1.IngestorHashringSpec{
							{
								Name:        "test-hashring",
								Labels:      map[string]string{"test": "my-ingester-test"},
								StorageSize: "100Mi",
								Tenants:     []string{"test-tenant"},
								Replicas:    3,
							},
						},
					},
				},
			}
			By("setting up the thanos receive ingest resources", func() {
				Expect(k8sClient.Create(context.Background(), resource)).Should(Succeed())
				name := resourceName + "-test-hashring"

				controllerReconciler := &ThanosReceiveReconciler{
					Client: k8sClient,
					Scheme: k8sClient.Scheme(),
				}

				_, err := controllerReconciler.Reconcile(ctx, reconcile.Request{
					NamespacedName: typeNamespacedName,
				})
				Expect(err).NotTo(HaveOccurred())

				EventuallyWithOffset(1, func() error {
					sa := &corev1.ServiceAccount{}
					if err := k8sClient.Get(ctx, types.NamespacedName{
						Name:      name,
						Namespace: ns,
					}, sa); err != nil {
						return err
					}

					svc := &corev1.Service{}
					if err := k8sClient.Get(ctx, types.NamespacedName{
						Name:      name,
						Namespace: ns,
					}, svc); err != nil {
						return err
					}

					sts := &appsv1.StatefulSet{}
					if err := k8sClient.Get(ctx, types.NamespacedName{
						Name:      name,
						Namespace: ns,
					}, sts); err != nil {
						return err
					}
					return nil

				}, time.Minute*1, time.Second*10).Should(Succeed())
			})
		})
	})
})
