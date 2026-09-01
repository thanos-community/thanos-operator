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
	"time"

	. "github.com/onsi/ginkgo/v2"
	. "github.com/onsi/gomega"

	"github.com/thanos-community/thanos-operator/api/v1alpha1"
	"github.com/thanos-community/thanos-operator/internal/controller"
	"github.com/thanos-community/thanos-operator/internal/pkg/manifests"
	"github.com/thanos-community/thanos-operator/test/e2e/suite"
	"github.com/thanos-community/thanos-operator/test/utils"

	appsv1 "k8s.io/api/apps/v1"
	corev1 "k8s.io/api/core/v1"
	apierrors "k8s.io/apimachinery/pkg/api/errors"
	resourceapi "k8s.io/apimachinery/pkg/api/resource"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/types"
	"k8s.io/utils/ptr"

	"sigs.k8s.io/controller-runtime/pkg/client"
)

const storeName = "example-store"

// shard returns the generated resource name for a store shard index (matches the
// service and statefulset names the operator creates for a multi-shard store).
func shard(i int32) string {
	return controller.StoreNameFromParent(storeName, ptr.To(i))
}

// relabelConfig is the sharding relabel-config arg the operator sets on a shard: a
// hashmod over __block_id by the shard count, keeping only this shard's slice.
func relabelConfig(shardCount, shardIndex int) string {
	return fmt.Sprintf(`--selector.relabel-config=
- action: hashmod
  source_labels: ["__block_id"]
  target_label: shard
  modulus: %d
- action: keep
  source_labels: ["shard"]
  regex: %d`, shardCount, shardIndex)
}

var _ = Describe("Thanos Store", Ordered, func() {
	Context("When a sharded, HA ThanosStore is created", func() {
		It("should bring up all shards with the configured flags", func() {
			cr := &v1alpha1.ThanosStore{
				ObjectMeta: metav1.ObjectMeta{
					Name:      storeName,
					Namespace: namespace,
				},
				Spec: v1alpha1.ThanosStoreSpec{
					Replicas: 2,
					CommonFields: v1alpha1.CommonFields{
						Version: suite.ThanosVersion(),
						Labels:  map[string]string{"some-label": "xyz"},
					},
					ShardingStrategy: v1alpha1.ShardingStrategy{
						Type:   v1alpha1.Block,
						Shards: 3,
					},
					IgnoreDeletionMarksDelay: v1alpha1.Duration("48h"),
					StoreLimitsOptions: &v1alpha1.StoreLimitsOptions{
						StoreLimitsRequestSamples: 1000,
						StoreLimitsRequestSeries:  500,
					},
					BlockConfig: &v1alpha1.BlockConfig{
						BlockDiscoveryStrategy:    v1alpha1.BlockDiscoveryStrategy("recursive"),
						BlockMetaFetchConcurrency: ptr.To(int32(16)),
					},
					StorageConfiguration: v1alpha1.StorageConfiguration{
						Size: resourceapi.MustParse("100Mi"),
					},
					ObjectStorageConfig: suite.ObjStoreConfig(),
				},
			}
			Expect(c.Create(context.Background(), cr, &client.CreateOptions{})).To(Succeed())

			// Each shard runs 2 replicas (HA).
			for i := int32(0); i < 3; i++ {
				Eventually(func() bool {
					return utils.VerifyStatefulSetReplicasRunning(c, 2, shard(i), namespace)
				}, time.Minute*5, time.Second*2).Should(BeTrue())
			}

			// Each shard keeps only its own slice via a modulus-3 hashmod.
			for i := 0; i < 3; i++ {
				Eventually(func() bool {
					return utils.VerifyStatefulSetArgs(c, shard(int32(i)), namespace, 0, relabelConfig(3, i))
				}, time.Minute*5, time.Second*2).Should(BeTrue())
			}

			// The operator renders the rest of the configured options as flags.
			By("rendering the configured store flags")
			Eventually(func() bool {
				return utils.VerifyStatefulSetArgs(c, shard(0), namespace, 0,
					"--ignore-deletion-marks-delay=48h",
					"--store.limits.request-samples=1000",
					"--store.limits.request-series=500",
					"--block-discovery-strategy=recursive",
					"--block-meta-fetch-concurrency=16",
				)
			}, time.Minute*3, time.Second*2).Should(BeTrue())
		})

		It("should label each shard service for store-api group discovery", func() {
			// The operator's job is to expose each shard's service with the labels a
			// query keys off to discover it as an HA endpoint group. Replicas > 1 adds
			// the group label so discovery load-balances across the shard's replicas.
			for i := int32(0); i < 3; i++ {
				Eventually(func(g Gomega) {
					svc := &corev1.Service{}
					g.Expect(c.Get(context.Background(), types.NamespacedName{Name: shard(i), Namespace: namespace}, svc)).To(Succeed())
					g.Expect(svc.Labels).To(HaveKeyWithValue(manifests.DefaultStoreAPILabel, manifests.DefaultStoreAPIValue))
					g.Expect(svc.Labels).To(HaveKeyWithValue(manifests.PartOfLabel, manifests.DefaultPartOfLabel))
					g.Expect(svc.Labels).To(HaveKeyWithValue(string(manifests.GroupLabel), "true"))
				}, time.Minute*2, time.Second*2).Should(Succeed())
			}
		})
	})

	Context("When the store is scaled down to fewer shards", func() {
		It("should prune the removed shard's resources", func() {
			updated := &v1alpha1.ThanosStore{}
			Expect(c.Get(context.Background(), types.NamespacedName{Name: storeName, Namespace: namespace}, updated)).To(Succeed())
			updated.Spec.ShardingStrategy.Shards = 2
			Expect(c.Update(context.Background(), updated)).To(Succeed())

			removed := shard(2)
			By("deleting the removed shard's statefulset")
			Eventually(func() bool {
				err := c.Get(context.Background(), types.NamespacedName{Name: removed, Namespace: namespace}, &appsv1.StatefulSet{})
				return apierrors.IsNotFound(err)
			}, time.Minute*3, time.Second*2).Should(BeTrue())

			By("deleting the removed shard's service")
			Eventually(func() bool {
				err := c.Get(context.Background(), types.NamespacedName{Name: removed, Namespace: namespace}, &corev1.Service{})
				return apierrors.IsNotFound(err)
			}, time.Minute*3, time.Second*2).Should(BeTrue())
		})

		It("should keep the remaining shards and re-shard them", func() {
			// The two surviving shards keep their identities and re-shard to modulus 2.
			for i := 0; i < 2; i++ {
				Eventually(func() bool {
					return utils.VerifyStatefulSetReplicasRunning(c, 2, shard(int32(i)), namespace) &&
						utils.VerifyStatefulSetArgs(c, shard(int32(i)), namespace, 0, relabelConfig(2, i))
				}, time.Minute*5, time.Second*2).Should(BeTrue())
			}
		})
	})

	Context("When the store replicas are scaled down to one", func() {
		It("should drop the endpoint-group label from the shard services", func() {
			updated := &v1alpha1.ThanosStore{}
			Expect(c.Get(context.Background(), types.NamespacedName{Name: storeName, Namespace: namespace}, updated)).To(Succeed())
			updated.Spec.Replicas = 1
			Expect(c.Update(context.Background(), updated)).To(Succeed())

			// A single replica is no longer HA, so the operator stops advertising the
			// shard for endpoint-group discovery while keeping it a store-api endpoint.
			for i := int32(0); i < 2; i++ {
				Eventually(func(g Gomega) {
					svc := &corev1.Service{}
					g.Expect(c.Get(context.Background(), types.NamespacedName{Name: shard(i), Namespace: namespace}, svc)).To(Succeed())
					g.Expect(svc.Labels).NotTo(HaveKey(string(manifests.GroupLabel)))
					g.Expect(svc.Labels).To(HaveKeyWithValue(manifests.DefaultStoreAPILabel, manifests.DefaultStoreAPIValue))
				}, time.Minute*3, time.Second*2).Should(Succeed())
			}
		})
	})
})
