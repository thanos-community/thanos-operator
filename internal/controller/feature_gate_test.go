package controller

import (
	"context"
	manifestquery "github.com/thanos-community/thanos-operator/internal/pkg/manifests/query"
	"os"
	"time"

	. "github.com/onsi/ginkgo/v2"
	. "github.com/onsi/gomega"

	monitoringthanosiov1alpha1 "github.com/thanos-community/thanos-operator/api/v1alpha1"
	"github.com/thanos-community/thanos-operator/internal/pkg/manifests"
	"github.com/thanos-community/thanos-operator/internal/pkg/manifests/compact"
	"github.com/thanos-community/thanos-operator/test/utils"

	corev1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/utils/ptr"
)

var _ = Describe("ServiceMonitor FeatureGate", Ordered, func() {
	const (
		ns = "feature-gate-service-monitor-test"
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
		resourceName := "test-compact-resource"
		shardName := "test-shard"

		shardOne := compact.Options{
			Options:   manifests.Options{Owner: resourceName},
			ShardName: ptr.To(shardName), ShardIndex: ptr.To(0)}.GetGeneratedResourceName()
		shardTwo := compact.Options{
			Options:   manifests.Options{Owner: resourceName},
			ShardName: ptr.To(shardName), ShardIndex: ptr.To(1)}.GetGeneratedResourceName()

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
					CommonFields: monitoringthanosiov1alpha1.CommonFields{},
					Labels:       map[string]string{"some-label": "xyz"},
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
				},
			}

			By("creating the service monitor when enabled", func() {
				Expect(k8sClient.Create(context.Background(), resource)).Should(Succeed())
				Eventually(func() bool {
					for _, shard := range []string{shardOne, shardTwo} {
						if utils.VerifyServiceMonitorExists(k8sClient, shard, ns) {
							return true
						}
					}
					return false
				}, time.Minute*1, time.Second*10).Should(BeTrue())

			})

			By("removing service monitor when disabled", func() {
				resource.Spec.FeatureGates = &monitoringthanosiov1alpha1.FeatureGates{
					ServiceMonitorConfig: &monitoringthanosiov1alpha1.ServiceMonitorConfig{
						Enable: ptr.To(false),
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
		})
	})

	Context("When reconciling a ThanosQuery", func() {
		resourceName := "test-resource"
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
					ReplicaLabels: []string{"replica"},
				},
			}
			By("creating the service monitor when enabled", func() {
				Expect(k8sClient.Create(context.Background(), resource)).Should(Succeed())
				Eventually(func() bool {
					return utils.VerifyServiceMonitorExists(k8sClient, name, ns)
				}, time.Minute*1, time.Second*10).Should(BeTrue())
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
		})
	})

	Context("When reconciling a ThanosReceive", func() {
		resourceName := "test-resource"
		hashringName := "test-hashring"
		routerName := ReceiveRouterNameFromParent(resourceName)
		ingesterName := ReceiveIngesterNameFromParent(resourceName, hashringName)
		It("should reconcile correctly", func() {
			if os.Getenv("EXCLUDE_RECEIVE") == skipValue {
				Skip("Skipping ThanosReceive controller tests")
			}
			resource := &monitoringthanosiov1alpha1.ThanosReceive{
				ObjectMeta: metav1.ObjectMeta{
					Name:      resourceName,
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
								Name:        hashringName,
								Labels:      map[string]string{"test": "my-ingester-test"},
								StorageSize: "100Mi",
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

			By("creating service monitor for ingester and router when enabled", func() {
				Expect(k8sClient.Create(context.Background(), resource)).Should(Succeed())

				workloads := []string{ingesterName, routerName}
				for _, workload := range workloads {
					Eventually(func() bool {
						return utils.VerifyServiceMonitorExists(k8sClient, workload, ns)
					}).Should(BeTrue())
				}
			})

			By("removing service monitor from ingester and router when disabled", func() {
				workloads := []string{ingesterName, routerName}

				for _, workload := range workloads {
					Expect(utils.VerifyServiceMonitorExists(k8sClient, workload, ns)).To(BeTrue())
				}

				resource.Spec.FeatureGates = &monitoringthanosiov1alpha1.FeatureGates{
					ServiceMonitorConfig: &monitoringthanosiov1alpha1.ServiceMonitorConfig{
						Enable: ptr.To(false),
					},
				}
				Expect(k8sClient.Update(context.Background(), resource)).Should(Succeed())

				Eventually(func() bool {
					for _, workload := range workloads {
						if utils.VerifyServiceMonitorExists(k8sClient, workload, ns) {
							return true
						}
					}
					return false
				}, time.Minute*1, time.Second*10).Should(BeFalse())
			})
		})
	})

	Context("When reconciling a ThanosRuler", func() {
		resourceName := "test-resource"
		It("should reconcile correctly", func() {
			if os.Getenv("EXCLUDE_RULER") == skipValue {
				Skip("Skipping ThanosRuler controller tests")
			}

			resource := &monitoringthanosiov1alpha1.ThanosRuler{
				ObjectMeta: metav1.ObjectMeta{
					Name:      resourceName,
					Namespace: ns,
				},
				Spec: monitoringthanosiov1alpha1.ThanosRulerSpec{
					Replicas:     2,
					CommonFields: monitoringthanosiov1alpha1.CommonFields{},
					StorageSize:  "1Gi",
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

			By("creating the service monitor when enabled", func() {
				Expect(k8sClient.Create(context.Background(), resource)).Should(Succeed())
				Eventually(func() bool {
					return utils.VerifyServiceMonitorExists(k8sClient, RulerNameFromParent(resourceName), ns)
				}).Should(BeTrue())
			})

			By("removing service monitor when disabled", func() {
				resource.Spec.FeatureGates.ServiceMonitorConfig.Enable = ptr.To(false)
				Expect(k8sClient.Update(context.Background(), resource)).Should(Succeed())
				Eventually(func() bool {
					return utils.VerifyServiceMonitorExists(k8sClient, RulerNameFromParent(resourceName), ns)
				}, time.Minute*1, time.Second*10).Should(BeFalse())
			})
		})

		Context("When reconciling a ThanosStore", func() {
			resourceName := "test-resource"
			It("should reconcile correctly", func() {
				if os.Getenv("EXCLUDE_STORE") == skipValue {
					Skip("Skipping ThanosStore controller tests")
				}
				firstShard := StoreNameFromParent(resourceName, ptr.To(int32(0)))
				resource := &monitoringthanosiov1alpha1.ThanosStore{
					ObjectMeta: metav1.ObjectMeta{
						Name:      resourceName,
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
						StorageSize: "1Gi",
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
						return utils.VerifyServiceMonitorExists(k8sClient, firstShard, ns)
					}).Should(BeTrue())
				})

				By("removing service monitor when disabled", func() {
					name := StoreNameFromParent(resourceName, nil)
					resource.Spec.FeatureGates = &monitoringthanosiov1alpha1.FeatureGates{
						ServiceMonitorConfig: &monitoringthanosiov1alpha1.ServiceMonitorConfig{
							Enable: ptr.To(false),
						},
					}
					Expect(k8sClient.Update(ctx, resource)).Should(Succeed())
					Eventually(func() bool {
						return utils.VerifyServiceMonitorExists(k8sClient, name, ns)
					}, time.Second*30, time.Second*10).Should(BeFalse())
				})
			})
		})
	})
})
