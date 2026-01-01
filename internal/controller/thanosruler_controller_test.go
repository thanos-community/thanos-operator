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
	"fmt"
	"os"
	"time"

	. "github.com/onsi/ginkgo/v2"
	. "github.com/onsi/gomega"

	monitoringv1 "github.com/prometheus-operator/prometheus-operator/pkg/apis/monitoring/v1"

	monitoringthanosiov1alpha1 "github.com/thanos-community/thanos-operator/api/v1alpha1"
	"github.com/thanos-community/thanos-operator/internal/pkg/manifests"
	"github.com/thanos-community/thanos-operator/test/utils"

	corev1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/types"
	"k8s.io/apimachinery/pkg/util/intstr"
)

var _ = Describe("ThanosRuler Controller", Ordered, func() {
	Context("When reconciling a resource", func() {
		const (
			resourceName = "test-resource"
			ns           = "test-ruler"
		)

		ctx := context.Background()

		typeNamespacedName := types.NamespacedName{
			Name:      resourceName,
			Namespace: ns,
		}

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

		AfterEach(func() {
			resource := &monitoringthanosiov1alpha1.ThanosRuler{}
			err := k8sClient.Get(ctx, typeNamespacedName, resource)
			Expect(err).NotTo(HaveOccurred())

			By("Cleanup the specific resource instance ThanosRuler")
			Expect(k8sClient.Delete(ctx, resource)).To(Succeed())
		})

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
					StorageConfiguration: monitoringthanosiov1alpha1.StorageConfiguration{
						Size: "1Gi",
					},
					ObjectStorageConfig: monitoringthanosiov1alpha1.ObjectStorageConfig{
						LocalObjectReference: corev1.LocalObjectReference{
							Name: "thanos-objstore",
						},
						Key: "thanos.yaml",
					},
					RuleConfigSelector: metav1.LabelSelector{
						MatchLabels: map[string]string{
							manifests.DefaultPrometheusRuleLabel: manifests.DefaultPrometheusRuleValue,
						},
					},
					AlertmanagerURL: "http://alertmanager.com:9093",
					RuleTenancyConfig: &monitoringthanosiov1alpha1.RuleTenancyConfig{
						TenantLabel:      "tenant",
						TenantValueLabel: "operator.thanos.io/tenant",
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
						Labels:    requiredQueryServiceLabels,
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
					return verifier.Verify(k8sClient, RulerNameFromParent(resourceName), ns)
				}, time.Minute, time.Second*2).Should(BeTrue())

				EventuallyWithOffset(1, func() bool {
					return utils.VerifyStatefulSetArgs(k8sClient, RulerNameFromParent(resourceName), ns, 0, "--label=rule_replica=\"$(NAME)\"")
				}, time.Second*30, time.Second*2).Should(BeTrue())

				EventuallyWithOffset(1, func() bool {
					return utils.VerifyStatefulSetReplicas(
						k8sClient, 2, RulerNameFromParent(resourceName), ns)
				}, time.Second*30, time.Second*2).Should(BeTrue())

				EventuallyWithOffset(1, func() bool {
					arg := fmt.Sprintf("--query=dnssrv+_http._tcp.%s.%s.svc", "my-query", ns)
					return utils.VerifyStatefulSetArgs(k8sClient, RulerNameFromParent(resourceName), ns, 0, arg)
				}, time.Minute, time.Second*2).Should(BeTrue())
			})

			By("updating with new rule file", func() {
				cfgmap := &corev1.ConfigMap{
					ObjectMeta: metav1.ObjectMeta{
						Name:      "my-rules",
						Namespace: ns,
						Labels:    defaultRuleLabels,
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

				promRule := &monitoringv1.PrometheusRule{
					ObjectMeta: metav1.ObjectMeta{
						Name:      "test-promrule",
						Namespace: ns,
						Labels: map[string]string{
							manifests.DefaultPrometheusRuleLabel: manifests.DefaultPrometheusRuleValue,
							"operator.thanos.io/tenant":          "test",
						},
					},
					Spec: monitoringv1.PrometheusRuleSpec{
						Groups: []monitoringv1.RuleGroup{
							{
								Name: "example",
								Rules: []monitoringv1.Rule{
									{
										Alert: "HighRequestLatency",
										Expr:  intstr.FromString(`job:request_latency_seconds:mean5m{job="myjob"} > 0.5`),
										Labels: map[string]string{
											"severity": "page",
										},
									},
								},
							},
						},
					},
				}
				Expect(k8sClient.Create(context.Background(), promRule)).Should(Succeed())

				DeferCleanup(func() {
					_ = k8sClient.Delete(context.Background(), promRule)
				})

				promRule2 := &monitoringv1.PrometheusRule{
					ObjectMeta: metav1.ObjectMeta{
						Name:      "test-promrule-2",
						Namespace: ns,
						Labels: map[string]string{
							manifests.DefaultPrometheusRuleLabel: manifests.DefaultPrometheusRuleValue,
							"operator.thanos.io/tenant":          "test",
						},
					},
					Spec: monitoringv1.PrometheusRuleSpec{
						Groups: []monitoringv1.RuleGroup{
							{
								Name: "example",
								Rules: []monitoringv1.Rule{
									{
										Alert: "HighRequestLatency",
										Expr:  intstr.FromString(`job:request_latency_seconds:mean5m{job="myjob"} > 0.5`),
										Labels: map[string]string{
											"severity": "page",
										},
									},
								},
							},
						},
					},
				}
				Expect(k8sClient.Create(context.Background(), promRule2)).Should(Succeed())

				DeferCleanup(func() {
					_ = k8sClient.Delete(context.Background(), promRule2)
				})

				EventuallyWithOffset(1, func() bool {
					arg := "--rule-file=/etc/thanos/rules/" + resource.GetName() + "-promrule-0/" + promRule.Name + ".yaml"
					return utils.VerifyStatefulSetArgs(k8sClient, RulerNameFromParent(resourceName), ns, 0, arg)
				}, time.Minute, time.Second*2).Should(BeTrue())

				EventuallyWithOffset(1, func() bool {
					arg := "--rule-file=/etc/thanos/rules/" + resource.GetName() + "-promrule-0/" + promRule2.Name + ".yaml"
					return utils.VerifyStatefulSetArgs(k8sClient, RulerNameFromParent(resourceName), ns, 0, arg)
				}, time.Minute, time.Second*2).Should(BeTrue())

				EventuallyWithOffset(1, func() bool {
					// When RuleTenancyConfig is enabled, user ConfigMaps are processed and bucketed
					arg := "--rule-file=/etc/thanos/rules/" + resource.GetName() + "-usercfgmap-0/my-rules-my-rules.yaml"
					return utils.VerifyStatefulSetArgs(k8sClient, RulerNameFromParent(resourceName), ns, 0, arg)
				}, time.Minute, time.Second*2).Should(BeTrue())

				EventuallyWithOffset(1, func() bool {
					cfgmapName := fmt.Sprintf("%s-promrule-0", resourceName)
					return utils.VerifyConfigMapContents(k8sClient, cfgmapName, ns, "test-promrule.yaml",
						`groups:
- labels:
    tenant: test
  name: example
  rules:
  - alert: HighRequestLatency
    expr: job:request_latency_seconds:mean5m{job="myjob",tenant="test"} > 0.5
    labels:
      severity: page
`)
				}, time.Second*10, time.Second*2).Should(BeTrue())

			})

			By("updating RuleConfigSelector with additional custom label foo:bar", func() {
				customLabelKey := "foo"
				customLabelValue := "bar"

				// Update the ThanosRuler to add custom label to selector
				Expect(k8sClient.Get(context.Background(), typeNamespacedName, resource)).Should(Succeed())
				resource.Spec.RuleConfigSelector.MatchLabels[customLabelKey] = customLabelValue
				Expect(k8sClient.Update(context.Background(), resource)).Should(Succeed())

				// Create a PrometheusRule with matching custom label
				promRuleWithCustomLabel := &monitoringv1.PrometheusRule{
					ObjectMeta: metav1.ObjectMeta{
						Name:      "custom-label-promrule",
						Namespace: ns,
						Labels: map[string]string{
							manifests.DefaultPrometheusRuleLabel: manifests.DefaultPrometheusRuleValue,
							customLabelKey:                       customLabelValue,
							"operator.thanos.io/tenant":          "test",
						},
					},
					Spec: monitoringv1.PrometheusRuleSpec{
						Groups: []monitoringv1.RuleGroup{
							{
								Name: "custom-group",
								Rules: []monitoringv1.Rule{
									{
										Alert: "CustomAlert",
										Expr:  intstr.FromString(`up == 0`),
										Labels: map[string]string{
											"severity": "critical",
										},
									},
								},
							},
						},
					},
				}
				Expect(k8sClient.Create(context.Background(), promRuleWithCustomLabel)).Should(Succeed())

				DeferCleanup(func() {
					_ = k8sClient.Delete(context.Background(), promRuleWithCustomLabel)
				})

				// Verify the rule file with custom label is configured
				EventuallyWithOffset(1, func() bool {
					arg := "--rule-file=/etc/thanos/rules/" + resource.GetName() + "-promrule-0/" + promRuleWithCustomLabel.Name + ".yaml"
					return utils.VerifyStatefulSetArgs(k8sClient, RulerNameFromParent(resourceName), ns, 0, arg)
				}, time.Minute, time.Second*2).Should(BeTrue())

				// Verify the ConfigMap has the custom label from selector
				EventuallyWithOffset(1, func() bool {
					cfgmapName := fmt.Sprintf("%s-promrule-0", resourceName)
					cm := &corev1.ConfigMap{}
					if err := k8sClient.Get(context.Background(), types.NamespacedName{Name: cfgmapName, Namespace: ns}, cm); err != nil {
						return false
					}
					// Check that custom label is present on the ConfigMap
					return cm.Labels[customLabelKey] == customLabelValue &&
						cm.Labels[manifests.PromRuleDerivedConfigMapLabel] == manifests.PromRuleDerivedConfigMapValue
				}, time.Second*30, time.Second*2).Should(BeTrue())

				// Verify the previous PrometheusRules without custom label are no longer picked up
				// (test-promrule and test-promrule-2 don't have foo:bar label)
				EventuallyWithOffset(1, func() bool {
					arg := "--rule-file=/etc/thanos/rules/" + resource.GetName() + "-promrule-0/test-promrule.yaml"
					return !utils.VerifyStatefulSetArgs(k8sClient, RulerNameFromParent(resourceName), ns, 0, arg)
				}, time.Minute, time.Second*2).Should(BeTrue())
			})

		})

		It("should enforce tenancy for PrometheusRule", func() {
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
						Size: "1Gi",
					},
					ObjectStorageConfig: monitoringthanosiov1alpha1.ObjectStorageConfig{
						LocalObjectReference: corev1.LocalObjectReference{
							Name: "thanos-objstore",
						},
						Key: "thanos.yaml",
					},
					RuleConfigSelector: metav1.LabelSelector{
						MatchLabels: map[string]string{
							manifests.DefaultPrometheusRuleLabel: manifests.DefaultPrometheusRuleValue,
						},
					},
					AlertmanagerURL: "http://alertmanager.com:9093",
					RuleTenancyConfig: &monitoringthanosiov1alpha1.RuleTenancyConfig{
						TenantLabel:      "tenant_id",
						TenantValueLabel: "tenant",
					},
				},
			}

			svcName := "tenancy-pr-query"
			By("creating ThanosRuler with tenancy config", func() {
				svc := &corev1.Service{
					ObjectMeta: metav1.ObjectMeta{
						Name:      svcName,
						Namespace: ns,
						Labels:    requiredQueryServiceLabels,
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

			By("creating PrometheusRule with tenant label", func() {
				promRule := &monitoringv1.PrometheusRule{
					ObjectMeta: metav1.ObjectMeta{
						Name:      "tenant-a-rule",
						Namespace: ns,
						Labels: map[string]string{
							manifests.DefaultPrometheusRuleLabel: manifests.DefaultPrometheusRuleValue,
							"tenant":                             "team-a",
						},
					},
					Spec: monitoringv1.PrometheusRuleSpec{
						Groups: []monitoringv1.RuleGroup{
							{
								Name: "tenant-a-alerts",
								Rules: []monitoringv1.Rule{
									{
										Alert: "HighCPU",
										Expr:  intstr.FromString(`cpu_usage{job="app"} > 80`),
										Labels: map[string]string{
											"severity": "warning",
										},
									},
									{
										Record: "app:requests:rate5m",
										Expr:   intstr.FromString(`rate(http_requests_total{job="app"}[5m])`),
									},
								},
							},
						},
					},
				}
				Expect(k8sClient.Create(context.Background(), promRule)).Should(Succeed())

				DeferCleanup(func() {
					Expect(k8sClient.Delete(context.Background(), promRule)).Should(Succeed())
				})

				// Verify the ConfigMap is created with tenant labels applied
				EventuallyWithOffset(1, func() bool {
					cfgmapName := fmt.Sprintf("%s-promrule-0", resourceName)
					return utils.VerifyConfigMapContents(k8sClient, cfgmapName, ns, "tenant-a-rule.yaml",
						`groups:
- labels:
    tenant_id: team-a
  name: tenant-a-alerts
  rules:
  - alert: HighCPU
    expr: cpu_usage{job="app",tenant_id="team-a"} > 80
    labels:
      severity: warning
  - expr: rate(http_requests_total{job="app",tenant_id="team-a"}[5m])
    record: app:requests:rate5m
`)
				}, time.Second*30, time.Second*2).Should(BeTrue())
			})

			By("creating PrometheusRule with different tenant", func() {
				promRule2 := &monitoringv1.PrometheusRule{
					ObjectMeta: metav1.ObjectMeta{
						Name:      "tenant-b-rule",
						Namespace: ns,
						Labels: map[string]string{
							manifests.DefaultPrometheusRuleLabel: manifests.DefaultPrometheusRuleValue,
							"tenant":                             "team-b",
						},
					},
					Spec: monitoringv1.PrometheusRuleSpec{
						Groups: []monitoringv1.RuleGroup{
							{
								Name: "tenant-b-alerts",
								Rules: []monitoringv1.Rule{
									{
										Alert: "LowMemory",
										Expr:  intstr.FromString(`memory_available{job="database"} < 20`),
										Labels: map[string]string{
											"severity": "critical",
										},
									},
								},
							},
						},
					},
				}
				Expect(k8sClient.Create(context.Background(), promRule2)).Should(Succeed())

				DeferCleanup(func() {
					Expect(k8sClient.Delete(context.Background(), promRule2)).Should(Succeed())
				})

				// Verify both tenant ConfigMaps exist with correct tenancy
				EventuallyWithOffset(1, func() bool {
					cfgmapName := fmt.Sprintf("%s-promrule-0", resourceName)
					return utils.VerifyConfigMapContents(k8sClient, cfgmapName, ns, "tenant-b-rule.yaml",
						`groups:
- labels:
    tenant_id: team-b
  name: tenant-b-alerts
  rules:
  - alert: LowMemory
    expr: memory_available{job="database",tenant_id="team-b"} < 20
    labels:
      severity: critical
`)
				}, time.Second*30, time.Second*2).Should(BeTrue())
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
						Size: "1Gi",
					},
					ObjectStorageConfig: monitoringthanosiov1alpha1.ObjectStorageConfig{
						LocalObjectReference: corev1.LocalObjectReference{
							Name: "thanos-objstore",
						},
						Key: "thanos.yaml",
					},
					RuleConfigSelector: metav1.LabelSelector{
						MatchLabels: map[string]string{
							manifests.DefaultPrometheusRuleLabel: manifests.DefaultPrometheusRuleValue,
						},
					},
					AlertmanagerURL: "http://alertmanager.com:9093",
					RuleTenancyConfig: &monitoringthanosiov1alpha1.RuleTenancyConfig{
						TenantLabel:      "tenant_id",
						TenantValueLabel: "app.tenant",
					},
				},
			}

			svcName := "tenancy-cm-query"
			By("setting up ThanosRuler and query service", func() {
				svc := &corev1.Service{
					ObjectMeta: metav1.ObjectMeta{
						Name:      svcName,
						Namespace: ns,
						Labels:    requiredQueryServiceLabels,
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
				}, time.Minute, time.Second*2).Should(BeTrue())
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
				}, time.Minute, time.Second*2).Should(BeTrue())
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
						Size: "1Gi",
					},
					ObjectStorageConfig: monitoringthanosiov1alpha1.ObjectStorageConfig{
						LocalObjectReference: corev1.LocalObjectReference{
							Name: "thanos-objstore",
						},
						Key: "thanos.yaml",
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
			var promRuleName string
			svcName := "cleanup-query"

			By("setting up ThanosRuler and query service", func() {
				svc := &corev1.Service{
					ObjectMeta: metav1.ObjectMeta{
						Name:      svcName,
						Namespace: ns,
						Labels:    requiredQueryServiceLabels,
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
						Labels:    defaultRuleLabels,
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
				}, time.Second*30, time.Second*2).Should(BeTrue())

				// Verify the rule file is referenced in StatefulSet args
				EventuallyWithOffset(1, func() bool {
					arg := "--rule-file=/etc/thanos/rules/" + resource.GetName() + "-usercfgmap-0/cleanup-test-rules-test-rules.yaml"
					return utils.VerifyStatefulSetArgs(k8sClient, RulerNameFromParent(resourceName), ns, 0, arg)
				}, time.Minute, time.Second*2).Should(BeTrue())
			})

			By("creating PrometheusRule", func() {
				promRuleName = "cleanup-test-promrule"
				promRule := &monitoringv1.PrometheusRule{
					ObjectMeta: metav1.ObjectMeta{
						Name:      promRuleName,
						Namespace: ns,
						Labels:    defaultRuleLabels,
					},
					Spec: monitoringv1.PrometheusRuleSpec{
						Groups: []monitoringv1.RuleGroup{
							{
								Name: "promrule-test",
								Rules: []monitoringv1.Rule{
									{
										Alert: "PrometheusRuleAlert",
										Expr:  intstr.FromString(`up == 0`),
									},
								},
							},
						},
					},
				}
				Expect(k8sClient.Create(context.Background(), promRule)).Should(Succeed())

				// Verify PrometheusRule-derived ConfigMap exists
				EventuallyWithOffset(1, func() bool {
					cfgmapName := fmt.Sprintf("%s-promrule-0", resourceName)
					return utils.VerifyConfigMapExists(k8sClient, cfgmapName, ns)
				}, time.Second*30, time.Second*2).Should(BeTrue())
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
					return !utils.VerifyStatefulSetArgs(k8sClient, RulerNameFromParent(resourceName), ns, 0, arg)
				}, time.Minute, time.Second*2).Should(BeTrue())

				// Verify PrometheusRule-derived ConfigMap still exists (not affected by user ConfigMap deletion)
				EventuallyWithOffset(1, func() bool {
					cfgmapName := fmt.Sprintf("%s-promrule-0", resourceName)
					return utils.VerifyConfigMapExists(k8sClient, cfgmapName, ns)
				}, time.Second*30, time.Second*2).Should(BeTrue())

				// Verify PrometheusRule is still referenced
				EventuallyWithOffset(1, func() bool {
					arg := "--rule-file=/etc/thanos/rules/" + resource.GetName() + "-promrule-0/" + promRuleName + ".yaml"
					return utils.VerifyStatefulSetArgs(k8sClient, RulerNameFromParent(resourceName), ns, 0, arg)
				}, time.Minute, time.Second*2).Should(BeTrue())
			})

			By("deleting PrometheusRule", func() {
				promRule := &monitoringv1.PrometheusRule{
					ObjectMeta: metav1.ObjectMeta{
						Name:      promRuleName,
						Namespace: ns,
					},
				}
				Expect(k8sClient.Delete(context.Background(), promRule)).Should(Succeed())

				// Verify PrometheusRule is no longer referenced
				EventuallyWithOffset(1, func() bool {
					arg := "--rule-file=/etc/thanos/rules/" + resource.GetName() + "-promrule-0/" + promRuleName + ".yaml"
					return !utils.VerifyStatefulSetArgs(k8sClient, RulerNameFromParent(resourceName), ns, 0, arg)
				}, time.Minute, time.Second*2).Should(BeTrue())
			})
		})

	})
})
