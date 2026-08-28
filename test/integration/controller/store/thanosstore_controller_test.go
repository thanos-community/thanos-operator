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

package store

import (
	"context"
	"fmt"
	"os"
	"time"

	. "github.com/onsi/ginkgo/v2"
	. "github.com/onsi/gomega"

	monitoringthanosiov1alpha1 "github.com/thanos-community/thanos-operator/api/v1alpha1"

	"github.com/thanos-community/thanos-operator/internal/controller"
	"github.com/thanos-community/thanos-operator/internal/pkg/manifests"
	"github.com/thanos-community/thanos-operator/test/utils"

	appsv1 "k8s.io/api/apps/v1"
	corev1 "k8s.io/api/core/v1"
	resourceapi "k8s.io/apimachinery/pkg/api/resource"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/types"
	"k8s.io/utils/ptr"
	"sigs.k8s.io/controller-runtime/pkg/client"
)

var _ = Describe("ThanosStore Controller", func() {
	Context("When reconciling a resource", func() {
		const resourceName = "test-resource"

		ctx := context.Background()

		// each spec gets its own namespace so specs stay isolated and need no
		// teardown (envtest has no namespace controller to reap them anyway)
		var ns string

		BeforeEach(func() {
			By("creating a unique namespace and objstore secret")
			namespace := &corev1.Namespace{
				ObjectMeta: metav1.ObjectMeta{GenerateName: "thanos-store-test-"},
			}
			Expect(k8sClient.Create(ctx, namespace)).Should(Succeed())
			ns = namespace.Name

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

		It("should reconcile correctly", func() {
			if os.Getenv("EXCLUDE_STORE") == skipValue {
				Skip("Skipping ThanosStore controller tests")
			}
			firstShard := controller.StoreNameFromParent(resourceName, ptr.To(int32(0)))
			secondShard := controller.StoreNameFromParent(resourceName, ptr.To(int32(1)))
			thirdShard := controller.StoreNameFromParent(resourceName, ptr.To(int32(2)))
			resource := &monitoringthanosiov1alpha1.ThanosStore{
				ObjectMeta: metav1.ObjectMeta{
					Name:      resourceName,
					Namespace: ns,
					Annotations: map[string]string{
						"store-meta": "annotation",
					},
				},
				Spec: monitoringthanosiov1alpha1.ThanosStoreSpec{
					Replicas: 2,
					CommonFields: monitoringthanosiov1alpha1.CommonFields{
						Labels: map[string]string{"some-label": "xyz"},
						Annotations: map[string]string{
							"store-spec": "annotation",
						},
					},
					ShardingStrategy: monitoringthanosiov1alpha1.ShardingStrategy{
						Type:   monitoringthanosiov1alpha1.Block,
						Shards: 3,
					},
					StorageConfiguration: monitoringthanosiov1alpha1.StorageConfiguration{
						Size: resourceapi.MustParse("1Gi"),
					},
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

			By("setting up the thanos store resources", func() {
				Expect(k8sClient.Create(context.Background(), resource)).Should(Succeed())
				verifier := utils.Verifier{}.WithService().WithServiceAccount().WithStatefulSet()
				for _, shard := range []string{firstShard, secondShard, thirdShard} {
					EventuallyWithOffset(1, func() bool {
						return verifier.Verify(k8sClient, shard, ns)
					}, time.Second*10).Should(BeTrue())
				}

				EventuallyWithOffset(1, func() bool {
					return utils.VerifyStatefulSetReplicas(
						k8sClient, 2, secondShard, ns)
				}, time.Second*10).Should(BeTrue())
			})

			By("verifying store annotations", func() {
				EventuallyWithOffset(1, func() error {
					var objs []client.Object
					objs = append(objs, &corev1.ServiceAccount{}, &corev1.Service{}, &appsv1.StatefulSet{})

					expectedAnnotations := map[string]string{
						"store-meta":                    "annotation",
						"store-spec":                    "annotation",
						manifests.StorageSizeAnnotation: "1Gi",
					}

					for _, shard := range []string{firstShard, secondShard, thirdShard} {
						if !utils.VerifyAnnotations(k8sClient, objs, shard, ns, expectedAnnotations) {
							return fmt.Errorf("expected annotation %q not found", expectedAnnotations)
						}
					}

					return nil
				}, time.Minute).Should(Succeed())
			})

			By("setting correct sharding arg on thanos store", func() {
				EventuallyWithOffset(1, func() bool {
					args := `--selector.relabel-config=
- action: hashmod
  source_labels: ["__block_id"]
  target_label: shard
  modulus: 3
- action: keep
  source_labels: ["shard"]
  regex: 0`
					return utils.VerifyStatefulSetArgs(k8sClient, firstShard, ns, 0, args)
				}, time.Second*10).Should(BeTrue())
			})

			By("checking additional container", func() {
				EventuallyWithOffset(1, func() bool {
					statefulSet := &appsv1.StatefulSet{}
					if err := k8sClient.Get(ctx, types.NamespacedName{
						Name:      firstShard,
						Namespace: ns,
					}, statefulSet); err != nil {
						return false
					}

					return len(statefulSet.Spec.Template.Spec.Containers) == 2
				}, time.Second*10).Should(BeTrue())
			})

			By("setting custom caches on thanos store", func() {
				resource.Spec.IndexCacheConfig = &monitoringthanosiov1alpha1.CacheConfig{
					ExternalCacheConfig: &corev1.SecretKeySelector{
						LocalObjectReference: corev1.LocalObjectReference{
							Name: "index-cache",
						},
						Key: "index-cache.yaml",
					},
				}
				resource.Spec.CachingBucketConfig = &monitoringthanosiov1alpha1.CacheConfig{
					ExternalCacheConfig: &corev1.SecretKeySelector{
						LocalObjectReference: corev1.LocalObjectReference{
							Name: "caching-bucket",
						},
						Key: "caching-bucket.yaml",
					},
				}

				Expect(k8sClient.Update(context.Background(), resource)).Should(Succeed())

				EventuallyWithOffset(1, func() bool {
					if !utils.VerifyCfgMapOrSecretEnvVarExists(
						k8sClient,
						&appsv1.StatefulSet{},
						firstShard,
						ns,
						0,
						"INDEX_CACHE_CONFIG",
						"index-cache.yaml",
						"index-cache") {
						return false
					}

					if !utils.VerifyCfgMapOrSecretEnvVarExists(
						k8sClient,
						&appsv1.StatefulSet{},
						firstShard,
						ns,
						0,
						"CACHING_BUCKET_CONFIG",
						"caching-bucket.yaml",
						"caching-bucket") {
						return false
					}

					return true
				}, time.Second*10).Should(BeTrue())
			})

			By("ensuring old shards are cleaned up", func() {
				resource.Spec.ShardingStrategy.Shards = 1
				Expect(k8sClient.Update(ctx, resource)).Should(Succeed())

				verifier := utils.Verifier{}.WithStatefulSet().WithService().WithServiceAccount()
				updatedName := controller.StoreNameFromParent(resourceName, nil)

				EventuallyWithOffset(1, func() bool {
					return verifier.Verify(k8sClient, updatedName, ns)
				}, time.Second*10).Should(BeTrue())

				EventuallyWithOffset(1, func() bool {
					return utils.VerifyStatefulSetExists(k8sClient, firstShard, ns)
				}, time.Second*10).Should(BeFalse())

				EventuallyWithOffset(1, func() bool {
					return utils.VerifyStatefulSetExists(k8sClient, secondShard, ns)
				}, time.Second*10).Should(BeFalse())

				EventuallyWithOffset(1, func() bool {
					return utils.VerifyStatefulSetExists(k8sClient, thirdShard, ns)
				}, time.Second*10).Should(BeFalse())
			})

		})
	})
})
