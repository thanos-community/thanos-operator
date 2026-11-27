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

package status

import (
	"time"

	. "github.com/onsi/ginkgo/v2"
	. "github.com/onsi/gomega"

	monitoringthanosiov1alpha1 "github.com/thanos-community/thanos-operator/api/v1alpha1"
	"github.com/thanos-community/thanos-operator/internal/controller"
	"github.com/thanos-community/thanos-operator/test/utils"

	appsv1 "k8s.io/api/apps/v1"
	corev1 "k8s.io/api/core/v1"
	"k8s.io/apimachinery/pkg/api/meta"
	"k8s.io/apimachinery/pkg/api/resource"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/types"
)

const objstoreYAML = `type: S3
config:
  bucket: test
  endpoint: http://localhost:9000
  access_key: Cheesecake
  secret_key: supersecret
  http_config:
    insecure_skip_verify: false
`

var _ = Describe("ObjectStatus controller", func() {
	var ns string

	BeforeEach(func() {
		namespace := &corev1.Namespace{
			ObjectMeta: metav1.ObjectMeta{GenerateName: "object-status-test-"},
		}
		Expect(k8sClient.Create(ctx, namespace)).To(Succeed())
		ns = namespace.Name
	})

	It("updates ThanosQuery replica status when the owned Deployment status changes", func() {
		const crName = "query-status"
		deploymentName := controller.QueryNameFromParent(crName)

		Expect(k8sClient.Create(ctx, &monitoringthanosiov1alpha1.ThanosQuery{
			ObjectMeta: metav1.ObjectMeta{Name: crName, Namespace: ns},
			Spec: monitoringthanosiov1alpha1.ThanosQuerySpec{
				Replicas:      2,
				ReplicaLabels: []string{"replica"},
			},
		})).To(Succeed())

		Eventually(func(g Gomega) {
			g.Expect(utils.VerifyDeploymentExists(k8sClient, deploymentName, ns)).To(BeTrue())
		}).Should(Succeed())

		setDeploymentStatus(deploymentName, ns, appsv1.DeploymentStatus{
			Replicas:            2,
			ReadyReplicas:       2,
			AvailableReplicas:   2,
			UpdatedReplicas:     2,
			UnavailableReplicas: 0,
		})

		Eventually(func(g Gomega) {
			query := &monitoringthanosiov1alpha1.ThanosQuery{}
			g.Expect(k8sClient.Get(ctx, types.NamespacedName{Name: crName, Namespace: ns}, query)).To(Succeed())
			g.Expect(query.Status.Querier.Replicas).To(Equal(int32(2)))
			g.Expect(query.Status.Querier.ReadyReplicas).To(Equal(int32(2)))
			g.Expect(query.Status.Querier.AvailableReplicas).To(Equal(int32(2)))
			g.Expect(query.Status.Querier.UpdatedReplicas).To(Equal(int32(2)))
			g.Expect(query.Status.Querier.UnavailableReplicas).To(Equal(int32(0)))
		}).WithTimeout(time.Minute).Should(Succeed())

		By("reacting to a later Deployment status change without a CR spec update")
		setDeploymentStatus(deploymentName, ns, appsv1.DeploymentStatus{
			Replicas:            2,
			ReadyReplicas:       1,
			AvailableReplicas:   1,
			UpdatedReplicas:     2,
			UnavailableReplicas: 1,
		})

		Eventually(func(g Gomega) {
			query := &monitoringthanosiov1alpha1.ThanosQuery{}
			g.Expect(k8sClient.Get(ctx, types.NamespacedName{Name: crName, Namespace: ns}, query)).To(Succeed())
			g.Expect(query.Status.Querier.ReadyReplicas).To(Equal(int32(1)))
			g.Expect(query.Status.Querier.UnavailableReplicas).To(Equal(int32(1)))
		}).WithTimeout(time.Minute).Should(Succeed())
	})

	It("updates only the targeted ThanosQuery when one Deployment status changes", func() {
		const firstCR = "query-a"
		const secondCR = "query-b"
		firstDeploy := controller.QueryNameFromParent(firstCR)
		secondDeploy := controller.QueryNameFromParent(secondCR)

		for _, name := range []string{firstCR, secondCR} {
			Expect(k8sClient.Create(ctx, &monitoringthanosiov1alpha1.ThanosQuery{
				ObjectMeta: metav1.ObjectMeta{Name: name, Namespace: ns},
				Spec: monitoringthanosiov1alpha1.ThanosQuerySpec{
					Replicas:      1,
					ReplicaLabels: []string{"replica"},
				},
			})).To(Succeed())
		}

		Eventually(func(g Gomega) {
			g.Expect(utils.VerifyDeploymentExists(k8sClient, firstDeploy, ns)).To(BeTrue())
			g.Expect(utils.VerifyDeploymentExists(k8sClient, secondDeploy, ns)).To(BeTrue())
		}).Should(Succeed())

		setDeploymentStatus(firstDeploy, ns, appsv1.DeploymentStatus{
			Replicas:          3,
			ReadyReplicas:     3,
			AvailableReplicas: 3,
			UpdatedReplicas:   3,
		})

		Eventually(func(g Gomega) {
			first := &monitoringthanosiov1alpha1.ThanosQuery{}
			g.Expect(k8sClient.Get(ctx, types.NamespacedName{Name: firstCR, Namespace: ns}, first)).To(Succeed())
			g.Expect(first.Status.Querier.ReadyReplicas).To(Equal(int32(3)))

			second := &monitoringthanosiov1alpha1.ThanosQuery{}
			g.Expect(k8sClient.Get(ctx, types.NamespacedName{Name: secondCR, Namespace: ns}, second)).To(Succeed())
			g.Expect(second.Status.Querier.ReadyReplicas).To(BeZero())
		}).WithTimeout(time.Minute).Should(Succeed())
	})

	It("preserves Ready conditions when replica status is patched", func() {
		const crName = "query-dual-writer"
		deploymentName := controller.QueryNameFromParent(crName)

		Expect(k8sClient.Create(ctx, &monitoringthanosiov1alpha1.ThanosQuery{
			ObjectMeta: metav1.ObjectMeta{Name: crName, Namespace: ns},
			Spec: monitoringthanosiov1alpha1.ThanosQuerySpec{
				Replicas:      1,
				ReplicaLabels: []string{"replica"},
			},
		})).To(Succeed())

		Eventually(func(g Gomega) {
			g.Expect(utils.VerifyDeploymentExists(k8sClient, deploymentName, ns)).To(BeTrue())
			query := &monitoringthanosiov1alpha1.ThanosQuery{}
			g.Expect(k8sClient.Get(ctx, types.NamespacedName{Name: crName, Namespace: ns}, query)).To(Succeed())
			ready := meta.FindStatusCondition(query.Status.Conditions, controller.ConditionReady)
			g.Expect(ready).NotTo(BeNil())
			g.Expect(ready.Status).To(Equal(metav1.ConditionTrue))
		}).WithTimeout(time.Minute).Should(Succeed())

		setDeploymentStatus(deploymentName, ns, appsv1.DeploymentStatus{
			Replicas:          1,
			ReadyReplicas:     1,
			AvailableReplicas: 1,
			UpdatedReplicas:   1,
		})

		Eventually(func(g Gomega) {
			query := &monitoringthanosiov1alpha1.ThanosQuery{}
			g.Expect(k8sClient.Get(ctx, types.NamespacedName{Name: crName, Namespace: ns}, query)).To(Succeed())
			g.Expect(query.Status.Querier.ReadyReplicas).To(Equal(int32(1)))

			ready := meta.FindStatusCondition(query.Status.Conditions, controller.ConditionReady)
			g.Expect(ready).NotTo(BeNil())
			g.Expect(ready.Status).To(Equal(metav1.ConditionTrue))
			g.Expect(ready.Reason).To(Equal(controller.ReasonReconcileComplete))
		}).WithTimeout(time.Minute).Should(Succeed())
	})

	It("updates ThanosStore shard status when the owned StatefulSet status changes", func() {
		const crName = "store-status"
		statefulSetName := controller.StoreNameFromParent(crName, nil)

		Expect(k8sClient.Create(ctx, &corev1.Secret{
			ObjectMeta: metav1.ObjectMeta{Name: "thanos-objstore", Namespace: ns},
			StringData: map[string]string{"thanos.yaml": objstoreYAML},
		})).To(Succeed())

		Expect(k8sClient.Create(ctx, &monitoringthanosiov1alpha1.ThanosStore{
			ObjectMeta: metav1.ObjectMeta{Name: crName, Namespace: ns},
			Spec: monitoringthanosiov1alpha1.ThanosStoreSpec{
				Replicas: 2,
				StorageConfiguration: monitoringthanosiov1alpha1.StorageConfiguration{
					Size: resource.MustParse("1Gi"),
				},
				ObjectStorageConfig: monitoringthanosiov1alpha1.ObjectStorageConfig{
					LocalObjectReference: corev1.LocalObjectReference{Name: "thanos-objstore"},
					Key:                  "thanos.yaml",
				},
			},
		})).To(Succeed())

		Eventually(func(g Gomega) {
			g.Expect(utils.VerifyStatefulSetExists(k8sClient, statefulSetName, ns)).To(BeTrue())
		}).Should(Succeed())

		setStatefulSetStatus(statefulSetName, ns, appsv1.StatefulSetStatus{
			Replicas:          2,
			ReadyReplicas:     2,
			AvailableReplicas: 2,
			UpdatedReplicas:   2,
			CurrentReplicas:   2,
		})

		Eventually(func(g Gomega) {
			store := &monitoringthanosiov1alpha1.ThanosStore{}
			g.Expect(k8sClient.Get(ctx, types.NamespacedName{Name: crName, Namespace: ns}, store)).To(Succeed())
			shard, ok := store.Status.ShardStatuses["default"]
			g.Expect(ok).To(BeTrue())
			g.Expect(shard.Replicas).To(Equal(int32(2)))
			g.Expect(shard.ReadyReplicas).To(Equal(int32(2)))
			g.Expect(shard.AvailableReplicas).To(Equal(int32(2)))
			g.Expect(shard.CurrentReplicas).To(Equal(int32(2)))
		}).WithTimeout(time.Minute).Should(Succeed())
	})
})

func setDeploymentStatus(name, namespace string, status appsv1.DeploymentStatus) {
	deployment := &appsv1.Deployment{}
	Expect(k8sClient.Get(ctx, types.NamespacedName{Name: name, Namespace: namespace}, deployment)).To(Succeed())
	deployment.Status = status
	Expect(k8sClient.Status().Update(ctx, deployment)).To(Succeed())
}

func setStatefulSetStatus(name, namespace string, status appsv1.StatefulSetStatus) {
	statefulSet := &appsv1.StatefulSet{}
	Expect(k8sClient.Get(ctx, types.NamespacedName{Name: name, Namespace: namespace}, statefulSet)).To(Succeed())
	statefulSet.Status = status
	Expect(k8sClient.Status().Update(ctx, statefulSet)).To(Succeed())
}
