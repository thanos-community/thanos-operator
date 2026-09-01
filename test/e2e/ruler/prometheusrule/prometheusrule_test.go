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

package prometheusrule

import (
	"context"
	"fmt"
	"time"

	. "github.com/onsi/ginkgo/v2"
	. "github.com/onsi/gomega"

	monitoringv1 "github.com/prometheus-operator/prometheus-operator/pkg/apis/monitoring/v1"

	"github.com/thanos-community/thanos-operator/api/v1alpha1"
	"github.com/thanos-community/thanos-operator/internal/controller"
	"github.com/thanos-community/thanos-operator/internal/pkg/manifests"
	"github.com/thanos-community/thanos-operator/test/e2e/suite"
	"github.com/thanos-community/thanos-operator/test/utils"

	corev1 "k8s.io/api/core/v1"
	resourceapi "k8s.io/apimachinery/pkg/api/resource"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/types"
	"k8s.io/apimachinery/pkg/util/intstr"

	"sigs.k8s.io/controller-runtime/pkg/client"
)

var _ = Describe("Thanos Ruler PrometheusRule feature gate", Ordered, func() {
	const (
		rulerName    = "example-ruler"
		promRuleName = "example-prometheus-rule"
	)
	statefulSetName := controller.RulerNameFromParent(rulerName)
	derivedConfigMap := fmt.Sprintf("%s-promrule-0", rulerName)

	It("should bring up the ruler, its query dependency and the PrometheusRule", func() {
		// The ruler needs a query-api service present to build its StatefulSet
		// (thanosruler_controller.go: buildRuler errors on "no query API services
		// found"), so stand the query up first.
		//
		// The PrometheusRule is created BEFORE the ruler so the ruler's very first pod
		// already renders --rule-file plus the config-reloader sidecar. Create the
		// ruler first and it re-rolls when the rule is later discovered.
		suite.NewQuery(c, namespace)

		promRule := &monitoringv1.PrometheusRule{
			ObjectMeta: metav1.ObjectMeta{
				Name:      promRuleName,
				Namespace: namespace,
				Labels: map[string]string{
					manifests.DefaultPrometheusRuleLabel: manifests.DefaultPrometheusRuleValue,
				},
			},
			Spec: monitoringv1.PrometheusRuleSpec{
				Groups: []monitoringv1.RuleGroup{
					{
						Name: "example",
						Rules: []monitoringv1.Rule{
							{
								Record: "example_pr_rule",
								Expr:   intstr.FromString("vector(1)"),
							},
						},
					},
				},
			},
		}
		Expect(c.Create(context.Background(), promRule, &client.CreateOptions{})).To(Succeed())

		cr := &v1alpha1.ThanosRuler{
			ObjectMeta: metav1.ObjectMeta{
				Name:      rulerName,
				Namespace: namespace,
			},
			Spec: v1alpha1.ThanosRulerSpec{
				CommonFields: v1alpha1.CommonFields{
					Version: suite.ThanosVersion(),
				},
				// Short eval interval so the derived rule fires quickly; the default is
				// 1m, which otherwise dominates the read-back wait.
				EvaluationInterval: v1alpha1.Duration("5s"),
				StorageConfiguration: v1alpha1.StorageConfiguration{
					Size: resourceapi.MustParse("100Mi"),
				},
				RuleConfigSelector: metav1.LabelSelector{
					MatchLabels: map[string]string{
						manifests.DefaultPrometheusRuleLabel: manifests.DefaultPrometheusRuleValue,
					},
				},
				RulerMode: v1alpha1.RulerMode{
					Type: "Stateful",
					Stateful: &v1alpha1.StatefulSpec{
						ObjectStorageConfig: suite.ObjStoreConfig(),
					},
				},
				AlertmanagerURL: "http://alertmanager.com:9093",
			},
		}
		Expect(c.Create(context.Background(), cr, &client.CreateOptions{})).To(Succeed())

		Eventually(func() bool {
			return utils.VerifyStatefulSetReplicasRunning(c, 1, statefulSetName, namespace)
		}, time.Minute*5, time.Second*2).Should(BeTrue())
	})

	It("should derive a ConfigMap for the discovered PrometheusRule", func() {
		By("wiring the derived rule file into the ruler statefulset")
		Eventually(func() bool {
			return utils.VerifyStatefulSetArgs(c,
				statefulSetName,
				namespace,
				0,
				"--rule-file=/etc/thanos/rules/"+derivedConfigMap+"/"+promRuleName+".yaml",
			)
		}, time.Minute*3, time.Second*2).Should(BeTrue())

		By("creating a derived ConfigMap labelled as PrometheusRule-derived")
		Eventually(func() bool {
			cm := &corev1.ConfigMap{}
			if err := c.Get(context.Background(), types.NamespacedName{Name: derivedConfigMap, Namespace: namespace}, cm); err != nil {
				return false
			}
			return cm.Labels[manifests.PromRuleDerivedConfigMapLabel] == manifests.PromRuleDerivedConfigMapValue
		}, time.Minute*3, time.Second*2).Should(BeTrue())
	})

	It("should evaluate the PrometheusRule-derived rule and allow querying it", func() {
		localPort, cancelFn, err := utils.SetupQueryPortForward(c, namespace)
		Expect(err).To(BeNil())
		defer cancelFn()

		Eventually(func() error {
			resp, err := utils.QueryPrometheus("example_pr_rule", localPort)
			if err != nil {
				return err
			}
			if len(resp.Data.Result) == 0 {
				return fmt.Errorf("no results found for PrometheusRule-derived rule")
			}
			return nil
		}, time.Minute*3, time.Second*5).Should(Succeed())
	})
})
