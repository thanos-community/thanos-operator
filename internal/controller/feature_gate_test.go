package controller

import (
	"context"
	"os"

	. "github.com/onsi/ginkgo/v2"
	. "github.com/onsi/gomega"

	monitoringthanosiov1alpha1 "github.com/thanos-community/thanos-operator/api/v1alpha1"
	"github.com/thanos-community/thanos-operator/internal/pkg/manifests"
	"github.com/thanos-community/thanos-operator/internal/pkg/manifests/compact"
	manifestquery "github.com/thanos-community/thanos-operator/internal/pkg/manifests/query"
	"github.com/thanos-community/thanos-operator/test/utils"

	corev1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/utils/ptr"
)

var _ = Describe("ServiceMonitor/PDB FeatureGate", Ordered, func() {
	const (
		ns           = "feature-gate-test"
		resourceName = "test-resource"
	)

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

	AfterAll(func() {
		By("cleaning up the namespace")
		Expect(k8sClient.Delete(ctx, &corev1.Namespace{
			ObjectMeta: metav1.ObjectMeta{
				Name: ns,
			},
		})).Should(Succeed())
	})

	Context("When reconciling a ThanosCompact", func() {
		compactResourceName := "test-compact-resource"
		shardName := "test-shard"

		shardOne := compact.Options{
			Options:   manifests.Options{Owner: compactResourceName},
			ShardName: ptr.To(shardName)}.GetGeneratedResourceName()

		It("should reconcile correctly", func() {
			if os.Getenv("EXCLUDE_COMPACT") == skipValue {
				Skip("Skipping ThanosCompact controller tests")
			}

			resource := &monitoringthanosiov1alpha1.ThanosCompact{
				ObjectMeta: metav1.ObjectMeta{
					Name:      compactResourceName,
					Namespace: ns,
				},
				Spec: monitoringthanosiov1alpha1.ThanosCompactSpec{
					CommonFields: monitoringthanosiov1alpha1.CommonFields{},
					Labels:       map[string]string{"some-label": "xyz"},
					ShardingConfig: []monitoringthanosiov1alpha1.ShardingConfig{
						{
							ShardName: shardName,
							ExternalLabelSharding: []monitoringthanosiov1alpha1.ExternalLabelShardingConfig{
								{
									Label: "tenant_id",
									Value: "someone",
								},
							},
						},
					},
					StorageConfiguration: monitoringthanosiov1alpha1.StorageConfiguration{
						Size: "1Gi",
					},
					ObjectStorageConfig: monitoringthanosiov1alpha1.ObjectStorageConfig{
						LocalObjectReference: corev1.LocalObjectReference{
							Name: "thanos-objstore",
						},
						Key: "thanos.yaml",
					},
				},
			}

			By("creating the service monitor when enabled", func() {
				Expect(k8sClient.Create(context.Background(), resource)).Should(Succeed())
				Eventually(func() bool {
					for _, shard := range []string{shardOne} {
						if utils.VerifyServiceMonitorExists(k8sClient, shard, ns) {
							return true
						}
					}
					return false
				}).Should(BeTrue())

			})

		})
	})

	Context("When reconciling a ThanosQuery", func() {
		queryResourceName := resourceName
		It("should reconcile correctly", func() {
			if os.Getenv("EXCLUDE_QUERY") == skipValue {
				Skip("Skipping ThanosQuery controller tests")
			}
			name := manifestquery.Options{Options: manifests.Options{Owner: queryResourceName}}.GetGeneratedResourceName()
			resource := &monitoringthanosiov1alpha1.ThanosQuery{
				ObjectMeta: metav1.ObjectMeta{
					Name:      queryResourceName,
					Namespace: ns,
				},
				Spec: monitoringthanosiov1alpha1.ThanosQuerySpec{
					CommonFields:  monitoringthanosiov1alpha1.CommonFields{},
					ReplicaLabels: []string{"replica"},
				},
			}
			Expect(k8sClient.Create(context.Background(), resource)).Should(Succeed())

			By("creating the service monitor when enabled", func() {
				Eventually(func() bool {
					return utils.VerifyServiceMonitorExists(k8sClient, name, ns)
				}).Should(BeTrue())
			})

			By("creating the PDB when enabled", func() {
				Eventually(func() bool {
					return utils.VerifyPodDisruptionBudgetExists(k8sClient, name, ns)
				}).Should(BeTrue())
			})

			By("removing PDB when disabled", func() {
				resource.Spec.FeatureGates = &monitoringthanosiov1alpha1.FeatureGates{
					PodDisruptionBudgetConfig: &monitoringthanosiov1alpha1.PodDisruptionBudgetConfig{
						Enable: ptr.To(false),
					},
				}
				Expect(k8sClient.Update(context.Background(), resource)).Should(Succeed())
				Eventually(func() bool {
					return utils.VerifyPodDisruptionBudgetExists(k8sClient, name, ns)
				}).Should(BeFalse())
			})

		})
	})

	Context("When reconciling a ThanosReceive", func() {
		receiveResourceName := resourceName
		hashringName := "test-hashring"
		routerName := ReceiveRouterNameFromParent(receiveResourceName)
		ingesterName := ReceiveIngesterNameFromParent(receiveResourceName, hashringName)
		It("should reconcile correctly", func() {
			if os.Getenv("EXCLUDE_RECEIVE") == skipValue {
				Skip("Skipping ThanosReceive controller tests")
			}
			resource := &monitoringthanosiov1alpha1.ThanosReceive{
				ObjectMeta: metav1.ObjectMeta{
					Name:      receiveResourceName,
					Namespace: ns,
				},
				Spec: monitoringthanosiov1alpha1.ThanosReceiveSpec{
					Router: monitoringthanosiov1alpha1.RouterSpec{
						CommonFields:      monitoringthanosiov1alpha1.CommonFields{},
						Labels:            map[string]string{"test": "my-router-test"},
						ReplicationFactor: 3,
					},
					Ingester: monitoringthanosiov1alpha1.IngesterSpec{
						DefaultObjectStorageConfig: monitoringthanosiov1alpha1.ObjectStorageConfig{
							LocalObjectReference: corev1.LocalObjectReference{Name: "test-secret"},
							Key:                  "test-key",
						},
						Hashrings: []monitoringthanosiov1alpha1.IngesterHashringSpec{
							{
								Name:   hashringName,
								Labels: map[string]string{"test": "my-ingester-test"},
								StorageConfiguration: monitoringthanosiov1alpha1.StorageConfiguration{
									Size: "100Mi",
								},
								TenancyConfig: &monitoringthanosiov1alpha1.TenancyConfig{
									TenantMatcherType: "exact",
									Tenants:           []string{"test-tenant"},
								},
								Replicas: 3,
							},
						},
					},
				},
			}
			Expect(k8sClient.Create(context.Background(), resource)).Should(Succeed())
			workloads := []string{ingesterName, routerName}

			By("creating service monitor for ingester and router when enabled", func() {
				for _, workload := range workloads {
					Eventually(func() bool {
						return utils.VerifyServiceMonitorExists(k8sClient, workload, ns)
					}).Should(BeTrue())
				}
			})

			By("creating the PDB when enabled", func() {
				for _, workload := range workloads {
					Eventually(func() bool {
						return utils.VerifyPodDisruptionBudgetExists(k8sClient, workload, ns)
					}).Should(BeTrue())
				}
			})

			By("removing PDB when disabled", func() {
				resource.Spec.FeatureGates = &monitoringthanosiov1alpha1.FeatureGates{
					PodDisruptionBudgetConfig: &monitoringthanosiov1alpha1.PodDisruptionBudgetConfig{
						Enable: ptr.To(false),
					},
				}
				Expect(k8sClient.Update(context.Background(), resource)).Should(Succeed())
				for _, workload := range workloads {
					Eventually(func() bool {
						return utils.VerifyPodDisruptionBudgetExists(k8sClient, workload, ns)
					}).Should(BeFalse())
				}
			})
		})
	})

	Context("When reconciling a ThanosRuler", func() {
		rulerResourceName := resourceName
		It("should reconcile correctly", func() {
			if os.Getenv("EXCLUDE_RULER") == skipValue {
				Skip("Skipping ThanosRuler controller tests")
			}

			resource := &monitoringthanosiov1alpha1.ThanosRuler{
				ObjectMeta: metav1.ObjectMeta{
					Name:      rulerResourceName,
					Namespace: ns,
				},
				Spec: monitoringthanosiov1alpha1.ThanosRulerSpec{
					Replicas:     2,
					CommonFields: monitoringthanosiov1alpha1.CommonFields{},
					StorageConfiguration: monitoringthanosiov1alpha1.StorageConfiguration{
						Size: "1Gi",
					},
					ObjectStorageConfig: monitoringthanosiov1alpha1.ObjectStorageConfig{
						LocalObjectReference: corev1.LocalObjectReference{
							Name: "thanos-objstore",
						},
						Key: "thanos.yaml",
					},
					PrometheusRuleSelector: metav1.LabelSelector{
						MatchLabels: map[string]string{
							manifests.DefaultPrometheusRuleLabel: manifests.DefaultPrometheusRuleValue,
						},
					},
					AlertmanagerURL: "http://alertmanager.com:9093",
				},
			}
			Expect(k8sClient.Create(context.Background(), resource)).Should(Succeed())

			By("creating the service monitor when enabled", func() {
				Eventually(func() bool {
					return utils.VerifyServiceMonitorExists(k8sClient, RulerNameFromParent(rulerResourceName), ns)
				}).Should(BeTrue())
			})

			By("creating the PDB when enabled", func() {
				Eventually(func() bool {
					return utils.VerifyPodDisruptionBudgetExists(k8sClient, RulerNameFromParent(rulerResourceName), ns)
				}).Should(BeTrue())
			})

			By("removing PDB when disabled", func() {
				resource.Spec.FeatureGates = &monitoringthanosiov1alpha1.FeatureGates{
					PodDisruptionBudgetConfig: &monitoringthanosiov1alpha1.PodDisruptionBudgetConfig{
						Enable: ptr.To(false),
					},
				}
				Expect(k8sClient.Update(context.Background(), resource)).Should(Succeed())
				Eventually(func() bool {
					return utils.VerifyPodDisruptionBudgetExists(k8sClient, RulerNameFromParent(rulerResourceName), ns)
				}).Should(BeFalse())
			})
		})

		Context("When reconciling a ThanosStore", func() {
			storeResourceName := resourceName
			It("should reconcile correctly", func() {
				if os.Getenv("EXCLUDE_STORE") == skipValue {
					Skip("Skipping ThanosStore controller tests")
				}
				firstShard := StoreNameFromParent(storeResourceName, ptr.To(int32(0)))
				resource := &monitoringthanosiov1alpha1.ThanosStore{
					ObjectMeta: metav1.ObjectMeta{
						Name:      storeResourceName,
						Namespace: ns,
					},
					Spec: monitoringthanosiov1alpha1.ThanosStoreSpec{
						Replicas:     2,
						CommonFields: monitoringthanosiov1alpha1.CommonFields{},
						Labels:       map[string]string{"some-label": "xyz"},
						ShardingStrategy: monitoringthanosiov1alpha1.ShardingStrategy{
							Type:   monitoringthanosiov1alpha1.Block,
							Shards: 3,
						},
						StorageConfiguration: monitoringthanosiov1alpha1.StorageConfiguration{
							Size: "1Gi",
						},
						ObjectStorageConfig: monitoringthanosiov1alpha1.ObjectStorageConfig{
							LocalObjectReference: corev1.LocalObjectReference{
								Name: "thanos-objstore",
							},
							Key: "thanos.yaml",
						},
					},
				}
				Expect(k8sClient.Create(context.Background(), resource)).Should(Succeed())

				By("creating the service monitor when enabled", func() {
					Eventually(func() bool {
						return utils.VerifyServiceMonitorExists(k8sClient, firstShard, ns)
					}).Should(BeTrue())
				})

				By("creating the PDB when enabled", func() {
					Eventually(func() bool {
						return utils.VerifyPodDisruptionBudgetExists(k8sClient, firstShard, ns)
					}).Should(BeTrue())
				})

				By("removing PDB when disabled", func() {
					name := StoreNameFromParent(storeResourceName, nil)
					resource.Spec.FeatureGates = &monitoringthanosiov1alpha1.FeatureGates{
						PodDisruptionBudgetConfig: &monitoringthanosiov1alpha1.PodDisruptionBudgetConfig{
							Enable: ptr.To(false),
						},
					}
					Expect(k8sClient.Update(ctx, resource)).Should(Succeed())
					Eventually(func() bool {
						return utils.VerifyPodDisruptionBudgetExists(k8sClient, name, ns)
					}).Should(BeFalse())
				})
			})
		})
	})
})
