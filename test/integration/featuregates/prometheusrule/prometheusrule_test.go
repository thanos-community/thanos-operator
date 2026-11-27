package prometheusrule

import (
	"fmt"

	. "github.com/onsi/ginkgo/v2"
	. "github.com/onsi/gomega"

	monitoringv1 "github.com/prometheus-operator/prometheus-operator/pkg/apis/monitoring/v1"

	monitoringthanosiov1alpha1 "github.com/thanos-community/thanos-operator/api/v1alpha1"
	"github.com/thanos-community/thanos-operator/internal/controller"
	"github.com/thanos-community/thanos-operator/internal/pkg/manifests"
	"github.com/thanos-community/thanos-operator/test/utils"

	appsv1 "k8s.io/api/apps/v1"
	corev1 "k8s.io/api/core/v1"
	apierrors "k8s.io/apimachinery/pkg/api/errors"
	resourceapi "k8s.io/apimachinery/pkg/api/resource"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/apis/meta/v1/unstructured"
	"k8s.io/apimachinery/pkg/runtime"
	"k8s.io/apimachinery/pkg/types"
	"k8s.io/apimachinery/pkg/util/intstr"
	"k8s.io/utils/ptr"
	"sigs.k8s.io/controller-runtime/pkg/client"
)

const testRulerName = "test-ruler"

const objstoreYAML = `type: S3
config:
  bucket: test
  endpoint: http://localhost:9000
  access_key: Cheesecake
  secret_key: supersecret
  http_config:
    insecure_skip_verify: false
`

// createNamespace creates an isolated namespace for a spec. Each spec gets its own
// namespace so the checks share nothing and stay collision-free and parallel-ready.
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

// createQuery creates a ThanosQuery in the given namespace so the ruler can
// discover a query API service via same-namespace SD. Without it, ruler
// reconciliation stops before any rule ConfigMaps are generated.
func createQuery(ns string) {
	Expect(k8sClient.Create(ctx, &monitoringthanosiov1alpha1.ThanosQuery{
		ObjectMeta: metav1.ObjectMeta{Name: "test-query", Namespace: ns},
		Spec: monitoringthanosiov1alpha1.ThanosQuerySpec{
			ReplicaLabels: []string{"replica"},
			Replicas:      1,
		},
	})).Should(Succeed())
}

// newRuler builds a stateful ThanosRuler with the given rule selector and optional
// tenancy config. Callers create it after the namespace, objstore secret, and query
// are in place.
func newRuler(name, ns string, selector map[string]string, tenancy *monitoringthanosiov1alpha1.RuleTenancyConfig) *monitoringthanosiov1alpha1.ThanosRuler {
	return &monitoringthanosiov1alpha1.ThanosRuler{
		ObjectMeta: metav1.ObjectMeta{Name: name, Namespace: ns},
		Spec: monitoringthanosiov1alpha1.ThanosRulerSpec{
			Replicas: 1,
			StorageConfiguration: monitoringthanosiov1alpha1.StorageConfiguration{
				Size: resourceapi.MustParse("1Gi"),
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
			RuleConfigSelector: metav1.LabelSelector{MatchLabels: selector},
			AlertmanagerURL:    "http://alertmanager.com:9093",
			RuleTenancyConfig:  tenancy,
		},
	}
}

func alertRule(name, ns string, labels map[string]string, group string, rules []monitoringv1.Rule) *monitoringv1.PrometheusRule {
	return &monitoringv1.PrometheusRule{
		ObjectMeta: metav1.ObjectMeta{Name: name, Namespace: ns, Labels: labels},
		Spec: monitoringv1.PrometheusRuleSpec{
			Groups: []monitoringv1.RuleGroup{{Name: group, Rules: rules}},
		},
	}
}

var _ = Describe("Ruler rule selectors", func() {
	DescribeTable("validates nonempty rule selectors", func(selector map[string]any, valid bool) {
		namespace := &corev1.Namespace{ObjectMeta: metav1.ObjectMeta{GenerateName: "ruler-selector-"}}
		Expect(k8sClient.Create(ctx, namespace)).To(Succeed())

		ruler := newRuler(testRulerName, namespace.Name, nil, nil)
		ruler.Spec.Paused = ptr.To(true)
		object, err := runtime.DefaultUnstructuredConverter.ToUnstructured(ruler)
		Expect(err).NotTo(HaveOccurred())
		raw := &unstructured.Unstructured{Object: object}
		raw.SetGroupVersionKind(monitoringthanosiov1alpha1.GroupVersion.WithKind("ThanosRuler"))
		if selector == nil {
			unstructured.RemoveNestedField(raw.Object, "spec", "ruleConfigSelector")
		} else {
			Expect(unstructured.SetNestedMap(raw.Object, selector, "spec", "ruleConfigSelector")).To(Succeed())
		}

		err = k8sClient.Create(ctx, raw)
		if valid {
			Expect(err).NotTo(HaveOccurred())
			if selector == nil {
				matchLabels, found, err := unstructured.NestedStringMap(raw.Object, "spec", "ruleConfigSelector", "matchLabels")
				Expect(err).NotTo(HaveOccurred())
				Expect(found).To(BeTrue())
				Expect(matchLabels).To(Equal(controller.DefaultRuleLabels))
			}
			return
		}
		Expect(apierrors.IsInvalid(err)).To(BeTrue(), "expected invalid selector, got %v", err)
		Expect(err.Error()).To(ContainSubstring("ruleConfigSelector must have at least one label selector"))

		By("rejecting an update from a valid selector to an empty selector")
		Expect(unstructured.SetNestedStringMap(raw.Object, controller.DefaultRuleLabels,
			"spec", "ruleConfigSelector", "matchLabels")).To(Succeed())
		Expect(k8sClient.Create(ctx, raw)).To(Succeed())
		Expect(unstructured.SetNestedMap(raw.Object, selector, "spec", "ruleConfigSelector")).To(Succeed())
		err = k8sClient.Update(ctx, raw)
		Expect(apierrors.IsInvalid(err)).To(BeTrue(), "expected invalid selector update, got %v", err)
		Expect(err.Error()).To(ContainSubstring("ruleConfigSelector must have at least one label selector"))
	},
		Entry("omitted selector uses the default", nil, true),
		Entry("labels only", map[string]any{"matchLabels": map[string]any{"team": "observability"}}, true),
		Entry("expressions only", map[string]any{"matchExpressions": []any{
			map[string]any{"key": "team", "operator": "Exists"},
		}}, true),
		Entry("both forms", map[string]any{
			"matchLabels": map[string]any{"team": "observability"},
			"matchExpressions": []any{
				map[string]any{"key": "disabled", "operator": "DoesNotExist"},
			},
		}, true),
		Entry("empty selector", map[string]any{}, false),
		Entry("empty labels", map[string]any{"matchLabels": map[string]any{}}, false),
		Entry("empty expressions", map[string]any{"matchExpressions": []any{}}, false),
		Entry("both forms empty", map[string]any{"matchLabels": map[string]any{}, "matchExpressions": []any{}}, false),
	)

	DescribeTable("discovers rules with expression-only rule selectors", func(source string) {
		ns := "ruler-expressions-" + source
		createNamespace(ns)
		createObjstoreSecret(ns)
		createQuery(ns)

		ruler := newRuler(testRulerName, ns, nil, nil)
		ruler.Spec.RuleConfigSelector = metav1.LabelSelector{
			MatchExpressions: []metav1.LabelSelectorRequirement{
				{Key: "team", Operator: metav1.LabelSelectorOpIn, Values: []string{"observability"}},
			},
		}
		Expect(k8sClient.Create(ctx, ruler)).To(Succeed())
		workloadName := controller.RulerNameFromParent(ruler.Name)
		Eventually(func() bool {
			return utils.VerifyStatefulSetExists(k8sClient, workloadName, ns)
		}).Should(BeTrue())

		newRule := func(name string, labels map[string]string) client.Object {
			if source == "usercfgmap" {
				return &corev1.ConfigMap{
					ObjectMeta: metav1.ObjectMeta{Name: name, Namespace: ns, Labels: labels},
					Data: map[string]string{"rules.yaml": `groups:
- name: example
  rules:
  - alert: TargetDown
    expr: up == 0
`},
				}
			}
			return alertRule(name, ns, labels, "example", []monitoringv1.Rule{
				{Alert: "TargetDown", Expr: intstr.FromString("up == 0")},
			})
		}
		selected := newRule("selected", manifests.MergeMaps(controller.DefaultRuleLabels, map[string]string{"team": "observability"}))
		excluded := newRule("excluded", manifests.MergeMaps(controller.DefaultRuleLabels, map[string]string{"team": "other"}))
		missingRequiredLabel := newRule("missing-required-label", map[string]string{"team": "observability"})
		for _, rule := range []client.Object{excluded, missingRequiredLabel, selected} {
			Expect(k8sClient.Create(ctx, rule)).To(Succeed())
		}

		ruleArg := func(name string) string {
			if source == "usercfgmap" {
				name += "-rules"
			}
			return fmt.Sprintf("--rule-file=/etc/thanos/rules/%s-%s-0/%s.yaml", ruler.Name, source, name)
		}
		expectRuleFiles := func(included, excluded string) {
			Eventually(func(g Gomega) {
				var workload appsv1.StatefulSet
				g.Expect(k8sClient.Get(ctx, types.NamespacedName{Name: workloadName, Namespace: ns}, &workload)).To(Succeed())
				args := workload.Spec.Template.Spec.Containers[0].Args
				g.Expect(args).To(ContainElement(ruleArg(included)))
				g.Expect(args).NotTo(ContainElement(ruleArg(excluded)))
				g.Expect(args).NotTo(ContainElement(ruleArg("missing-required-label")))
			}).Should(Succeed())
		}
		expectRuleFiles("selected", "excluded")

		By("updating labels to change which rule matches")
		selected.GetLabels()["team"] = "other"
		Expect(k8sClient.Update(ctx, selected)).To(Succeed())
		excluded.GetLabels()["team"] = "observability"
		Expect(k8sClient.Update(ctx, excluded)).To(Succeed())
		expectRuleFiles("excluded", "selected")
	},
		Entry("ConfigMap", "usercfgmap"),
		Entry("PrometheusRule", "promrule"),
	)
})

var _ = Describe("PrometheusRule feature gate", func() {
	It("discovers a PrometheusRule and applies tenancy to the generated rule file", func() {
		const ns = "pr-discover"
		createNamespace(ns)
		createObjstoreSecret(ns)
		createQuery(ns)

		rulerName := testRulerName
		ss := controller.RulerNameFromParent(rulerName)
		cfgmapName := fmt.Sprintf("%s-promrule-0", rulerName)

		ruler := newRuler(rulerName, ns,
			map[string]string{manifests.DefaultPrometheusRuleLabel: manifests.DefaultPrometheusRuleValue},
			&monitoringthanosiov1alpha1.RuleTenancyConfig{
				EnforcedTenantIdentifier: ptr.To("tenant"),
				TenantSpecifierLabel:     ptr.To("operator.thanos.io/tenant"),
			})
		Expect(k8sClient.Create(ctx, ruler)).Should(Succeed())

		promRule := alertRule("test-promrule", ns, map[string]string{
			manifests.DefaultPrometheusRuleLabel: manifests.DefaultPrometheusRuleValue,
			"operator.thanos.io/tenant":          "test",
		}, "example", []monitoringv1.Rule{
			{
				Alert:  "HighRequestLatency",
				Expr:   intstr.FromString(`job:request_latency_seconds:mean5m{job="myjob"} > 0.5`),
				Labels: map[string]string{"severity": "page"},
			},
		})
		Expect(k8sClient.Create(ctx, promRule)).Should(Succeed())

		Eventually(func() bool {
			arg := "--rule-file=/etc/thanos/rules/" + rulerName + "-promrule-0/" + promRule.Name + ".yaml"
			return utils.VerifyStatefulSetArgs(k8sClient, ss, ns, 0, arg)
		}).Should(BeTrue())

		Eventually(func() error {
			return utils.ConfigMapDataMatches(k8sClient, cfgmapName, ns, "test-promrule.yaml",
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
		}).Should(Succeed())
	})

	It("enforces tenancy across PrometheusRules from different tenants", func() {
		const ns = "pr-tenancy"
		createNamespace(ns)
		createObjstoreSecret(ns)
		createQuery(ns)

		rulerName := testRulerName
		cfgmapName := fmt.Sprintf("%s-promrule-0", rulerName)

		ruler := newRuler(rulerName, ns,
			map[string]string{manifests.DefaultPrometheusRuleLabel: manifests.DefaultPrometheusRuleValue},
			&monitoringthanosiov1alpha1.RuleTenancyConfig{
				EnforcedTenantIdentifier: ptr.To("tenant_id"),
				TenantSpecifierLabel:     ptr.To("tenant"),
			})
		Expect(k8sClient.Create(ctx, ruler)).Should(Succeed())

		tenantA := alertRule("tenant-a-rule", ns, map[string]string{
			manifests.DefaultPrometheusRuleLabel: manifests.DefaultPrometheusRuleValue,
			"tenant":                             "team-a",
		}, "tenant-a-alerts", []monitoringv1.Rule{
			{
				Alert:  "HighCPU",
				Expr:   intstr.FromString(`cpu_usage{job="app"} > 80`),
				Labels: map[string]string{"severity": "warning"},
			},
			{
				Record: "app:requests:rate5m",
				Expr:   intstr.FromString(`rate(http_requests_total{job="app"}[5m])`),
			},
		})
		Expect(k8sClient.Create(ctx, tenantA)).Should(Succeed())

		Eventually(func() error {
			return utils.ConfigMapDataMatches(k8sClient, cfgmapName, ns, "tenant-a-rule.yaml",
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
		}).Should(Succeed())

		tenantB := alertRule("tenant-b-rule", ns, map[string]string{
			manifests.DefaultPrometheusRuleLabel: manifests.DefaultPrometheusRuleValue,
			"tenant":                             "team-b",
		}, "tenant-b-alerts", []monitoringv1.Rule{
			{
				Alert:  "LowMemory",
				Expr:   intstr.FromString(`memory_available{job="database"} < 20`),
				Labels: map[string]string{"severity": "critical"},
			},
		})
		Expect(k8sClient.Create(ctx, tenantB)).Should(Succeed())

		Eventually(func() error {
			return utils.ConfigMapDataMatches(k8sClient, cfgmapName, ns, "tenant-b-rule.yaml",
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
		}).Should(Succeed())
	})

	It("re-filters PrometheusRules when a custom selector label is added", func() {
		const ns = "pr-selector"
		createNamespace(ns)
		createObjstoreSecret(ns)
		createQuery(ns)

		rulerName := testRulerName
		ss := controller.RulerNameFromParent(rulerName)
		cfgmapName := fmt.Sprintf("%s-promrule-0", rulerName)

		ruler := newRuler(rulerName, ns,
			map[string]string{manifests.DefaultPrometheusRuleLabel: manifests.DefaultPrometheusRuleValue}, nil)
		Expect(k8sClient.Create(ctx, ruler)).Should(Succeed())

		// A rule matching only the default selector is picked up initially.
		defaultRule := alertRule("test-promrule", ns, map[string]string{
			manifests.DefaultPrometheusRuleLabel: manifests.DefaultPrometheusRuleValue,
		}, "example", []monitoringv1.Rule{
			{
				Alert:  "HighRequestLatency",
				Expr:   intstr.FromString(`job:request_latency_seconds:mean5m{job="myjob"} > 0.5`),
				Labels: map[string]string{"severity": "page"},
			},
		})
		Expect(k8sClient.Create(ctx, defaultRule)).Should(Succeed())

		Eventually(func() bool {
			arg := "--rule-file=/etc/thanos/rules/" + rulerName + "-promrule-0/test-promrule.yaml"
			return utils.VerifyStatefulSetArgs(k8sClient, ss, ns, 0, arg)
		}).Should(BeTrue())

		// Narrow the selector by adding a custom label.
		const customKey, customVal = "foo", "bar"
		Eventually(func() error {
			cur := &monitoringthanosiov1alpha1.ThanosRuler{}
			if err := k8sClient.Get(ctx, types.NamespacedName{Name: rulerName, Namespace: ns}, cur); err != nil {
				return err
			}
			cur.Spec.RuleConfigSelector.MatchLabels[customKey] = customVal
			return k8sClient.Update(ctx, cur)
		}).Should(Succeed())

		customRule := alertRule("custom-label-promrule", ns, map[string]string{
			manifests.DefaultPrometheusRuleLabel: manifests.DefaultPrometheusRuleValue,
			customKey:                            customVal,
		}, "custom-group", []monitoringv1.Rule{
			{
				Alert:  "CustomAlert",
				Expr:   intstr.FromString(`up == 0`),
				Labels: map[string]string{"severity": "critical"},
			},
		})
		Expect(k8sClient.Create(ctx, customRule)).Should(Succeed())

		Eventually(func() bool {
			arg := "--rule-file=/etc/thanos/rules/" + rulerName + "-promrule-0/custom-label-promrule.yaml"
			return utils.VerifyStatefulSetArgs(k8sClient, ss, ns, 0, arg)
		}).Should(BeTrue())

		Eventually(func() bool {
			cm := &corev1.ConfigMap{}
			if err := k8sClient.Get(ctx, types.NamespacedName{Name: cfgmapName, Namespace: ns}, cm); err != nil {
				return false
			}
			return cm.Labels[customKey] == customVal &&
				cm.Labels[manifests.PromRuleDerivedConfigMapLabel] == manifests.PromRuleDerivedConfigMapValue
		}).Should(BeTrue())

		// The rule without the custom label is no longer referenced.
		Eventually(func() bool {
			arg := "--rule-file=/etc/thanos/rules/" + rulerName + "-promrule-0/test-promrule.yaml"
			return !utils.VerifyStatefulSetArgs(k8sClient, ss, ns, 0, arg)
		}).Should(BeTrue())
	})

	It("removes the rule file when a PrometheusRule is deleted", func() {
		const ns = "pr-cleanup"
		createNamespace(ns)
		createObjstoreSecret(ns)
		createQuery(ns)

		rulerName := testRulerName
		ss := controller.RulerNameFromParent(rulerName)

		ruler := newRuler(rulerName, ns,
			map[string]string{manifests.DefaultPrometheusRuleLabel: manifests.DefaultPrometheusRuleValue}, nil)
		Expect(k8sClient.Create(ctx, ruler)).Should(Succeed())

		promRule := alertRule("cleanup-test-promrule", ns, map[string]string{
			manifests.DefaultPrometheusRuleLabel: manifests.DefaultPrometheusRuleValue,
		}, "promrule-test", []monitoringv1.Rule{
			{Alert: "PrometheusRuleAlert", Expr: intstr.FromString(`up == 0`)},
		})
		Expect(k8sClient.Create(ctx, promRule)).Should(Succeed())

		arg := "--rule-file=/etc/thanos/rules/" + rulerName + "-promrule-0/cleanup-test-promrule.yaml"
		Eventually(func() bool {
			return utils.VerifyStatefulSetArgs(k8sClient, ss, ns, 0, arg)
		}).Should(BeTrue())

		Expect(k8sClient.Delete(ctx, promRule)).Should(Succeed())
		Eventually(func() bool {
			return !utils.VerifyStatefulSetArgs(k8sClient, ss, ns, 0, arg)
		}).Should(BeTrue())
	})
})
