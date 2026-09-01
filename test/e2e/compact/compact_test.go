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

package compact

import (
	"context"
	"time"

	. "github.com/onsi/ginkgo/v2"
	. "github.com/onsi/gomega"

	"github.com/thanos-community/thanos-operator/api/v1alpha1"
	"github.com/thanos-community/thanos-operator/internal/pkg/manifests"
	"github.com/thanos-community/thanos-operator/internal/pkg/manifests/compact"
	"github.com/thanos-community/thanos-operator/test/e2e/suite"
	"github.com/thanos-community/thanos-operator/test/utils"

	resourceapi "k8s.io/apimachinery/pkg/api/resource"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/utils/ptr"

	"sigs.k8s.io/controller-runtime/pkg/client"
)

const compactName = "example-compact"

var _ = Describe("Thanos Compact", Ordered, func() {
	stsName := compact.Options{Options: manifests.Options{Owner: compactName}}.GetGeneratedResourceName()

	// The operator's job here is to translate the CR into the right compactor flags;
	// whether compaction actually happens is core Thanos, not this operator. So we set
	// a distinctive, non-default config and assert those flags land on the pod.
	Context("When ThanosCompact is created", func() {
		It("should bring up the compact statefulset", func() {
			cr := &v1alpha1.ThanosCompact{
				ObjectMeta: metav1.ObjectMeta{
					Name:      compactName,
					Namespace: namespace,
				},
				Spec: v1alpha1.ThanosCompactSpec{
					CommonFields: v1alpha1.CommonFields{
						Version: suite.ThanosVersion(),
					},
					StorageConfiguration: v1alpha1.StorageConfiguration{
						Size: resourceapi.MustParse("100Mi"),
					},
					ObjectStorageConfig: suite.ObjStoreConfig(),
					RetentionConfig: v1alpha1.RetentionResolutionConfig{
						Raw:         v1alpha1.Duration("7d"),
						FiveMinutes: v1alpha1.Duration("30d"),
						OneHour:     v1alpha1.Duration("90d"),
					},
					CompactConfig: &v1alpha1.CompactConfig{
						CompactConcurrency: ptr.To(int32(4)),
						CleanupInterval:    ptr.To(v1alpha1.Duration("10m")),
						ConsistencyDelay:   ptr.To(v1alpha1.Duration("1h")),
					},
					DownsamplingConfig: &v1alpha1.DownsamplingConfig{
						Disable: ptr.To(true),
					},
				},
			}

			err := c.Create(context.Background(), cr, &client.CreateOptions{})
			Expect(err).To(BeNil())

			Eventually(func() bool {
				return utils.VerifyStatefulSetReplicasRunning(c, 1, stsName, namespace)
			}, time.Minute*5, time.Second*1).Should(BeTrue())
		})

		It("should render the configured flags onto the statefulset", func() {
			expectArgs := []string{
				"--retention.resolution-raw=7d",
				"--retention.resolution-5m=30d",
				"--retention.resolution-1h=90d",
				"--compact.concurrency=4",
				"--compact.cleanup-interval=10m",
				"--consistency-delay=1h",
				"--downsampling.disable",
			}
			Eventually(func() bool {
				return utils.VerifyStatefulSetArgs(c, stsName, namespace, 0, expectArgs...)
			}, time.Minute*1, time.Second*2).Should(BeTrue())
		})
	})
})
