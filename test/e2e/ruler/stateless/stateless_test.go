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

package stateless

import (
	"context"
	"fmt"
	"time"

	. "github.com/onsi/ginkgo/v2"
	. "github.com/onsi/gomega"

	"github.com/thanos-community/thanos-operator/api/v1alpha1"
	"github.com/thanos-community/thanos-operator/internal/controller"
	"github.com/thanos-community/thanos-operator/internal/pkg/manifests"
	"github.com/thanos-community/thanos-operator/internal/pkg/manifests/receive"
	"github.com/thanos-community/thanos-operator/test/e2e/suite"
	"github.com/thanos-community/thanos-operator/test/utils"

	corev1 "k8s.io/api/core/v1"
	resourceapi "k8s.io/apimachinery/pkg/api/resource"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/utils/ptr"

	"sigs.k8s.io/controller-runtime/pkg/client"
)

var _ = Describe("Thanos stateless ruler", Ordered, func() {
	const statelessRulerName = "stateless-ruler"
	var receiveName string

	It("should bring up its receive and query dependencies", func() {
		receiveName = suite.NewReceive(c, namespace)
		suite.NewQuery(c, namespace)
	})

	It("should generate correct resources", func() {
		r := &v1alpha1.ThanosRuler{
			ObjectMeta: metav1.ObjectMeta{
				Name:      statelessRulerName,
				Namespace: namespace,
			},
			Spec: v1alpha1.ThanosRulerSpec{
				QueryLabelSelector: &metav1.LabelSelector{
					MatchLabels: map[string]string{
						manifests.DefaultQueryAPILabel: manifests.DefaultQueryAPIValue,
					},
				},
				CommonFields: v1alpha1.CommonFields{
					Version: suite.ThanosVersion(),
				},
				// Short eval interval so the derived rule fires quickly; the default is
				// 1m, which otherwise dominates the read-back wait.
				EvaluationInterval: v1alpha1.Duration("5s"),
				Replicas:           1,
				StorageConfiguration: v1alpha1.StorageConfiguration{
					Size: resourceapi.MustParse("1Gi"),
				},
				RulerMode: v1alpha1.RulerMode{
					Type:      "Stateless",
					Stateless: &v1alpha1.StatelessSpec{},
				},
				RuleConfigSelector: metav1.LabelSelector{
					MatchLabels: map[string]string{
						manifests.DefaultPrometheusRuleLabel: manifests.DefaultPrometheusRuleValue,
						"stateless":                          "true",
					},
				},
				AlertmanagerURL: "http://alertmanager.com:9093",
				RuleTenancyConfig: &v1alpha1.RuleTenancyConfig{
					EnforcedTenantIdentifier: ptr.To("tenant_id"),
					TenantSpecifierLabel:     ptr.To("operator.thanos.io/tenant"),
				},
			},
		}
		err := c.Create(context.Background(), r)
		Expect(err).To(BeNil())

		Eventually(func() bool {
			return utils.VerifyStatefulSetReplicasRunning(c, 1, controller.RulerNameFromParent(statelessRulerName), namespace)
		}, time.Minute*3, time.Second*5).Should(BeTrue())
	})

	It("should set up rule resource", func() {
		// Source the rule from a plain ConfigMap rather than a PrometheusRule so this
		// suite exercises only the stateless data path (evaluate -> remote-write ->
		// query back) and stays independent of the PrometheusRule discovery feature,
		// which the prometheusrule suite covers. Tenancy works the same either way:
		// the tenant comes from the operator.thanos.io/tenant label.
		ruleCM := &corev1.ConfigMap{
			ObjectMeta: metav1.ObjectMeta{
				Name:      "stateless-rule",
				Namespace: namespace,
				Labels: map[string]string{
					manifests.DefaultPrometheusRuleLabel: manifests.DefaultPrometheusRuleValue,
					"operator.thanos.io/tenant":          "stateless_tenant",
					"stateless":                          "true",
				},
			},
			Data: map[string]string{
				"rules.yaml": `groups:
  - name: example-rule
    rules:
      - record: example_stateless_rule
        expr: sum(test_metric)
`,
			},
		}
		err := c.Create(context.Background(), ruleCM, &client.CreateOptions{})
		Expect(err).To(BeNil())

		routerLabels := map[string]string{
			manifests.ComponentLabel: receive.RouterComponentName,
			manifests.OwnerLabel:     receiveName,
		}
		header := map[string]string{
			"THANOS-TENANT": "stateless_tenant",
		}
		Eventually(func() error {
			return utils.DoRemoteWriteRequest(c, utils.StatelessRemoteWriteRequest(), namespace, routerLabels, header, receive.RemoteWritePort)
		}, time.Minute*1, time.Second*1).Should(Succeed())
	})

	It("should allow querying of evaluated rules", func() {
		cancelFn, err := utils.SetupQueryPortForward(c, namespace)
		Expect(err).To(BeNil())
		defer cancelFn()

		Eventually(func() error {
			resp, err := utils.QueryPrometheus(`example_stateless_rule{tenant_id="stateless_tenant"}`)
			if err != nil {
				return err
			}
			if len(resp.Data.Result) == 0 {
				return fmt.Errorf("no results found for evaluated stateless rule")
			}
			return nil
		}, time.Minute*3, time.Second*5).Should(Succeed())
	})
})
