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
	"log"
	"os"
	"time"

	. "github.com/onsi/ginkgo/v2"
	. "github.com/onsi/gomega"
	"github.com/prometheus/client_golang/prometheus"

	corev1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/types"
	ctrl "sigs.k8s.io/controller-runtime"
	"sigs.k8s.io/controller-runtime/pkg/reconcile"
)

var _ = Describe("ConfigMapToDiskReconciler", Ordered, func() {
	Context("When reconciling a resource", func() {
		const (
			resourceName = "test-resource"
			ns           = "configmap-sync"
		)

		const (
			key   = "test-key"
			value = "test-value"

			filePath = "/test.txt"
		)

		ctx := context.Background()

		typeNamespacedName := types.NamespacedName{
			Name:      resourceName,
			Namespace: ns,
		}
		logger := ctrl.Log.WithName("sync-controller-test")

		BeforeAll(func() {
			By("creating the namespace")
			Expect(k8sClient.Create(ctx, &corev1.Namespace{
				ObjectMeta: metav1.ObjectMeta{
					Name: ns,
				},
			})).Should(Succeed())
		})

		AfterEach(func() {
		})

		It("should reconcile correctly", func() {
			resource := &corev1.ConfigMap{
				ObjectMeta: metav1.ObjectMeta{
					Name:      resourceName,
					Namespace: ns,
				},
				Data: map[string]string{
					key: value,
				},
			}
			By("creating the ConfigMap and expecting the value to be written to disk", func() {
				Expect(k8sClient.Create(context.Background(), resource)).Should(Succeed())

				dir, err := os.MkdirTemp("/tmp", "test")
				if err != nil {
					log.Fatal(err)
				}
				//	defer os.RemoveAll(dir)

				opts := ConfigMapOptions{
					Name: resourceName,
					Key:  key,
					Path: dir + filePath,
				}

				controllerReconciler := NewConfigMapToDiskReconciler(logger, k8sClient, k8sClient.Scheme(), prometheus.NewRegistry(), opts)

				_, err = controllerReconciler.Reconcile(ctx, reconcile.Request{
					NamespacedName: typeNamespacedName,
				})
				Expect(err).NotTo(HaveOccurred())

				EventuallyWithOffset(1, func() bool {
					b, err := os.ReadFile(dir + filePath)
					if err != nil {
						return false
					}
					return string(b) == value
				}, time.Minute*1, time.Second*10).Should(BeTrue())

				controllerReconciler.reconciliationsFailedTotal.Collect()
			})

		})
	})
})
