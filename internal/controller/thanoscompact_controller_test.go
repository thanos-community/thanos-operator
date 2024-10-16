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
	"os"
	"time"

	. "github.com/onsi/ginkgo/v2"
	. "github.com/onsi/gomega"

	monitoringthanosiov1alpha1 "github.com/thanos-community/thanos-operator/api/v1alpha1"
	"github.com/thanos-community/thanos-operator/internal/pkg/manifests"
	"github.com/thanos-community/thanos-operator/internal/pkg/manifests/compact"
	"github.com/thanos-community/thanos-operator/test/utils"

	appsv1 "k8s.io/api/apps/v1"
	corev1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/types"
	"k8s.io/utils/ptr"
)

var _ = Describe("ThanosCompact Controller", Ordered, func() {
	Context("When reconciling a resource", func() {
		const (
			ns           = "thanos-compact-test"
			resourceName = "test-compact-resource"
			shardName    = "test-shard"
		)

		ctx := context.Background()

		typeNamespacedName := types.NamespacedName{
			Name:      resourceName,
			Namespace: ns,
		}

		shardOne := compact.Options{
			Options:   manifests.Options{Owner: resourceName},
			ShardName: ptr.To(shardName), ShardIndex: ptr.To(0)}.GetGeneratedResourceName()
		shardTwo := compact.Options{
			Options:   manifests.Options{Owner: resourceName},
			ShardName: ptr.To(shardName), ShardIndex: ptr.To(1)}.GetGeneratedResourceName()

		BeforeAll(func() {
			By("creating the namespace and objstore secret")
			Expect(k8sClient.Create(ctx, &corev1.Namespace{
				ObjectMeta: metav1.ObjectMeta{
					Name: ns,
				},
			})).Should(Succeed())

			Expect(k8sClient.Create(ctx, &corev1.Secret{
				ObjectMeta: metav1.ObjectMeta{
					Name:      "thanos-objstore",
					Namespace: ns,
				},
				StringData: map[string]string{
					"thanos.yaml": `type: S3
config:
  bucket: test
  endpoint: http://localhost:9000
  access_key: Cheesecake
  secret_key: supersecret
  http_config:
    insecure_skip_verify: false
`,
				},
			})).Should(Succeed())
		})

		AfterEach(func() {
			resource := &monitoringthanosiov1alpha1.ThanosCompact{}
			err := k8sClient.Get(ctx, typeNamespacedName, resource)
			Expect(err).NotTo(HaveOccurred())

			By("Cleanup the specific resource instance ThanosCompact")
			Expect(k8sClient.Delete(ctx, resource)).To(Succeed())
		})

		It("should reconcile correctly", func() {
			if os.Getenv("EXCLUDE_COMPACT") == skipValue {
				Skip("Skipping ThanosCompact controller tests")
			}

			resource := &monitoringthanosiov1alpha1.ThanosCompact{
				ObjectMeta: metav1.ObjectMeta{
					Name:      resourceName,
					Namespace: ns,
				},
				Spec: monitoringthanosiov1alpha1.ThanosCompactSpec{
					CommonThanosFields: monitoringthanosiov1alpha1.CommonThanosFields{},
					Labels:             map[string]string{"some-label": "xyz"},
					ShardingConfig: &monitoringthanosiov1alpha1.ShardingConfig{
						ExternalLabelSharding: []monitoringthanosiov1alpha1.ExternalLabelShardingConfig{
							{
								ShardName: shardName,
								Label:     "tenant_id",
								Values:    []string{"someone", "anyone-else"},
							},
						},
					},
					StorageSize: "1Gi",
					ObjectStorageConfig: monitoringthanosiov1alpha1.ObjectStorageConfig{
						LocalObjectReference: corev1.LocalObjectReference{
							Name: "thanos-objstore",
						},
						Key: "thanos.yaml",
					},
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

			By("setting up the thanos compact resources", func() {
				Expect(k8sClient.Create(context.Background(), resource)).Should(Succeed())
				verifier := utils.Verifier{}.WithStatefulSet().WithService().WithServiceAccount()
				for _, shard := range []string{shardOne, shardTwo} {
					EventuallyWithOffset(1, func() bool {
						return verifier.Verify(k8sClient, shard, ns)
					}, time.Second*10, time.Second*2).Should(BeTrue())
				}

				for _, shard := range []string{shardOne, shardTwo} {
					EventuallyWithOffset(1, func() bool {
						return utils.VerifyStatefulSetReplicas(
							k8sClient, 1, shard, ns)
					}, time.Second*10, time.Second*2).Should(BeTrue())
				}
			})

			By("setting correct sharding arg on thanos compact", func() {
				EventuallyWithOffset(1, func() bool {
					args := `--selector.relabel-config=
- action: keep
  source_labels: ["tenant_id"]
  regex: someone`
					return utils.VerifyStatefulSetArgs(k8sClient, shardOne, ns, 0, args)
				}, time.Second*10, time.Second*2).Should(BeTrue())

				EventuallyWithOffset(1, func() bool {
					args := `--selector.relabel-config=
- action: keep
  source_labels: ["tenant_id"]
  regex: anyone-else`
					return utils.VerifyStatefulSetArgs(k8sClient, shardTwo, ns, 0, args)
				}, time.Second*10, time.Second*2).Should(BeTrue())
			})

			By("checking additional container", func() {
				EventuallyWithOffset(1, func() bool {
					statefulSet := &appsv1.StatefulSet{}
					if err := k8sClient.Get(ctx, types.NamespacedName{
						Name:      shardOne,
						Namespace: ns,
					}, statefulSet); err != nil {
						return false
					}

					return len(statefulSet.Spec.Template.Spec.Containers) == 2
				}, time.Second*10, time.Second*2).Should(BeTrue())
			})

			By("removing service monitor when disabled", func() {
				for _, shard := range []string{shardOne, shardTwo} {
					Expect(utils.VerifyServiceMonitorExists(k8sClient, shard, ns)).To(BeTrue())
				}

				enableSelfMonitor := false
				resource.Spec.CommonThanosFields = monitoringthanosiov1alpha1.CommonThanosFields{
					ServiceMonitorConfig: &monitoringthanosiov1alpha1.ServiceMonitorConfig{
						Enable: &enableSelfMonitor,
					},
				}
				Expect(k8sClient.Update(context.Background(), resource)).Should(Succeed())

				Eventually(func() bool {
					for _, shard := range []string{shardOne, shardTwo} {
						if utils.VerifyServiceMonitorExists(k8sClient, shard, ns) {
							return true
						}
					}
					return false
				}, time.Minute*1, time.Second*10).Should(BeFalse())
			})

			By("ensuring old shards are cleaned up", func() {
				resource.Spec.ShardingConfig = nil
				Expect(k8sClient.Update(ctx, resource)).Should(Succeed())
				verifier := utils.Verifier{}.WithStatefulSet().WithService().WithServiceAccount()

				EventuallyWithOffset(1, func() bool {
					for _, shard := range []string{shardOne, shardTwo} {
						if verifier.Verify(k8sClient, shard, ns) {
							return true
						}
					}
					return false
				}, time.Second*10, time.Second*2).Should(BeFalse())

				EventuallyWithOffset(1, func() bool {
					name := compact.Options{Options: manifests.Options{Owner: resourceName}}.GetGeneratedResourceName()
					return verifier.Verify(k8sClient, name, ns)
				}, time.Second*10, time.Second*2).Should(BeTrue())
			})
		})
	})
})
