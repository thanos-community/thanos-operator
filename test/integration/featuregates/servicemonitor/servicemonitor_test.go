package servicemonitor

import (
	. "github.com/onsi/ginkgo/v2"
	. "github.com/onsi/gomega"

	monitoringthanosiov1alpha1 "github.com/thanos-community/thanos-operator/api/v1alpha1"
	"github.com/thanos-community/thanos-operator/internal/controller"
	"github.com/thanos-community/thanos-operator/internal/pkg/manifests"
	"github.com/thanos-community/thanos-operator/internal/pkg/manifests/compact"
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
// component gets its own namespace: the checks are independent and share nothing,
// which keeps them collision-free and ready to run in parallel.
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

var _ = Describe("ServiceMonitor feature gate", func() {
	It("creates a ServiceMonitor for ThanosCompact", func() {
		const ns = "sm-compact"
		createNamespace(ns)
		createObjstoreSecret(ns)

		compactName := "test-compact"
		shardName := "test-shard"
		shard := compact.Options{
			Options:   manifests.Options{Owner: compactName},
			ShardName: ptr.To(shardName),
		}.GetGeneratedResourceName()

		resource := &monitoringthanosiov1alpha1.ThanosCompact{
			ObjectMeta: metav1.ObjectMeta{Name: compactName, Namespace: ns},
			Spec: monitoringthanosiov1alpha1.ThanosCompactSpec{
				ShardingConfig: []monitoringthanosiov1alpha1.ShardingConfig{
					{
						ShardName: shardName,
						ExternalLabelSharding: []monitoringthanosiov1alpha1.ExternalLabelShardingConfig{
							{Label: "tenant_id", Value: "someone"},
						},
					},
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
			return utils.VerifyServiceMonitorExists(k8sClient, shard, ns)
		}).Should(BeTrue())
	})

	It("creates a ServiceMonitor for ThanosQuery", func() {
		const ns = "sm-query"
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
			return utils.VerifyServiceMonitorExists(k8sClient, name, ns)
		}).Should(BeTrue())
	})

	It("creates ServiceMonitors for ThanosReceive ingester and router", func() {
		const ns = "sm-receive"
		createNamespace(ns)

		receiveName := "test-receive"
		hashringName := "test-hashring"
		router := controller.ReceiveRouterNameFromParent(receiveName)
		ingester := controller.ReceiveIngesterNameFromParent(receiveName, hashringName)

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

		for _, workload := range []string{ingester, router} {
			Eventually(func() bool {
				return utils.VerifyServiceMonitorExists(k8sClient, workload, ns)
			}).Should(BeTrue())
		}
	})

	It("creates a ServiceMonitor for ThanosRuler", func() {
		const ns = "sm-ruler"
		createNamespace(ns)
		createObjstoreSecret(ns)

		// The ruler discovers query endpoints via same-namespace SD, so a query
		// must exist alongside it or reconciliation stops before the SM is made.
		Expect(k8sClient.Create(ctx, &monitoringthanosiov1alpha1.ThanosQuery{
			ObjectMeta: metav1.ObjectMeta{Name: "test-query", Namespace: ns},
			Spec: monitoringthanosiov1alpha1.ThanosQuerySpec{
				ReplicaLabels: []string{"replica"},
				Replicas:      1,
			},
		})).Should(Succeed())

		rulerName := "test-ruler"
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
			return utils.VerifyServiceMonitorExists(k8sClient, controller.RulerNameFromParent(rulerName), ns)
		}).Should(BeTrue())
	})

	It("creates a ServiceMonitor for ThanosStore", func() {
		const ns = "sm-store"
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
			return utils.VerifyServiceMonitorExists(k8sClient, firstShard, ns)
		}).Should(BeTrue())
	})
})
