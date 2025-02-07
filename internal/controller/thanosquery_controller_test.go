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
	"os"
	"time"

	. "github.com/onsi/ginkgo/v2"
	. "github.com/onsi/gomega"

	monitoringthanosiov1alpha1 "github.com/thanos-community/thanos-operator/api/v1alpha1"
	"github.com/thanos-community/thanos-operator/internal/pkg/manifests"
	manifestquery "github.com/thanos-community/thanos-operator/internal/pkg/manifests/query"
	"github.com/thanos-community/thanos-operator/internal/pkg/manifests/receive"
	"github.com/thanos-community/thanos-operator/test/utils"

	appsv1 "k8s.io/api/apps/v1"
	corev1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/types"
	"k8s.io/apimachinery/pkg/util/intstr"
	"k8s.io/utils/ptr"
)

var _ = Describe("ThanosQuery Controller", Ordered, func() {
	Context("When reconciling a resource", func() {
		const (
			resourceName = "test-resource"
			ns           = "thanos-query-test"
		)

		// we use a sample receive Service to test store discovery
		const (
			receiveSvcName = "thanos-receive"
		)
		receivePort := corev1.ServicePort{
			Name:       receive.GRPCPortName,
			Port:       receive.GRPCPort,
			TargetPort: intstr.FromInt32(receive.GRPCPort),
		}

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
			resource := &monitoringthanosiov1alpha1.ThanosQuery{}
			err := k8sClient.Get(ctx, typeNamespacedName, resource)
			Expect(err).NotTo(HaveOccurred())

			By("Cleanup the specific resource instance ThanosQuery")
			Expect(k8sClient.Delete(ctx, resource)).To(Succeed())
		})

		It("should reconcile correctly", func() {
			if os.Getenv("EXCLUDE_QUERY") == skipValue {
				Skip("Skipping ThanosQuery controller tests")
			}
			name := manifestquery.Options{Options: manifests.Options{Owner: resourceName}}.GetGeneratedResourceName()
			resource := &monitoringthanosiov1alpha1.ThanosQuery{
				ObjectMeta: metav1.ObjectMeta{
					Name:      resourceName,
					Namespace: ns,
				},
				Spec: monitoringthanosiov1alpha1.ThanosQuerySpec{
					CommonFields:  monitoringthanosiov1alpha1.CommonFields{},
					Replicas:      3,
					ReplicaLabels: []string{"replica"},
					Labels:        map[string]string{"some-label": "xyz"},
					Additional: monitoringthanosiov1alpha1.Additional{
						Containers: []corev1.Container{
							{
								Name:  "jaeger-agent",
								Image: "jaegertracing/jaeger-agent:1.22",
								Args:  []string{"--reporter.grpc.host-port=jaeger-collector:14250"},
							},
						},
					},
				},
			}
			By("setting up the thanos query resources", func() {
				Expect(k8sClient.Create(context.Background(), resource)).Should(Succeed())
				verifier := utils.Verifier{}.WithDeployment().WithService().WithServiceAccount()
				EventuallyWithOffset(1, func() bool {
					return verifier.Verify(k8sClient, name, ns)
				}, time.Minute*1, time.Second*10).Should(BeTrue())
			})

			By("setting endpoints on the thanos query", func() {

				svc := &corev1.Service{
					ObjectMeta: metav1.ObjectMeta{
						Name:      receiveSvcName,
						Namespace: ns,
						Labels:    requiredStoreServiceLabels,
					},
					Spec: corev1.ServiceSpec{
						Ports: []corev1.ServicePort{receivePort},
					},
				}
				Expect(k8sClient.Create(context.Background(), svc)).Should(Succeed())
				expectArg := fmt.Sprintf("--endpoint=dnssrv+_%s._tcp.%s.%s.svc.cluster.local", receive.GRPCPortName, receiveSvcName, ns)
				EventuallyWithOffset(1, func() bool {
					return utils.VerifyDeploymentArgs(k8sClient, name, ns, 0, expectArg)
				}, time.Minute*1, time.Second*10).Should(BeTrue())
			})

			By("setting strict & ignoring services on the thanos query + additional container", func() {
				labels := requiredStoreServiceLabels
				labels[string(manifests.StrictLabel)] = manifests.DefaultStoreAPIValue

				svc := &corev1.Service{
					ObjectMeta: metav1.ObjectMeta{
						Name:      receiveSvcName,
						Namespace: ns,
						Labels:    labels,
					},
					Spec: corev1.ServiceSpec{
						Ports: []corev1.ServicePort{receivePort},
					},
				}
				Expect(k8sClient.Update(context.Background(), svc)).Should(Succeed())

				svcToIgnore := &corev1.Service{
					ObjectMeta: metav1.ObjectMeta{
						Name:      "some-svc-to-ignore-at-event-handler",
						Namespace: ns,
						Labels: map[string]string{
							"app": "nginx",
						},
					},
					Spec: corev1.ServiceSpec{
						Ports: []corev1.ServicePort{receivePort},
					},
				}
				Expect(k8sClient.Create(context.Background(), svcToIgnore)).Should(Succeed())
				EventuallyWithOffset(1, func() error {
					deployment := &appsv1.Deployment{}
					if err := k8sClient.Get(ctx, types.NamespacedName{
						Name:      name,
						Namespace: ns,
					}, deployment); err != nil {
						return err
					}

					if len(deployment.Spec.Template.Spec.Containers[0].Args) != 13 {
						return fmt.Errorf("expected 13 args, got %d: %v",
							len(deployment.Spec.Template.Spec.Containers[0].Args),
							deployment.Spec.Template.Spec.Containers[0].Args)
					}

					arg := fmt.Sprintf("--endpoint-strict=dnssrv+_%s._tcp.%s.%s.svc.cluster.local", receive.GRPCPortName, receiveSvcName, ns)
					if utils.VerifyDeploymentArgs(k8sClient, name, ns, 0, arg) == false {
						return fmt.Errorf("expected arg %q", arg)
					}

					if utils.VerifyDeploymentArgs(k8sClient, name, ns, 1, "--reporter.grpc.host-port=jaeger-collector:14250") == false {
						return fmt.Errorf("expected arg for additional container --reporter.grpc.host-port=jaeger-collector:14250")
					}

					return nil

				}, time.Minute*1, time.Second*10).Should(Succeed())
			})

			By("setting up the thanos query with query frontend", func() {
				oneh := monitoringthanosiov1alpha1.Duration("1h")
				thirtym := monitoringthanosiov1alpha1.Duration("30m")
				resource.Spec.QueryFrontend = &monitoringthanosiov1alpha1.QueryFrontendSpec{

					Replicas:          2,
					CompressResponses: true,
					QueryRangeResponseCacheConfig: &monitoringthanosiov1alpha1.CacheConfig{
						InMemoryCacheConfig: &monitoringthanosiov1alpha1.InMemoryCacheConfig{
							MaxItemSize: ptr.To(monitoringthanosiov1alpha1.StorageSize("1Mi")),
							MaxSize:     ptr.To(monitoringthanosiov1alpha1.StorageSize("1Gi")),
						},
						ExternalCacheConfig: nil,
					},
					QueryRangeSplitInterval: &oneh,
					LabelsSplitInterval:     &thirtym,
					QueryRangeMaxRetries:    10,
					LabelsMaxRetries:        5,
				}

				Expect(k8sClient.Update(context.Background(), resource)).Should(Succeed())
				verifier := utils.Verifier{}.WithDeployment().WithService().WithServiceAccount()
				EventuallyWithOffset(1, func() bool {
					return verifier.Verify(k8sClient, QueryFrontendNameFromParent(resourceName), ns)
				}, time.Minute, time.Second*2).Should(BeTrue())
			})

			By("verifying the query frontend deployment configuration", func() {
				inMemoryConfig := `type: IN-MEMORY
config:
  max_size: 1Gi
  max_item_size: 1Mi
`
				EventuallyWithOffset(1, func() error {
					expectedArgs := []string{
						"--query-frontend.compress-responses",
						fmt.Sprintf("--query-range.response-cache-config=%s", inMemoryConfig),
						fmt.Sprintf("--labels.response-cache-config=%s", inMemoryConfig),
						"--query-range.split-interval=1h",
						"--labels.split-interval=30m",
						"--query-range.max-retries-per-request=10",
						"--labels.max-retries-per-request=5",
					}

					for _, expectedArg := range expectedArgs {
						if !utils.VerifyDeploymentArgs(k8sClient, QueryFrontendNameFromParent(resourceName), ns, 0, expectedArg) {
							return fmt.Errorf("expected arg %q not found", expectedArg)
						}
					}

					return nil
				}, time.Minute, time.Second*10).Should(Succeed())
			})

			By("verifying query frontend is linked to query service", func() {
				EventuallyWithOffset(1, func() error {
					expectedArg := fmt.Sprintf("--query-frontend.downstream-url=http://%s.%s.svc.cluster.local:9090", name, ns)
					if !utils.VerifyDeploymentArgs(k8sClient, QueryFrontendNameFromParent(resourceName), ns, 0, expectedArg) {
						return fmt.Errorf("expected arg %q not found", expectedArg)
					}
					return nil
				}, time.Second*30, time.Second*10).Should(Succeed())
			})

			By("removing service monitor when disabled", func() {
				Expect(utils.VerifyServiceMonitorExists(k8sClient, name, ns)).To(BeTrue())
				resource.Spec.FeatureGates = &monitoringthanosiov1alpha1.FeatureGates{
					ServiceMonitorConfig: &monitoringthanosiov1alpha1.ServiceMonitorConfig{
						Enable: ptr.To(false),
					},
				}
				Expect(k8sClient.Update(context.Background(), resource)).Should(Succeed())

				Eventually(func() bool {
					return utils.VerifyServiceMonitorExists(k8sClient, name, ns)
				}, time.Minute*1, time.Second*10).Should(BeFalse())
			})

			By("checking paused state", func() {
				isPaused := true
				resource.Spec.Paused = &isPaused

				Expect(k8sClient.Update(context.Background(), resource)).Should(Succeed())
				labels := requiredStoreServiceLabels
				labels[string(manifests.StrictLabel)] = manifests.DefaultStoreAPIValue
				svcPaused := &corev1.Service{
					ObjectMeta: metav1.ObjectMeta{
						Name:      "paused-svc",
						Namespace: ns,
						Labels:    labels,
					},
					Spec: corev1.ServiceSpec{
						Ports: []corev1.ServicePort{receivePort},
					},
				}
				Expect(k8sClient.Create(context.Background(), svcPaused)).Should(Succeed())
				EventuallyWithOffset(1, func() error {
					deployment := &appsv1.Deployment{}
					if err := k8sClient.Get(ctx, types.NamespacedName{
						Name:      name,
						Namespace: ns,
					}, deployment); err != nil {
						return err
					}

					// If not paused would end up with 14 args.
					if len(deployment.Spec.Template.Spec.Containers[0].Args) != 13 {
						return fmt.Errorf("expected 13 args, got %d: %v",
							len(deployment.Spec.Template.Spec.Containers[0].Args),
							deployment.Spec.Template.Spec.Containers[0].Args)
					}

					return nil
				}, time.Second*10, time.Second*10).Should(Succeed())
			})
		})
	})
})
