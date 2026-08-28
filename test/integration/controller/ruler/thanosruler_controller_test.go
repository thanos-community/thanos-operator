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

package ruler

import (
	"context"
	"fmt"
	"os"
	"time"

	. "github.com/onsi/ginkgo/v2"
	. "github.com/onsi/gomega"
	appsv1 "k8s.io/api/apps/v1"

	monitoringthanosiov1alpha1 "github.com/thanos-community/thanos-operator/api/v1alpha1"

	"github.com/thanos-community/thanos-operator/internal/controller"
	"github.com/thanos-community/thanos-operator/internal/pkg/manifests"
	"github.com/thanos-community/thanos-operator/test/utils"

	corev1 "k8s.io/api/core/v1"
	resourceapi "k8s.io/apimachinery/pkg/api/resource"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/util/intstr"
	"k8s.io/utils/ptr"
	"sigs.k8s.io/controller-runtime/pkg/client"
)

var _ = Describe("ThanosRuler Controller", func() {
	Context("When reconciling a resource", func() {
		const resourceName = "test-resource"

		ctx := context.Background()

		// each spec gets its own namespace so specs stay isolated and need no
		// teardown (envtest has no namespace controller to reap them anyway)
		var ns string

		BeforeEach(func() {
			By("creating a unique namespace and objstore secret")
			namespace := &corev1.Namespace{
				ObjectMeta: metav1.ObjectMeta{GenerateName: "test-ruler-"},
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
			if os.Getenv("EXCLUDE_RULER") == skipValue {
				Skip("Skipping ThanosRuler controller tests")
			}
			resource := &monitoringthanosiov1alpha1.ThanosRuler{
				ObjectMeta: metav1.ObjectMeta{
					Name:      resourceName,
					Namespace: ns,
					Annotations: map[string]string{
						"ruler-meta": "annotation",
					},
				},
				Spec: monitoringthanosiov1alpha1.ThanosRulerSpec{
					Replicas: 2,
					CommonFields: monitoringthanosiov1alpha1.CommonFields{
						Annotations: map[string]string{"ruler-spec": "annotation"},
					},
					StorageConfiguration: monitoringthanosiov1alpha1.StorageConfiguration{
						Size: resourceapi.MustParse("1Gi"),
					},
					RulerMode: monitoringthanosiov1alpha1.RulerMode{
						Type: "Stateful",
						Stateful: &monitoringthanosiov1alpha1.StatefulSpec{
							ObjectStorageConfig: monitoringthanosiov1alpha1.ObjectStorageConfig{
								LocalObjectReference: corev1.LocalObjectReference{
									Name: "thanos-objstore",
								},
								Key: "thanos.yaml",
							},
						},
					},
					RuleConfigSelector: metav1.LabelSelector{
						MatchLabels: map[string]string{
							manifests.DefaultPrometheusRuleLabel: manifests.DefaultPrometheusRuleValue,
						},
					},
					AlertmanagerURL: "http://alertmanager.com:9093",
					RuleTenancyConfig: &monitoringthanosiov1alpha1.RuleTenancyConfig{
						EnforcedTenantIdentifier: ptr.To("tenant"),
						TenantSpecifierLabel:     ptr.To("operator.thanos.io/tenant"),
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

			By("setting up the thanos ruler resources", func() {
				svc := &corev1.Service{
					ObjectMeta: metav1.ObjectMeta{
						Name:      "my-query",
						Namespace: ns,
						Labels:    controller.RequiredQueryServiceLabels,
					},
					Spec: corev1.ServiceSpec{
						Ports: []corev1.ServicePort{
							{
								Name:       "grpc",
								Port:       10901,
								TargetPort: intstr.FromInt32(10901),
							},
						},
					},
				}
				Expect(k8sClient.Create(context.Background(), svc)).Should(Succeed())

				DeferCleanup(func() {
					_ = k8sClient.Delete(context.Background(), svc)
				})

				Expect(k8sClient.Create(context.Background(), resource)).Should(Succeed())
				verifier := utils.Verifier{}.WithServiceAccount().WithService().WithStatefulSet()
				EventuallyWithOffset(1, func() bool {
					return verifier.Verify(k8sClient, controller.RulerNameFromParent(resourceName), ns)
				}, time.Minute).Should(BeTrue())

				EventuallyWithOffset(1, func() bool {
					return utils.VerifyStatefulSetArgs(k8sClient, controller.RulerNameFromParent(resourceName), ns, 0, "--label=rule_replica=\"$(NAME)\"")
				}, time.Second*30).Should(BeTrue())

				EventuallyWithOffset(1, func() bool {
					return utils.VerifyStatefulSetReplicas(
						k8sClient, 2, controller.RulerNameFromParent(resourceName), ns)
				}, time.Second*30).Should(BeTrue())

				EventuallyWithOffset(1, func() bool {
					arg := fmt.Sprintf("--query=dnssrv+_http._tcp.%s.%s.svc", "my-query", ns)
					return utils.VerifyStatefulSetArgs(k8sClient, controller.RulerNameFromParent(resourceName), ns, 0, arg)
				}, time.Minute).Should(BeTrue())
			})

			By("verifying ruler annotations", func() {
				EventuallyWithOffset(1, func() error {
					var objs []client.Object
					objs = append(objs, &corev1.ServiceAccount{}, &appsv1.StatefulSet{}, &corev1.Service{})

					expectedAnnotations := map[string]string{
						"ruler-meta":                    "annotation",
						"ruler-spec":                    "annotation",
						manifests.StorageSizeAnnotation: "1Gi",
					}

					if !utils.VerifyAnnotations(k8sClient, objs, controller.RulerNameFromParent(resourceName), ns, expectedAnnotations) {
						return fmt.Errorf("expected annotation %q not found", expectedAnnotations)
					}

					return nil
				}, time.Minute).Should(Succeed())
			})

			By("updating with new rule file", func() {
				cfgmap := &corev1.ConfigMap{
					ObjectMeta: metav1.ObjectMeta{
						Name:      "my-rules",
						Namespace: ns,
						Labels:    controller.DefaultRuleLabels,
					},
					Data: map[string]string{
						"my-rules.yaml": `groups:
- name: example
  rules:
  - alert: HighRequestLatency
    expr: job:request_latency_seconds:mean5m{job="myjob"} > 0.5
    for: 10m
    labels:
      severity: page
`,
					},
				}
				Expect(k8sClient.Create(context.Background(), cfgmap)).Should(Succeed())

				DeferCleanup(func() {
					_ = k8sClient.Delete(context.Background(), cfgmap)
				})

				EventuallyWithOffset(1, func() bool {
					// When RuleTenancyConfig is enabled, user ConfigMaps are processed and bucketed
					arg := "--rule-file=/etc/thanos/rules/" + resource.GetName() + "-usercfgmap-0/my-rules-my-rules.yaml"
					return utils.VerifyStatefulSetArgs(k8sClient, controller.RulerNameFromParent(resourceName), ns, 0, arg)
				}, time.Minute).Should(BeTrue())
			})

		})

		It("should enforce tenancy for user-provided ConfigMaps", func() {
			if os.Getenv("EXCLUDE_RULER") == skipValue {
				Skip("Skipping ThanosRuler controller tests")
			}
			resource := &monitoringthanosiov1alpha1.ThanosRuler{
				ObjectMeta: metav1.ObjectMeta{
					Name:      resourceName,
					Namespace: ns,
				},
				Spec: monitoringthanosiov1alpha1.ThanosRulerSpec{
					Replicas:     1,
					CommonFields: monitoringthanosiov1alpha1.CommonFields{},
					StorageConfiguration: monitoringthanosiov1alpha1.StorageConfiguration{
						Size: resourceapi.MustParse("1Gi"),
					},
					RulerMode: monitoringthanosiov1alpha1.RulerMode{
						Type: "Stateful",
						Stateful: &monitoringthanosiov1alpha1.StatefulSpec{
							ObjectStorageConfig: monitoringthanosiov1alpha1.ObjectStorageConfig{
								LocalObjectReference: corev1.LocalObjectReference{
									Name: "thanos-objstore",
								},
								Key: "thanos.yaml",
							},
						},
					},
					RuleConfigSelector: metav1.LabelSelector{
						MatchLabels: map[string]string{
							manifests.DefaultPrometheusRuleLabel: manifests.DefaultPrometheusRuleValue,
						},
					},
					AlertmanagerURL: "http://alertmanager.com:9093",
					RuleTenancyConfig: &monitoringthanosiov1alpha1.RuleTenancyConfig{
						EnforcedTenantIdentifier: ptr.To("tenant_id"),
						TenantSpecifierLabel:     ptr.To("app.tenant"),
					},
				},
			}

			svcName := "tenancy-cm-query"
			By("setting up ThanosRuler and query service", func() {
				svc := &corev1.Service{
					ObjectMeta: metav1.ObjectMeta{
						Name:      svcName,
						Namespace: ns,
						Labels:    controller.RequiredQueryServiceLabels,
					},
					Spec: corev1.ServiceSpec{
						Ports: []corev1.ServicePort{
							{
								Name:       "grpc",
								Port:       10901,
								TargetPort: intstr.FromInt32(10901),
							},
						},
					},
				}
				Expect(k8sClient.Create(context.Background(), svc)).Should(Succeed())
				Expect(k8sClient.Create(context.Background(), resource)).Should(Succeed())

				DeferCleanup(func() {
					Expect(k8sClient.Delete(context.Background(), svc)).Should(Succeed())
				})
			})

			By("creating user ConfigMap with tenant label", func() {
				cfgmap := &corev1.ConfigMap{
					ObjectMeta: metav1.ObjectMeta{
						Name:      "user-rules-tenant-x",
						Namespace: ns,
						Labels: map[string]string{
							manifests.DefaultPrometheusRuleLabel: manifests.DefaultPrometheusRuleValue,
							"app.tenant":                         "tenant-x",
						},
					},
					Data: map[string]string{
						"rules.yaml": `groups:
- name: user-alerts
  rules:
  - alert: ServiceDown
    expr: up == 0
`,
					},
				}
				Expect(k8sClient.Create(context.Background(), cfgmap)).Should(Succeed())

				DeferCleanup(func() {
					Expect(k8sClient.Delete(context.Background(), cfgmap)).Should(Succeed())
				})

				// Verify generated ConfigMap has tenant labels enforced
				EventuallyWithOffset(1, func() bool {
					cfgmapName := fmt.Sprintf("%s-usercfgmap-0", resourceName)
					return utils.VerifyConfigMapContents(k8sClient, cfgmapName, ns, "user-rules-tenant-x-rules.yaml",
						`groups:
- labels:
    tenant_id: tenant-x
  name: user-alerts
  rules:
  - alert: ServiceDown
    expr: up{tenant_id="tenant-x"} == 0
`)
				}, time.Minute).Should(BeTrue())
			})

			By("creating another user ConfigMap with different tenant", func() {
				cfgmap2 := &corev1.ConfigMap{
					ObjectMeta: metav1.ObjectMeta{
						Name:      "user-rules-tenant-y",
						Namespace: ns,
						Labels: map[string]string{
							manifests.DefaultPrometheusRuleLabel: manifests.DefaultPrometheusRuleValue,
							"app.tenant":                         "tenant-y",
						},
					},
					Data: map[string]string{
						"rules.yaml": `groups:
- name: database-alerts
  rules:
  - alert: DatabaseConnectionHigh
    expr: db_connections > 100
`,
					},
				}
				Expect(k8sClient.Create(context.Background(), cfgmap2)).Should(Succeed())

				DeferCleanup(func() {
					Expect(k8sClient.Delete(context.Background(), cfgmap2)).Should(Succeed())
				})

				// Verify the second tenant's ConfigMap is also processed correctly
				EventuallyWithOffset(1, func() bool {
					cfgmapName := fmt.Sprintf("%s-usercfgmap-0", resourceName)
					return utils.VerifyConfigMapContents(k8sClient, cfgmapName, ns, "user-rules-tenant-y-rules.yaml",
						`groups:
- labels:
    tenant_id: tenant-y
  name: database-alerts
  rules:
  - alert: DatabaseConnectionHigh
    expr: db_connections{tenant_id="tenant-y"} > 100
`)
				}, time.Minute).Should(BeTrue())
			})
		})

		It("should cleanup generated ConfigMaps when user ConfigMap is deleted", func() {
			if os.Getenv("EXCLUDE_RULER") == skipValue {
				Skip("Skipping ThanosRuler controller tests")
			}
			resource := &monitoringthanosiov1alpha1.ThanosRuler{
				ObjectMeta: metav1.ObjectMeta{
					Name:      resourceName,
					Namespace: ns,
				},
				Spec: monitoringthanosiov1alpha1.ThanosRulerSpec{
					Replicas:     1,
					CommonFields: monitoringthanosiov1alpha1.CommonFields{},
					StorageConfiguration: monitoringthanosiov1alpha1.StorageConfiguration{
						Size: resourceapi.MustParse("1Gi"),
					},
					RulerMode: monitoringthanosiov1alpha1.RulerMode{
						Type: "Stateful",
						Stateful: &monitoringthanosiov1alpha1.StatefulSpec{
							ObjectStorageConfig: monitoringthanosiov1alpha1.ObjectStorageConfig{
								LocalObjectReference: corev1.LocalObjectReference{
									Name: "thanos-objstore",
								},
								Key: "thanos.yaml",
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

			var userConfigMapName string
			svcName := "cleanup-query"

			By("setting up ThanosRuler and query service", func() {
				svc := &corev1.Service{
					ObjectMeta: metav1.ObjectMeta{
						Name:      svcName,
						Namespace: ns,
						Labels:    controller.RequiredQueryServiceLabels,
					},
					Spec: corev1.ServiceSpec{
						Ports: []corev1.ServicePort{
							{
								Name:       "grpc",
								Port:       10901,
								TargetPort: intstr.FromInt32(10901),
							},
						},
					},
				}
				Expect(k8sClient.Create(context.Background(), svc)).Should(Succeed())
				Expect(k8sClient.Create(context.Background(), resource)).Should(Succeed())

				DeferCleanup(func() {
					Expect(k8sClient.Delete(context.Background(), svc)).Should(Succeed())
				})
			})

			By("creating user ConfigMap with rules", func() {
				userConfigMapName = "cleanup-test-rules"
				cfgmap := &corev1.ConfigMap{
					ObjectMeta: metav1.ObjectMeta{
						Name:      userConfigMapName,
						Namespace: ns,
						Labels:    controller.DefaultRuleLabels,
					},
					Data: map[string]string{
						"test-rules.yaml": `groups:
- name: cleanup-test
  rules:
  - alert: TestAlert
    expr: up == 0
`,
					},
				}
				Expect(k8sClient.Create(context.Background(), cfgmap)).Should(Succeed())

				// Verify generated ConfigMap exists and contains the rule
				EventuallyWithOffset(1, func() bool {
					cfgmapName := fmt.Sprintf("%s-usercfgmap-0", resourceName)
					return utils.VerifyConfigMapExists(k8sClient, cfgmapName, ns)
				}, time.Second*30).Should(BeTrue())

				// Verify the rule file is referenced in StatefulSet args
				EventuallyWithOffset(1, func() bool {
					arg := "--rule-file=/etc/thanos/rules/" + resource.GetName() + "-usercfgmap-0/cleanup-test-rules-test-rules.yaml"
					return utils.VerifyStatefulSetArgs(k8sClient, controller.RulerNameFromParent(resourceName), ns, 0, arg)
				}, time.Minute).Should(BeTrue())
			})

			By("deleting user ConfigMap", func() {
				cfgmap := &corev1.ConfigMap{
					ObjectMeta: metav1.ObjectMeta{
						Name:      userConfigMapName,
						Namespace: ns,
					},
				}
				Expect(k8sClient.Delete(context.Background(), cfgmap)).Should(Succeed())

				// Verify that the generated ConfigMap no longer contains the deleted rule
				EventuallyWithOffset(1, func() bool {
					arg := "--rule-file=/etc/thanos/rules/" + resource.GetName() + "-usercfgmap-0/cleanup-test-rules-test-rules.yaml"
					return !utils.VerifyStatefulSetArgs(k8sClient, controller.RulerNameFromParent(resourceName), ns, 0, arg)
				}, time.Minute).Should(BeTrue())
			})
		})

		It("should enable stateless mode", func() {
			if os.Getenv("EXCLUDE_RULER") == skipValue {
				Skip("Skipping ThanosRuler controller tests")
			}
			resource := &monitoringthanosiov1alpha1.ThanosRuler{
				ObjectMeta: metav1.ObjectMeta{
					Name:      resourceName,
					Namespace: ns,
				},
				Spec: monitoringthanosiov1alpha1.ThanosRulerSpec{
					Replicas:     1,
					CommonFields: monitoringthanosiov1alpha1.CommonFields{},
					RulerMode: monitoringthanosiov1alpha1.RulerMode{
						Type:      "Stateless",
						Stateless: &monitoringthanosiov1alpha1.StatelessSpec{},
					},
					StorageConfiguration: monitoringthanosiov1alpha1.StorageConfiguration{
						Size: resourceapi.MustParse("1Gi"),
					},
					RuleTenancyConfig: &monitoringthanosiov1alpha1.RuleTenancyConfig{
						TenantSpecifierLabel: ptr.To(controller.DefaultTenantSpecifier),
					},
					RuleConfigSelector: metav1.LabelSelector{
						MatchLabels: map[string]string{
							manifests.DefaultPrometheusRuleLabel: manifests.DefaultPrometheusRuleValue,
						},
					},
					AlertmanagerURL: "http://alertmanager.com:9093",
				},
			}

			receiveSvcName := "test-receive"
			querySvcName := "test-query"
			cfgmapName := "test-config"

			By("setting up required resources", func() {
				queryService := &corev1.Service{
					ObjectMeta: metav1.ObjectMeta{
						Name:      querySvcName,
						Namespace: ns,
						Labels:    controller.RequiredQueryServiceLabels,
					},
					Spec: corev1.ServiceSpec{
						Ports: []corev1.ServicePort{
							{
								Name:       "grpc",
								Port:       10901,
								TargetPort: intstr.FromInt32(10901),
							},
						},
					},
				}
				Expect(k8sClient.Create(context.Background(), queryService)).Should(Succeed())
				DeferCleanup(func() error {
					return k8sClient.Delete(context.Background(), queryService)
				})

				receiveSvc := &corev1.Service{
					ObjectMeta: metav1.ObjectMeta{
						Name:      receiveSvcName,
						Namespace: ns,
						Labels:    controller.DefaultRemoteWriteLabels,
					},
					Spec: corev1.ServiceSpec{
						Ports: []corev1.ServicePort{
							{
								Name:       "remote-write",
								Port:       19291,
								TargetPort: intstr.IntOrString{IntVal: 19291},
							},
						},
					},
				}
				Expect(k8sClient.Create(context.Background(), receiveSvc)).Should(Succeed())
				DeferCleanup(func() error {
					return k8sClient.Delete(context.Background(), receiveSvc)
				})

				statelessRuleLabels := map[string]string{
					manifests.DefaultPrometheusRuleLabel: manifests.DefaultPrometheusRuleValue,
					controller.DefaultTenantSpecifier:    "test-tenant",
				}

				cfgmap := &corev1.ConfigMap{
					ObjectMeta: metav1.ObjectMeta{
						Name:      cfgmapName,
						Namespace: ns,
						Labels:    statelessRuleLabels,
					},
					Data: map[string]string{
						"test-rules.yaml": `groups:
- name: test
  rules:
  - alert: TestAlert
    expr: up == 0
`,
					},
				}
				Expect(k8sClient.Create(context.Background(), cfgmap)).Should(Succeed())
				DeferCleanup(func() error {
					return k8sClient.Delete(context.Background(), cfgmap)
				})

				Expect(k8sClient.Create(context.Background(), resource)).Should(Succeed())
				verifier := utils.Verifier{}.WithServiceAccount().WithService().WithStatefulSet().WithSecret()
				EventuallyWithOffset(1, func() bool {
					return verifier.Verify(k8sClient, controller.RulerNameFromParent(resourceName), ns)
				}, time.Minute).Should(BeTrue())

			})

			By("verify remote write Secret", func() {
				arg := "--remote-write.config-file=/etc/thanos/remote-write/remote-write.yaml"
				EventuallyWithOffset(1, func() bool {
					return utils.VerifyStatefulSetArgs(k8sClient, controller.RulerNameFromParent(resourceName), ns, 0, arg)
				}, time.Second*30).Should(BeTrue())

				EventuallyWithOffset(0, func() bool {
					secret := &corev1.Secret{}
					if err := k8sClient.Get(context.Background(), client.ObjectKey{Namespace: ns, Name: controller.RulerNameFromParent(resourceName)}, secret); err != nil {
						return false
					}
					if _, exists := secret.Data["remote-write.yaml"]; !exists {
						return false
					}

					expectedContent := fmt.Sprintf(`remote_write:
- url: http://%s.%s.svc:19291/api/v1/receive
  headers:
    THANOS-TENANT: test-tenant
  write_relabel_configs:
  - source_labels:
    - tenant_id
    regex: test-tenant
    action: keep
`, receiveSvcName, ns)

					return string(secret.Data["remote-write.yaml"]) == expectedContent
				}, time.Minute).Should(BeTrue())
			})

			By("switching to stateful mode", func() {
				Eventually(func() bool {
					existingRuler := &monitoringthanosiov1alpha1.ThanosRuler{}
					if err := k8sClient.Get(ctx, client.ObjectKey{Namespace: ns, Name: resourceName}, existingRuler); err != nil {
						return false
					}
					existingRuler.Spec.RulerMode.Type = "Stateful"
					existingRuler.Spec.RulerMode.Stateless = nil
					existingRuler.Spec.RulerMode.Stateful = &monitoringthanosiov1alpha1.StatefulSpec{
						ObjectStorageConfig: monitoringthanosiov1alpha1.ObjectStorageConfig{
							LocalObjectReference: corev1.LocalObjectReference{
								Name: "thanos-objstore",
							},
							Key: "thanos.yaml",
						},
					}
					if err := k8sClient.Update(ctx, existingRuler); err != nil {
						return false
					}
					return true
				}, time.Minute).Should(BeTrue())

				EventuallyWithOffset(1, func() bool {
					arg := "--remote-write.config-file=/etc/thanos/remote-write/remote-write.yaml"
					return !utils.VerifyStatefulSetArgs(k8sClient, controller.RulerNameFromParent(resourceName), ns, 0, arg)
				}, time.Minute).Should(BeTrue())

				EventuallyWithOffset(1, func() bool {
					arg := "--objstore.config=$(OBJSTORE_CONFIG)"
					return utils.VerifyStatefulSetArgs(k8sClient, controller.RulerNameFromParent(resourceName), ns, 0, arg)
				}, time.Minute).Should(BeTrue())

				Eventually(func() bool {
					secret := &corev1.Secret{}
					err := k8sClient.Get(ctx, client.ObjectKey{Namespace: ns, Name: controller.RulerNameFromParent(resourceName)}, secret)
					return err != nil
				}, time.Minute).Should(BeTrue())
			})
		})
	})
})
