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
	"time"

	. "github.com/onsi/ginkgo/v2"
	. "github.com/onsi/gomega"

	monitoringthanosiov1alpha1 "github.com/thanos-community/thanos-operator/api/v1alpha1"
	"github.com/thanos-community/thanos-operator/internal/pkg/manifests"
	"github.com/thanos-community/thanos-operator/test/utils"

	corev1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/types"
	"k8s.io/apimachinery/pkg/util/intstr"
)

var _ = Describe("ThanosQueryFrontend Controller", Ordered, func() {
	Context("When reconciling a resource", func() {
		const (
			resourceName = "test-queryfrontend"
			ns           = "tqueryfrontend"
		)

		ctx := context.Background()

		typeNamespacedName := types.NamespacedName{
			Name:      resourceName,
			Namespace: ns,
		}

		BeforeAll(func() {
			By("creating the namespace")
			Expect(k8sClient.Create(ctx, &corev1.Namespace{
				ObjectMeta: metav1.ObjectMeta{
					Name: ns,
				},
			})).Should(Succeed())
		})

		AfterEach(func() {
			resource := &monitoringthanosiov1alpha1.ThanosQueryFrontend{}
			err := k8sClient.Get(ctx, typeNamespacedName, resource)
			Expect(err).NotTo(HaveOccurred())

			By("Cleanup the specific resource instance ThanosQueryFrontend")
			Expect(k8sClient.Delete(ctx, resource)).To(Succeed())
		})

		It("should reconcile correctly", func() {
			oneh := monitoringthanosiov1alpha1.Duration("1h")
			thirtym := monitoringthanosiov1alpha1.Duration("30m")
			resource := &monitoringthanosiov1alpha1.ThanosQueryFrontend{
				ObjectMeta: metav1.ObjectMeta{
					Name:      resourceName,
					Namespace: ns,
				},
				Spec: monitoringthanosiov1alpha1.ThanosQueryFrontendSpec{
					CommonThanosFields: monitoringthanosiov1alpha1.CommonThanosFields{},
					Replicas:           2,
					CompressResponses:  true,
					QueryRangeResponseCacheConfig: &corev1.ConfigMapKeySelector{
						LocalObjectReference: corev1.LocalObjectReference{
							Name: "cache-config",
						},
						Key: "config.yaml",
					},
					QueryRangeSplitInterval: &oneh,
					LabelsSplitInterval:     &thirtym,
					QueryRangeMaxRetries:    10,
					LabelsMaxRetries:        5,
				},
			}

			By("setting up the thanos query frontend resources", func() {
				querySvc := &corev1.Service{
					ObjectMeta: metav1.ObjectMeta{
						Name:      "thanos-query",
						Namespace: ns,
						Labels: map[string]string{
							manifests.PartOfLabel:          manifests.DefaultPartOfLabel,
							manifests.DefaultQueryAPILabel: manifests.DefaultQueryAPIValue,
						},
					},
					Spec: corev1.ServiceSpec{
						Ports: []corev1.ServicePort{
							{
								Name:       "http",
								Port:       9090,
								TargetPort: intstr.FromInt(9090),
							},
						},
					},
				}
				Expect(k8sClient.Create(context.Background(), querySvc)).Should(Succeed())
				Expect(k8sClient.Create(context.Background(), resource)).Should(Succeed())

				EventuallyWithOffset(1, func() bool {
					return utils.VerifyExistenceOfRequiredNamedResources(
						k8sClient, utils.ExpectApiResourceDeployment, resourceName, ns)
				}, time.Second*30, time.Second*2).Should(BeTrue())
			})

			By("verifying the deployment configuration", func() {
				EventuallyWithOffset(1, func() error {
					expectedArgs := []string{
						"--query-frontend.compress-responses",
						"--query-range.response-cache-config=$(CACHE_CONFIG)",
						"--query-range.split-interval=1h",
						"--labels.split-interval=30m",
						"--query-range.max-retries-per-request=10",
						"--labels.max-retries-per-request=5",
					}

					for _, expectedArg := range expectedArgs {
						if !utils.VerifyDeploymentArgs(k8sClient, resourceName, ns, 0, expectedArg) {
							return fmt.Errorf("expected arg %q not found", expectedArg)
						}
					}

					return nil
				}, time.Minute, time.Second*10).Should(Succeed())
			})

			By("verifying query service is linked", func() {
				EventuallyWithOffset(1, func() error {
					if !utils.VerifyDeploymentArgs(k8sClient, resourceName, ns, 0, "--query-frontend.downstream-url=http://thanos-query."+ns+".svc.cluster.local:9090") {
						return fmt.Errorf("expected arg %q not found", "--query-frontend.downstream-url=http://thanos-query."+ns+".svc.cluster.local:9090")
					}
					return nil
				}, time.Second*30, time.Second*10).Should(Succeed())
			})
		})
	})
})
