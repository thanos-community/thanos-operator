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

package core

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

	appsv1 "k8s.io/api/apps/v1"
	corev1 "k8s.io/api/core/v1"
	resourceapi "k8s.io/apimachinery/pkg/api/resource"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/util/intstr"
	"k8s.io/utils/ptr"

	"sigs.k8s.io/controller-runtime/pkg/client"
)

const (
	receiveName = "example-receive"
	queryName   = "example-query"
	rulerName   = "example-ruler"

	prometheusPort = 9090

	hashringName    = "default"
	hashringTwoName = "two"
)

var _ = Describe("core", Ordered, func() {
	Context("Operator", func() {

		It("should run successfully", func() {
			By("validating that the controller-manager deployment is available")
			Eventually(func() error {
				deployments := &appsv1.DeploymentList{}
				if err := c.List(context.Background(), deployments,
					client.MatchingLabels{"control-plane": "controller-manager"},
					client.InNamespace(operatorNamespace)); err != nil {
					return err
				}
				if len(deployments.Items) != 1 {
					return fmt.Errorf("expected 1 controller-manager deployment, got %d", len(deployments.Items))
				}
				if deployments.Items[0].Status.ReadyReplicas < 1 {
					return fmt.Errorf("controller-manager has no ready replicas")
				}
				return nil
			}, time.Minute, time.Second*2).Should(Succeed())
		})
	})

	Describe("Thanos Receive", Ordered, func() {
		routerName := controller.ReceiveRouterNameFromParent(receiveName)
		ingesterName := controller.ReceiveIngesterNameFromParent(receiveName, hashringName)
		ingesterTwoName := controller.ReceiveIngesterNameFromParent(receiveName, hashringTwoName)

		Context("When ThanosReceive is created with hashrings", func() {
			It("should bring up the ingest components", func() {
				cr := &v1alpha1.ThanosReceive{
					ObjectMeta: metav1.ObjectMeta{
						Name:      receiveName,
						Namespace: namespace,
					},
					Spec: v1alpha1.ThanosReceiveSpec{
						StatefulSetFields: v1alpha1.StatefulSetFields{
							MinReadySeconds: ptr.To(int32(1)),
						},
						Ingester: v1alpha1.IngesterSpec{
							DefaultObjectStorageConfig: suite.ObjStoreConfig(),
							Hashrings: []v1alpha1.IngesterHashringSpec{
								{
									Name: hashringName,
									StorageConfiguration: v1alpha1.StorageConfiguration{
										Size: resourceapi.MustParse("100Mi"),
									},
									CommonFields: v1alpha1.CommonFields{
										Version: suite.ThanosVersion(),
									},
								},
								{
									Name: hashringTwoName,
									StorageConfiguration: v1alpha1.StorageConfiguration{
										Size: resourceapi.MustParse("100Mi"),
									},
									CommonFields: v1alpha1.CommonFields{
										Version: suite.ThanosVersion(),
									},
									TenancyConfig: &v1alpha1.TenancyConfig{
										Tenants: []string{
											"tenant1",
										},
										TenantMatcherType: "exact",
									},
								},
							},
						},
						Router: v1alpha1.RouterSpec{
							CommonFields: v1alpha1.CommonFields{
								Version: suite.ThanosVersion(),
							},
							Replicas:          1,
							ReplicationFactor: 1,
							HashringPolicy:    ptr.To(v1alpha1.HashringPolicyStatic),
							ExternalLabels: map[string]string{
								"receive": "true",
							},
						},
					},
				}
				err := c.Create(context.Background(), cr)
				Expect(err).NotTo(HaveOccurred())
				Eventually(func() bool {
					return utils.VerifyStatefulSetReplicasRunning(c, 1, ingesterName, namespace)
				}, time.Minute*5, time.Second*2).Should(BeTrue())
			})

			Context("When the ingesters have been created", func() {
				It("should bring up the router components", func() {
					Eventually(func() bool {
						return utils.VerifyDeploymentReplicasRunning(c, 1, routerName, namespace)
					}, time.Minute*5, time.Second*2).Should(BeTrue())
				})
				It("should create a ConfigMap with the correct hashring configuration", func() {
					//nolint:lll
					expect := fmt.Sprintf(`[
    {
        "hashring": "%s",
        "endpoints": [
            {
                "address": "%s-0.%s.%s.svc:10901",
				"capnproto_address": "",
                "az": ""
            }
        ],
        "algorithm": "ketama",
		"external_labels": {}
    },
    {
        "hashring": "%s",
        "tenants": [
            "tenant1"
        ],
        "tenant_matcher_type": "exact",
        "endpoints": [
            {
                "address": "%s-0.%s.%s.svc:10901",
				"capnproto_address": "",
                "az": ""
            }
        ],
        "algorithm": "ketama",
		"external_labels": {}
    }
]`, hashringName, ingesterName, ingesterName, namespace, hashringTwoName, ingesterTwoName, ingesterTwoName, namespace)
					Eventually(func() bool {
						return utils.VerifyConfigMapContents(c, routerName, namespace, receive.HashringConfigKey, expect)
					}, time.Minute*5, time.Second*2).Should(BeTrue())
				})
			})

		})

		Context("When ThanosReceive is fully operational", func() {
			It("should accept metrics over remote write", func() {
				matchLabels := map[string]string{
					manifests.ComponentLabel: receive.RouterComponentName,
					manifests.OwnerLabel:     receiveName,
				}
				Eventually(func() error {
					return utils.DoRemoteWriteRequest(c, utils.DefaultRemoteWriteRequest(), namespace, matchLabels, nil, receive.RemoteWritePort)
				}, time.Minute*2, time.Second*1).Should(Succeed())
			})
		})
	})

	Describe("Thanos Query", Ordered, func() {
		Context("When ThanosQuery is created", func() {
			It("should bring up the thanos query components", func() {
				cr := &v1alpha1.ThanosQuery{
					ObjectMeta: metav1.ObjectMeta{
						Name:      queryName,
						Namespace: namespace,
					},
					Spec: v1alpha1.ThanosQuerySpec{
						CommonFields: v1alpha1.CommonFields{
							Version: suite.ThanosVersion(),
						},
						Replicas: 1,
						ReplicaLabels: []string{
							"prometheus_replica",
							"replica",
							"rule_replica",
						},
						StoreLabelSelector: &metav1.LabelSelector{
							MatchLabels: map[string]string{
								manifests.DefaultStoreAPILabel: manifests.DefaultStoreAPIValue,
							},
						},
						// Default is 30s. When the ruler pod rolls (e.g. first rule file
						// added), query only reconnects to the new IP on the next SD tick,
						// so a low interval keeps the evaluated-rules query fast.
						Additional: v1alpha1.Additional{
							Args: []string{"--store.sd-dns-interval=5s"},
						},
					},
				}
				err := c.Create(context.Background(), cr)
				Expect(err).NotTo(HaveOccurred())

				deploymentName := controller.QueryNameFromParent(queryName)
				Eventually(func() bool {
					return utils.VerifyDeploymentReplicasRunning(c, 1, deploymentName, namespace)
				}, time.Minute*1, time.Second*1).Should(BeTrue())
				svcName := controller.ReceiveIngesterNameFromParent(receiveName, hashringName)
				Eventually(func() bool {
					return utils.VerifyDeploymentArgs(c,
						deploymentName,
						namespace,
						0,
						fmt.Sprintf("--endpoint=dnssrv+_grpc._tcp.%s.%s.svc", svcName, namespace),
					)
				}, time.Minute*1, time.Second*1).Should(BeTrue())
			})
		})
		Context("When querying for written metrics", func() {
			It("should be able to query the test metric written via remote write", func() {
				ctx := context.Background()
				selector := client.MatchingLabels{
					manifests.ComponentLabel: "query-layer",
				}
				queryPods := &corev1.PodList{}
				err := c.List(ctx, queryPods, selector, &client.ListOptions{Namespace: namespace})
				Expect(err).NotTo(HaveOccurred())
				Expect(queryPods.Items).NotTo(BeEmpty())

				pod := queryPods.Items[0].Name
				localPort, cancelFn, err := utils.StartPortForward(ctx, intstr.FromInt32(prometheusPort), "https", pod, namespace)
				Expect(err).NotTo(HaveOccurred())
				defer cancelFn()

				Eventually(func() error {
					resp, err := utils.QueryPrometheus("test_metric", localPort)
					if err != nil {
						return err
					}
					if len(resp.Data.Result) == 0 {
						return fmt.Errorf("no results found for test_metric")
					}
					return nil
				}, time.Minute*3, time.Second*5).Should(Succeed())
			})
		})
	})

	Describe("Thanos Ruler", Ordered, func() {
		statefulSetName := controller.RulerNameFromParent(rulerName)
		svcName := controller.QueryNameFromParent(queryName)

		Context("When ThanosRuler is created", func() {
			It("should bring up the rulers components", func() {
				cr := &v1alpha1.ThanosRuler{
					ObjectMeta: metav1.ObjectMeta{
						Name:      rulerName,
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
						StorageConfiguration: v1alpha1.StorageConfiguration{
							Size: resourceapi.MustParse("100Mi"),
						},
						RuleConfigSelector: metav1.LabelSelector{
							MatchLabels: map[string]string{
								manifests.DefaultPrometheusRuleLabel: manifests.DefaultPrometheusRuleValue,
								"stateful":                           "true",
							},
						},
						RulerMode: v1alpha1.RulerMode{
							Type: "Stateful",
							Stateful: &v1alpha1.StatefulSpec{
								ObjectStorageConfig: suite.ObjStoreConfig(),
							},
						},
						// Match the ruler feature suites: without this the CRD default
						// of 1m makes "query evaluated rules" wait a full eval cycle.
						EvaluationInterval: v1alpha1.Duration("5s"),
						AlertmanagerURL:    "http://alertmanager.com:9093",
					},
				}

				err := c.Create(context.Background(), cr)
				Expect(err).NotTo(HaveOccurred())

				Eventually(func() bool {
					return utils.VerifyStatefulSetReplicasRunning(c, 1, statefulSetName, namespace)
				}, time.Minute*5, time.Second*1).Should(BeTrue())
			})

			It("should validate the ruler has discovered the query service", func() {
				Eventually(func() bool {
					return utils.VerifyStatefulSetArgs(c,
						statefulSetName,
						namespace,
						0,
						fmt.Sprintf("--query=dnssrv+_http._tcp.%s.%s.svc", svcName, namespace),
					)
				}, time.Minute*3, time.Second*1).Should(BeTrue())
			})

			It("should pick up a rule configmap when configured", func() {
				cfgmap := &corev1.ConfigMap{
					ObjectMeta: metav1.ObjectMeta{
						Name:      "my-rules",
						Namespace: namespace,
						Labels: map[string]string{
							manifests.DefaultPrometheusRuleLabel: manifests.DefaultPrometheusRuleValue,
							"stateful":                           "true",
						},
					},
					Data: map[string]string{
						"my-rules.yaml": `groups:
  - name: example
    rules:
      - alert: HighRequestRate
        expr: sum(rate(http_requests_total[5m])) > 10
        for: 1m
        labels:
          severity: critical
        annotations:
          summary: High request rate
      - record: example_recording_rule
        expr: vector(1)
`,
					},
				}
				err := c.Create(context.Background(), cfgmap)
				Expect(err).NotTo(HaveOccurred())
				Eventually(func() bool {
					return utils.VerifyStatefulSetArgs(c,
						statefulSetName,
						namespace,
						0,
						"--rule-file=/etc/thanos/rules/"+rulerName+"-usercfgmap-0/my-rules-my-rules.yaml",
					)
				}, time.Minute*3, time.Second*1).Should(BeTrue())
			})

			It("should allow querying of evaluated rules", func() {
				localPort, cancelFn, err := utils.SetupQueryPortForward(c, namespace)
				Expect(err).NotTo(HaveOccurred())
				defer cancelFn()

				Eventually(func() error {
					resp, err := utils.QueryPrometheus(`example_recording_rule`, localPort)
					if err != nil {
						return err
					}
					if len(resp.Data.Result) == 0 {
						return fmt.Errorf("no results found for recording rule")
					}
					return nil
				}, time.Minute*3, time.Second*1).Should(Succeed())
			})
		})
	})
})
