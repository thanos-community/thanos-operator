package pdb

import (
	. "github.com/onsi/ginkgo/v2"
	. "github.com/onsi/gomega"

	monitoringthanosiov1alpha1 "github.com/thanos-community/thanos-operator/api/v1alpha1"
	"github.com/thanos-community/thanos-operator/internal/controller"
	"github.com/thanos-community/thanos-operator/internal/pkg/manifests"
	manifestquery "github.com/thanos-community/thanos-operator/internal/pkg/manifests/query"
	"github.com/thanos-community/thanos-operator/test/utils"

	corev1 "k8s.io/api/core/v1"
	"k8s.io/apimachinery/pkg/api/resource"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/utils/ptr"
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

// createNamespace creates an isolated namespace for a component's check. Each
// component gets its own namespace so the checks share nothing and stay
// collision-free and parallel-ready.
func createNamespace(ns string) {
	Expect(k8sClient.Create(ctx, &corev1.Namespace{
		ObjectMeta: metav1.ObjectMeta{Name: ns},
	})).Should(Succeed())
}

func createObjstoreSecret(ns string) {
	Expect(k8sClient.Create(ctx, &corev1.Secret{
		ObjectMeta: metav1.ObjectMeta{Name: "thanos-objstore", Namespace: ns},
		StringData: map[string]string{"thanos.yaml": objstoreYAML},
	})).Should(Succeed())
}

var _ = Describe("PodDisruptionBudget", func() {
	It("creates a PDB for ThanosQuery and removes it when scaled to one replica", func() {
		const ns = "pdb-query"
		createNamespace(ns)

		queryName := "test-query"
		name := manifestquery.Options{Options: manifests.Options{Owner: queryName}}.GetGeneratedResourceName()

		resource := &monitoringthanosiov1alpha1.ThanosQuery{
			ObjectMeta: metav1.ObjectMeta{Name: queryName, Namespace: ns},
			Spec: monitoringthanosiov1alpha1.ThanosQuerySpec{
				ReplicaLabels: []string{"replica"},
				Replicas:      2,
			},
		}
		Expect(k8sClient.Create(ctx, resource)).Should(Succeed())

		Eventually(func() bool {
			return utils.VerifyPodDisruptionBudgetExists(k8sClient, name, ns)
		}).Should(BeTrue())

		resource.Spec.Replicas = 1
		Expect(k8sClient.Update(ctx, resource)).Should(Succeed())
		Eventually(func() bool {
			return utils.VerifyPodDisruptionBudgetExists(k8sClient, name, ns)
		}).Should(BeFalse())
	})

	It("creates PDBs for ThanosReceive and removes them when scaled to one replica", func() {
		const ns = "pdb-receive"
		createNamespace(ns)

		receiveName := "test-receive"
		hashringName := "test-hashring"
		router := controller.ReceiveRouterNameFromParent(receiveName)
		ingester := controller.ReceiveIngesterNameFromParent(receiveName, hashringName)
		workloads := []string{ingester, router}

		resource := &monitoringthanosiov1alpha1.ThanosReceive{
			ObjectMeta: metav1.ObjectMeta{Name: receiveName, Namespace: ns},
			Spec: monitoringthanosiov1alpha1.ThanosReceiveSpec{
				Router: monitoringthanosiov1alpha1.RouterSpec{
					ReplicationFactor: 1,
					Replicas:          2,
				},
				Ingester: monitoringthanosiov1alpha1.IngesterSpec{
					DefaultObjectStorageConfig: monitoringthanosiov1alpha1.ObjectStorageConfig{
						LocalObjectReference: corev1.LocalObjectReference{Name: "test-secret"},
						Key:                  "test-key",
					},
					Hashrings: []monitoringthanosiov1alpha1.IngesterHashringSpec{
						{
							Name: hashringName,
							StorageConfiguration: monitoringthanosiov1alpha1.StorageConfiguration{
								Size: resource.MustParse("100Mi"),
							},
							Replicas: 3,
						},
					},
				},
			},
		}
		Expect(k8sClient.Create(ctx, resource)).Should(Succeed())

		for _, workload := range workloads {
			Eventually(func() bool {
				return utils.VerifyPodDisruptionBudgetExists(k8sClient, workload, ns)
			}).Should(BeTrue())
		}

		resource.Spec.Router.Replicas = 1
		for i := range resource.Spec.Ingester.Hashrings {
			resource.Spec.Ingester.Hashrings[i].Replicas = 1
		}
		Expect(k8sClient.Update(ctx, resource)).Should(Succeed())
		for _, workload := range workloads {
			Eventually(func() bool {
				return utils.VerifyPodDisruptionBudgetExists(k8sClient, workload, ns)
			}).Should(BeFalse())
		}
	})

	It("creates a PDB for ThanosRuler and removes it when scaled to one replica", func() {
		const ns = "pdb-ruler"
		createNamespace(ns)
		createObjstoreSecret(ns)

		// The ruler discovers query endpoints via same-namespace SD, so a query
		// must exist alongside it or reconciliation stops before the PDB is made.
		Expect(k8sClient.Create(ctx, &monitoringthanosiov1alpha1.ThanosQuery{
			ObjectMeta: metav1.ObjectMeta{Name: "test-query", Namespace: ns},
			Spec: monitoringthanosiov1alpha1.ThanosQuerySpec{
				ReplicaLabels: []string{"replica"},
				Replicas:      1,
			},
		})).Should(Succeed())

		rulerName := "test-ruler"
		name := controller.RulerNameFromParent(rulerName)
		resource := &monitoringthanosiov1alpha1.ThanosRuler{
			ObjectMeta: metav1.ObjectMeta{Name: rulerName, Namespace: ns},
			Spec: monitoringthanosiov1alpha1.ThanosRulerSpec{
				Replicas: 2,
				StorageConfiguration: monitoringthanosiov1alpha1.StorageConfiguration{
					Size: resource.MustParse("1Gi"),
				},
				RulerMode: monitoringthanosiov1alpha1.RulerMode{
					Type: "Stateful",
					Stateful: &monitoringthanosiov1alpha1.StatefulSpec{
						ObjectStorageConfig: monitoringthanosiov1alpha1.ObjectStorageConfig{
							LocalObjectReference: corev1.LocalObjectReference{Name: "thanos-objstore"},
							Key:                  "thanos.yaml",
						},
					},
				},
				RuleConfigSelector: metav1.LabelSelector{
					MatchLabels: map[string]string{
						manifests.DefaultPrometheusRuleLabel: manifests.DefaultPrometheusRuleValue,
					},
				},
				AlertmanagerURL: "http://alertmanager.com:9093",
			},
		}
		Expect(k8sClient.Create(ctx, resource)).Should(Succeed())

		Eventually(func() bool {
			return utils.VerifyPodDisruptionBudgetExists(k8sClient, name, ns)
		}).Should(BeTrue())

		resource.Spec.Replicas = 1
		Expect(k8sClient.Update(ctx, resource)).Should(Succeed())
		Eventually(func() bool {
			return utils.VerifyPodDisruptionBudgetExists(k8sClient, name, ns)
		}).Should(BeFalse())
	})

	It("creates a PDB for ThanosStore and removes it when scaled to one replica", func() {
		const ns = "pdb-store"
		createNamespace(ns)
		createObjstoreSecret(ns)

		storeName := "test-store"
		firstShard := controller.StoreNameFromParent(storeName, ptr.To(int32(0)))

		resource := &monitoringthanosiov1alpha1.ThanosStore{
			ObjectMeta: metav1.ObjectMeta{Name: storeName, Namespace: ns},
			Spec: monitoringthanosiov1alpha1.ThanosStoreSpec{
				Replicas: 2,
				ShardingStrategy: monitoringthanosiov1alpha1.ShardingStrategy{
					Type:   monitoringthanosiov1alpha1.Block,
					Shards: 3,
				},
				StorageConfiguration: monitoringthanosiov1alpha1.StorageConfiguration{
					Size: resource.MustParse("1Gi"),
				},
				ObjectStorageConfig: monitoringthanosiov1alpha1.ObjectStorageConfig{
					LocalObjectReference: corev1.LocalObjectReference{Name: "thanos-objstore"},
					Key:                  "thanos.yaml",
				},
			},
		}
		Expect(k8sClient.Create(ctx, resource)).Should(Succeed())

		Eventually(func() bool {
			return utils.VerifyPodDisruptionBudgetExists(k8sClient, firstShard, ns)
		}).Should(BeTrue())

		// When scaled below the sharding threshold the aggregate (unsharded) PDB
		// name is used for removal.
		resource.Spec.Replicas = 1
		Expect(k8sClient.Update(ctx, resource)).Should(Succeed())
		unsharded := controller.StoreNameFromParent(storeName, nil)
		Eventually(func() bool {
			return utils.VerifyPodDisruptionBudgetExists(k8sClient, unsharded, ns)
		}).Should(BeFalse())
	})
})
