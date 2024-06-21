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
	"fmt"
	"slices"
	"time"

	. "github.com/onsi/ginkgo/v2"
	. "github.com/onsi/gomega"
	"github.com/prometheus/client_golang/prometheus"

	monitoringthanosiov1alpha1 "github.com/thanos-community/thanos-operator/api/v1alpha1"
	"github.com/thanos-community/thanos-operator/internal/pkg/manifests"
	manifestquery "github.com/thanos-community/thanos-operator/internal/pkg/manifests/query"
	"github.com/thanos-community/thanos-operator/test/utils"

	appsv1 "k8s.io/api/apps/v1"
	corev1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/types"
	"k8s.io/apimachinery/pkg/util/intstr"

	ctrl "sigs.k8s.io/controller-runtime"
	"sigs.k8s.io/controller-runtime/pkg/reconcile"
)

var _ = Describe("ThanosQuery Controller", Ordered, func() {
	Context("When reconciling a resource", func() {
		const (
			resourceName = "test-resource"
			ns           = "tquery"
		)

		ctx := context.Background()

		typeNamespacedName := types.NamespacedName{
			Name:      resourceName,
			Namespace: ns,
		}
		logger := ctrl.Log.WithName("controller-test")

		BeforeAll(func() {
			By("creating the namespace")
			Expect(k8sClient.Create(ctx, &corev1.Namespace{
				ObjectMeta: metav1.ObjectMeta{
					Name: ns,
				},
			})).Should(Succeed())
		})

		AfterEach(func() {
			resource := &monitoringthanosiov1alpha1.ThanosQuery{}
			err := k8sClient.Get(ctx, typeNamespacedName, resource)
			Expect(err).NotTo(HaveOccurred())

			By("Cleanup the specific resource instance ThanosQuery")
			Expect(k8sClient.Delete(ctx, resource)).To(Succeed())
		})

		It("should reconcile correctly", func() {
			resource := &monitoringthanosiov1alpha1.ThanosQuery{
				ObjectMeta: metav1.ObjectMeta{
					Name:      resourceName,
					Namespace: ns,
				},
				Spec: monitoringthanosiov1alpha1.ThanosQuerySpec{
					CommonThanosFields:   monitoringthanosiov1alpha1.CommonThanosFields{},
					Replicas:             3,
					QuerierReplicaLabels: []string{"replica"},
					StoreLabelSelector: &metav1.LabelSelector{MatchLabels: map[string]string{
						manifests.DefaultStoreAPILabel: manifests.DefaultStoreAPIValue,
					}},
					Labels: map[string]string{"some-label": "xyz"},
				},
			}
			By("setting up the thanos query resources", func() {
				Expect(k8sClient.Create(context.Background(), resource)).Should(Succeed())

				controllerReconciler := NewThanosQueryReconciler(logger, k8sClient, k8sClient.Scheme(), nil, prometheus.NewRegistry())

				_, err := controllerReconciler.Reconcile(ctx, reconcile.Request{
					NamespacedName: typeNamespacedName,
				})
				Expect(err).NotTo(HaveOccurred())

				EventuallyWithOffset(1, func() error {
					if !utils.VerifyServiceAccountExists(k8sClient, resourceName, ns) {
						return fmt.Errorf("service account not found")
					}

					if !utils.VerifyServiceExists(k8sClient, resourceName, ns) {
						return fmt.Errorf("service not found")
					}

					if !utils.VerifyDeploymentExists(k8sClient, resourceName, ns) {
						return fmt.Errorf("deployment not found")
					}

					return nil

				}, time.Minute*1, time.Second*10).Should(Succeed())
			})

			By("setting endpoints on the thanos query", func() {
				svc := &corev1.Service{
					ObjectMeta: metav1.ObjectMeta{
						Name:      "thanos-receive",
						Namespace: ns,
						Labels: map[string]string{
							manifests.DefaultStoreAPILabel: manifests.DefaultStoreAPIValue,
						},
					},
					Spec: corev1.ServiceSpec{
						Ports: []corev1.ServicePort{
							{
								Name:       "grpc",
								Port:       10901,
								TargetPort: intstr.FromInt(10901),
							},
						},
					},
				}
				Expect(k8sClient.Create(context.Background(), svc)).Should(Succeed())

				controllerReconciler := NewThanosQueryReconciler(logger, k8sClient, k8sClient.Scheme(), nil, prometheus.NewRegistry())

				_, err := controllerReconciler.Reconcile(ctx, reconcile.Request{
					NamespacedName: typeNamespacedName,
				})
				Expect(err).NotTo(HaveOccurred())

				EventuallyWithOffset(1, func() error {
					if !utils.VerifyServiceAccountExists(k8sClient, resourceName, ns) {
						return fmt.Errorf("service account not found")
					}

					if !utils.VerifyServiceExists(k8sClient, resourceName, ns) {
						return fmt.Errorf("service not found")
					}

					deployment := &appsv1.Deployment{}
					if err := k8sClient.Get(ctx, types.NamespacedName{
						Name:      resourceName,
						Namespace: ns,
					}, deployment); err != nil {
						return err
					}

					if !slices.Contains(deployment.Spec.Template.Spec.Containers[0].Args,
						"--endpoint=dnssrv+_grpc._tcp.thanos-receive.tquery.svc.cluster.local") {
						return fmt.Errorf("endpoint not set: %v", deployment.Spec.Template.Spec.Containers[0].Args)
					}

					return nil

				}, time.Minute*1, time.Second*10).Should(Succeed())
			})

			By("setting strict and ignoring services on the thanos query", func() {
				svc := &corev1.Service{
					ObjectMeta: metav1.ObjectMeta{
						Name:      "thanos-receive",
						Namespace: ns,
						Labels: map[string]string{
							manifests.DefaultStoreAPILabel:    manifests.DefaultStoreAPIValue,
							string(manifestquery.StrictLabel): manifests.DefaultStoreAPIValue,
						},
					},
					Spec: corev1.ServiceSpec{
						Ports: []corev1.ServicePort{
							{
								Name:       "grpc",
								Port:       10901,
								TargetPort: intstr.FromInt(10901),
							},
						},
					},
				}
				Expect(k8sClient.Update(context.Background(), svc)).Should(Succeed())

				svcToIgnore := &corev1.Service{
					ObjectMeta: metav1.ObjectMeta{
						Name:      "some-svc",
						Namespace: ns,
						Labels: map[string]string{
							"app": "nginx",
						},
					},
					Spec: corev1.ServiceSpec{
						Ports: []corev1.ServicePort{
							{
								Name:       "grpc",
								Port:       10901,
								TargetPort: intstr.FromInt(10901),
							},
						},
					},
				}
				Expect(k8sClient.Create(context.Background(), svcToIgnore)).Should(Succeed())

				controllerReconciler := NewThanosQueryReconciler(logger, k8sClient, k8sClient.Scheme(), nil, prometheus.NewRegistry())

				_, err := controllerReconciler.Reconcile(ctx, reconcile.Request{
					NamespacedName: typeNamespacedName,
				})
				Expect(err).NotTo(HaveOccurred())

				EventuallyWithOffset(1, func() error {
					if !utils.VerifyServiceAccountExists(k8sClient, resourceName, ns) {
						return fmt.Errorf("service account not found")
					}

					if !utils.VerifyServiceExists(k8sClient, resourceName, ns) {
						return fmt.Errorf("service not found")
					}

					deployment := &appsv1.Deployment{}
					if err := k8sClient.Get(ctx, types.NamespacedName{
						Name:      resourceName,
						Namespace: ns,
					}, deployment); err != nil {
						return err
					}

					if len(deployment.Spec.Template.Spec.Containers[0].Args) != 14 {
						return fmt.Errorf("expected 14 args, got %d: %v",
							len(deployment.Spec.Template.Spec.Containers[0].Args),
							deployment.Spec.Template.Spec.Containers[0].Args)
					}

					if !slices.Contains(deployment.Spec.Template.Spec.Containers[0].Args,
						"--endpoint-strict=dnssrv+_grpc._tcp.thanos-receive.tquery.svc.cluster.local") {
						return fmt.Errorf("endpoint strict not set: %v", deployment.Spec.Template.Spec.Containers[0].Args)
					}

					return nil

				}, time.Minute*1, time.Second*10).Should(Succeed())
			})

		})
	})
})
