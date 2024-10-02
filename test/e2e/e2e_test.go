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

package e2e

import (
	"context"
	"fmt"
	"os"
	"os/exec"
	"time"

	monitoringv1 "github.com/prometheus-operator/prometheus-operator/pkg/apis/monitoring/v1"

	rbacv1 "k8s.io/api/rbac/v1"

	"github.com/thanos-community/thanos-operator/internal/pkg/manifests"
	"github.com/thanos-community/thanos-operator/internal/pkg/manifests/store"
	"k8s.io/apimachinery/pkg/util/intstr"

	. "github.com/onsi/ginkgo/v2"
	. "github.com/onsi/gomega"

	"github.com/thanos-community/thanos-operator/api/v1alpha1"
	"github.com/thanos-community/thanos-operator/internal/controller"
	"github.com/thanos-community/thanos-operator/internal/pkg/manifests/receive"
	"github.com/thanos-community/thanos-operator/test/utils"

	appsv1 "k8s.io/api/apps/v1"
	corev1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/runtime"
	"sigs.k8s.io/controller-runtime/pkg/client"
	"sigs.k8s.io/controller-runtime/pkg/client/config"
	logf "sigs.k8s.io/controller-runtime/pkg/log"
	"sigs.k8s.io/controller-runtime/pkg/log/zap"
)

const (
	namespace = "thanos-operator-system"

	objStoreSecret    = "thanos-object-storage"
	objStoreSecretKey = "thanos.yaml"

	receiveName = "example-receive"
	storeName   = "example-store"
	queryName   = "example-query"
	rulerName   = "example-ruler"
	compactName = "example-compact"

	prometheusPort = 9090

	hashringName = "default"
)

var _ = Describe("controller", Ordered, func() {
	var c client.Client

	BeforeAll(func() {
		logf.SetLogger(zap.New(zap.WriteTo(GinkgoWriter), zap.UseDevMode(true)))
		By("installing prometheus operator")
		Expect(utils.InstallPrometheusOperator()).To(Succeed())

		By("installing the cert-manager")
		Expect(utils.InstallCertManager()).To(Succeed())

		By("creating manager namespace")
		cmd := exec.Command("kubectl", "create", "ns", namespace)
		_, _ = utils.Run(cmd)

		By("install MinIO")
		Expect(utils.InstallMinIO()).To(Succeed())

		By("create secret")
		Expect(utils.CreateMinioObjectStorageSecret()).To(Succeed())

		scheme := runtime.NewScheme()
		if err := v1alpha1.AddToScheme(scheme); err != nil {
			fmt.Println("failed to add scheme")
			os.Exit(1)
		}
		if err := appsv1.AddToScheme(scheme); err != nil {
			fmt.Println("failed to add scheme")
			os.Exit(1)
		}
		if err := corev1.AddToScheme(scheme); err != nil {
			fmt.Println("failed to add scheme")
			os.Exit(1)
		}
		if err := monitoringv1.AddToScheme(scheme); err != nil {
			fmt.Println("failed to add scheme")
			os.Exit(1)
		}
		if err := rbacv1.AddToScheme(scheme); err != nil {
			fmt.Println("failed to add scheme")
			os.Exit(1)
		}

		cl, err := client.New(config.GetConfigOrDie(), client.Options{
			Scheme: scheme,
		})
		if err != nil {
			fmt.Println("failed to create client")
			os.Exit(1)
		}
		c = cl

		By("Setup prometheus")
		Expect(utils.SetUpPrometheus(c)).To(Succeed())
	})

	AfterAll(func() {
		By("uninstalling the Prometheus manager bundle")
		utils.UninstallPrometheusOperator()

		By("uninstalling the cert-manager bundle")
		utils.UninstallCertManager()

		By("uninstalling MinIO")
		utils.UninstallMinIO()

		By("removing manager namespace")
		cmd := exec.Command("kubectl", "delete", "ns", namespace)
		_, _ = utils.Run(cmd)
	})

	Context("Operator", func() {

		It("should run successfully", func() {
			var controllerPodName string
			var err error

			// projectimage stores the name of the image used in the example
			var projectimage = "example.com/thanos-operator:v0.0.1"

			By("building the manager(Operator) image")
			cmd := exec.Command("make", "docker-build", fmt.Sprintf("IMG=%s", projectimage))
			_, err = utils.Run(cmd)
			ExpectWithOffset(1, err).NotTo(HaveOccurred())

			By("loading the the manager(Operator) image on Kind")
			err = utils.LoadImageToKindClusterWithName(projectimage)
			ExpectWithOffset(1, err).NotTo(HaveOccurred())

			By("installing CRDs")
			cmd = exec.Command("make", "install")
			_, err = utils.Run(cmd)
			ExpectWithOffset(1, err).NotTo(HaveOccurred())

			By("deploying the controller-manager")
			cmd = exec.Command("make", "deploy", fmt.Sprintf("IMG=%s", projectimage))
			_, err = utils.Run(cmd)
			ExpectWithOffset(1, err).NotTo(HaveOccurred())

			By("validating that the controller-manager pod is running as expected")
			verifyControllerUp := func() error {
				// Get pod name

				cmd = exec.Command("kubectl", "get",
					"pods", "-l", "control-plane=controller-manager",
					"-o", "go-template={{ range .items }}"+
						"{{ if not .metadata.deletionTimestamp }}"+
						"{{ .metadata.name }}"+
						"{{ \"\\n\" }}{{ end }}{{ end }}",
					"-n", namespace,
				)

				podOutput, err := utils.Run(cmd)
				ExpectWithOffset(2, err).NotTo(HaveOccurred())
				podNames := utils.GetNonEmptyLines(string(podOutput))
				if len(podNames) != 1 {
					return fmt.Errorf("expect 1 controller pods running, but got %d", len(podNames))
				}
				controllerPodName = podNames[0]
				ExpectWithOffset(2, controllerPodName).Should(ContainSubstring("controller-manager"))

				// Validate pod status
				cmd = exec.Command("kubectl", "get",
					"pods", controllerPodName, "-o", "jsonpath={.status.phase}",
					"-n", namespace,
				)
				status, err := utils.Run(cmd)
				ExpectWithOffset(2, err).NotTo(HaveOccurred())
				if string(status) != "Running" {
					return fmt.Errorf("controller pod in %s status", status)
				}
				return nil
			}
			EventuallyWithOffset(1, verifyControllerUp, time.Minute, time.Second).Should(Succeed())

		})
	})

	Describe("Thanos Receive", Ordered, func() {
		routerName := controller.ReceiveRouterNameFromParent(receiveName)
		ingesterName := controller.ReceiveIngesterNameFromParent(receiveName, hashringName)

		Context("When ThanosReceive is created with hashrings", func() {
			It("should bring up the ingest components", func() {
				cr := &v1alpha1.ThanosReceive{
					ObjectMeta: metav1.ObjectMeta{
						Name:      receiveName,
						Namespace: namespace,
					},
					Spec: v1alpha1.ThanosReceiveSpec{
						Ingester: v1alpha1.IngesterSpec{
							DefaultObjectStorageConfig: v1alpha1.ObjectStorageConfig{
								LocalObjectReference: corev1.LocalObjectReference{
									Name: objStoreSecret,
								},
								Key: objStoreSecretKey,
							},
							Hashrings: []v1alpha1.IngesterHashringSpec{
								{
									Name:        hashringName,
									StorageSize: "100Mi",
								},
							},
						},
					},
				}
				err := c.Create(context.Background(), cr, &client.CreateOptions{})
				Expect(err).To(BeNil())
				Eventually(func() bool {
					return utils.VerifyStatefulSetReplicasRunning(c, 1, ingesterName, namespace)
				}, time.Minute*5, time.Second*10).Should(BeTrue())
			})

			It("should create a ConfigMap with the correct hashring configuration", func() {
				//nolint:lll
				expect := fmt.Sprintf(`[
    {
        "hashring": "default",
        "tenant_matcher_type": "exact",
        "endpoints": [
            {
                "address": "%s-0.%s.thanos-operator-system.svc.cluster.local:10901",
                "az": ""
            }
        ]
    }
]`, ingesterName, ingesterName)
				Eventually(func() bool {
					return utils.VerifyConfigMapContents(c, routerName, namespace, receive.HashringConfigKey, expect)
				}, time.Minute*5, time.Second*10).Should(BeTrue())
			})
		})

		Context("When the hashring configuration exists", func() {
			It("should bring up the router components", func() {
				Eventually(func() bool {
					return utils.VerifyDeploymentReplicasRunning(c, 1, routerName, namespace)
				}, time.Minute*5, time.Second*10).Should(BeTrue())
			})
		})

		Context("When ThanosReceive is fully operational", func() {
			It("should accept metrics over remote write", func() {
				ctx := context.Background()
				selector := client.MatchingLabels{
					manifests.ComponentLabel: receive.RouterComponentName,
				}
				router := &corev1.PodList{}
				err := c.List(ctx, router, selector, &client.ListOptions{Namespace: namespace})
				Expect(err).To(BeNil())
				Expect(len(router.Items)).To(Equal(1))

				pod := router.Items[0].Name
				port := intstr.IntOrString{IntVal: receive.RemoteWritePort}
				cancelFn, err := utils.StartPortForward(ctx, port, "https", pod, namespace)
				Expect(err).To(BeNil())
				defer cancelFn()

				Eventually(func() error {
					return utils.RemoteWrite(utils.DefaultRemoteWriteRequest(), nil, nil)
				}, time.Minute*1, time.Second*5).Should(Succeed())

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
						CommonThanosFields: v1alpha1.CommonThanosFields{},
						Replicas:           1,
						Labels: map[string]string{
							"some-label": "xyz",
						},
						QuerierReplicaLabels: []string{
							"prometheus_replica",
							"replica",
							"rule_replica",
						},
						StoreLabelSelector: &metav1.LabelSelector{
							MatchLabels: map[string]string{
								"operator.thanos.io/store-api": "true",
							},
						},
					},
				}
				err := c.Create(context.Background(), cr, &client.CreateOptions{})
				Expect(err).To(BeNil())

				deploymentName := controller.QueryNameFromParent(queryName)
				Eventually(func() bool {
					return utils.VerifyDeploymentReplicasRunning(c, 1, deploymentName, namespace)
				}, time.Minute*1, time.Second*10).Should(BeTrue())
				svcName := controller.ReceiveIngesterNameFromParent(receiveName, hashringName)
				Eventually(func() bool {
					return utils.VerifyDeploymentArgs(c,
						deploymentName,
						namespace,
						0,
						fmt.Sprintf("--endpoint=dnssrv+_grpc._tcp.%s.thanos-operator-system.svc.cluster.local", svcName),
					)
				}, time.Minute*1, time.Second*10).Should(BeTrue())
			})
		})
		Context("When Query Frontend is enabled", func() {
			It("should bring up the query frontend components", func() {
				tenSeconds := v1alpha1.Duration("10s")
				updatedCR := &v1alpha1.ThanosQuery{}
				err := c.Get(context.Background(), client.ObjectKey{Name: queryName, Namespace: namespace}, updatedCR)
				Expect(err).To(BeNil())

				updatedCR.Spec.QueryFrontend = &v1alpha1.QueryFrontendSpec{
					Replicas:             2,
					CompressResponses:    true,
					LogQueriesLongerThan: &tenSeconds,
					QueryRangeMaxRetries: 3,
					LabelsMaxRetries:     3,
					QueryLabelSelector: &metav1.LabelSelector{
						MatchLabels: map[string]string{
							manifests.DefaultQueryAPILabel: manifests.DefaultQueryAPIValue,
						},
					},
				}
				err = c.Update(context.Background(), updatedCR)
				Expect(err).To(BeNil())
				svcName := controller.QueryNameFromParent(queryName)
				deploymentName := controller.QueryFrontendNameFromParent(queryName)
				Eventually(func() bool {
					return utils.VerifyDeploymentReplicasRunning(c, 2, deploymentName, namespace)
				}, time.Minute*5, time.Second*10).Should(BeTrue())

				Eventually(func() bool {
					return utils.VerifyDeploymentArgs(c,
						deploymentName,
						namespace,
						0,
						"--query-frontend.downstream-url=http://"+svcName+"."+namespace+".svc.cluster.local:9090",
					)
				}, time.Minute*1, time.Second*10).Should(BeTrue())
			})
		})
		Context("When a client changes the ThanosQuery CR", func() {
			It("should update the Deployment", func() {

				updatedCR := &v1alpha1.ThanosQuery{}
				err := c.Get(context.Background(), client.ObjectKey{Name: queryName, Namespace: namespace}, updatedCR)
				Expect(err).To(BeNil())
				twentySeconds := v1alpha1.Duration("20s")

				updatedCR.Spec.QueryFrontend.Replicas = 3
				updatedCR.Spec.QueryFrontend.LogQueriesLongerThan = &twentySeconds

				err = c.Update(context.Background(), updatedCR)
				Expect(err).To(BeNil())

				deploymentName := controller.QueryFrontendNameFromParent(queryName)
				Eventually(func() bool {
					return utils.VerifyDeploymentReplicasRunning(c, 3, deploymentName, namespace)
				}, time.Minute*5, time.Second*10).Should(BeTrue())

				Eventually(func() bool {
					return utils.VerifyDeploymentArgs(c,
						deploymentName,
						namespace,
						0,
						"--query-frontend.log-queries-longer-than=20s",
					)
				}, time.Minute*1, time.Second*10).Should(BeTrue())
			})
		})
		Context("Discover Thanos Query as a target", func() {
			It("Prometheus should discover Thanos Query as a target", func() {
				pods := &corev1.PodList{}
				selector := client.MatchingLabels{
					"prometheus": "test-prometheus",
				}
				err := c.List(context.Background(), pods, selector, &client.ListOptions{Namespace: "default"})
				Expect(err).To(BeNil())
				Expect(len(pods.Items)).To(Equal(1))

				pod := pods.Items[0].Name
				port := intstr.IntOrString{IntVal: prometheusPort}
				cancelFn, err := utils.StartPortForward(context.Background(), port, "https", pod, "default")
				Expect(err).To(BeNil())
				defer cancelFn()

				Eventually(func() error {
					promResp, err := utils.QueryPrometheus("up{service=\"" + queryName + "\"}")
					if err != nil {
						return err
					}
					if len(promResp.Data.Result) > 0 && promResp.Data.Result[0].Metric["service"] != queryName {
						return fmt.Errorf("query service is not up in Prometheus")
					}
					return nil
				}, time.Minute*1, time.Second*5).Should(Succeed())
			})
		})
	})

	Describe("Thanos Ruler", Ordered, func() {
		Context("When ThanosRuler is created", func() {
			It("should bring up the rulers components", func() {
				cr := &v1alpha1.ThanosRuler{
					ObjectMeta: metav1.ObjectMeta{
						Name:      rulerName,
						Namespace: namespace,
						Labels: map[string]string{
							manifests.DefaultStoreAPILabel: manifests.DefaultStoreAPIValue,
						},
					},
					Spec: v1alpha1.ThanosRulerSpec{
						QueryLabelSelector: &metav1.LabelSelector{
							MatchLabels: map[string]string{
								manifests.DefaultQueryAPILabel: manifests.DefaultQueryAPIValue,
							},
						},
						CommonThanosFields: v1alpha1.CommonThanosFields{},
						StorageSize:        "100Mi",
						ObjectStorageConfig: v1alpha1.ObjectStorageConfig{
							LocalObjectReference: corev1.LocalObjectReference{
								Name: objStoreSecret,
							},
							Key: objStoreSecretKey,
						},
						AlertmanagerURL: "http://alertmanager.com:9093",
					},
				}
				err := c.Create(context.Background(), cr, &client.CreateOptions{})
				Expect(err).To(BeNil())

				statefulSetName := controller.RulerNameFromParent(rulerName)
				Eventually(func() bool {
					return utils.VerifyStatefulSetReplicasRunning(c, 1, statefulSetName, namespace)
				}, time.Minute*5, time.Second*10).Should(BeTrue())

				svcName := controller.QueryNameFromParent(queryName)
				Eventually(func() bool {
					return utils.VerifyStatefulSetArgs(c,
						statefulSetName,
						namespace,
						0,
						fmt.Sprintf("--query=dnssrv+_http._tcp.%s.thanos-operator-system.svc.cluster.local", svcName),
					)
				}, time.Minute*1, time.Second*10).Should(BeTrue())

				cfgmap := &corev1.ConfigMap{
					ObjectMeta: metav1.ObjectMeta{
						Name:      "my-rules",
						Namespace: namespace,
						Labels: map[string]string{
							manifests.DefaultRuleConfigLabel: manifests.DefaultRuleConfigValue,
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
`,
					},
				}

				err = c.Create(context.Background(), cfgmap, &client.CreateOptions{})
				Expect(err).To(BeNil())

				Eventually(func() bool {
					return utils.VerifyStatefulSetArgs(c,
						statefulSetName,
						namespace,
						0,
						"--rule-file=/etc/thanos/rules/my-rules.yaml",
					)
				}, time.Minute*1, time.Second*10).Should(BeTrue())
			})
		})
	})

	Describe("Thanos Store", Ordered, func() {
		Context("When ThanosStore is created", func() {
			It("should bring up the store components", func() {
				cr := &v1alpha1.ThanosStore{
					ObjectMeta: metav1.ObjectMeta{
						Name:      storeName,
						Namespace: namespace,
					},
					Spec: v1alpha1.ThanosStoreSpec{
						CommonThanosFields: v1alpha1.CommonThanosFields{},
						Labels:             map[string]string{"some-label": "xyz"},
						ShardingStrategy: v1alpha1.ShardingStrategy{
							Type:          v1alpha1.Block,
							Shards:        2,
							ShardReplicas: 2,
						},
						StorageSize: "100Mi",
						ObjectStorageConfig: v1alpha1.ObjectStorageConfig{
							LocalObjectReference: corev1.LocalObjectReference{
								Name: objStoreSecret,
							},
							Key: objStoreSecretKey,
						},
					},
				}

				err := c.Create(context.Background(), cr, &client.CreateOptions{})
				Expect(err).To(BeNil())
				firstShard := controller.StoreShardName(storeName, 0)
				Eventually(func() bool {
					return utils.VerifyStatefulSetReplicasRunning(c, 2, firstShard, namespace)
				}, time.Minute*5, time.Second*10).Should(BeTrue())

				Eventually(func() bool {
					expect := `--selector.relabel-config=
- action: hashmod
  source_labels: ["__block_id"]
  target_label: shard
  modulus: 2
- action: keep
  source_labels: ["shard"]
  regex: 0`

					return utils.VerifyStatefulSetArgs(c, firstShard, namespace, 0, expect)
				}, time.Minute*5, time.Second*10).Should(BeTrue())
			})
			It("should create a ConfigMap with the correct cache configuration", func() {
				Eventually(func() bool {
					return utils.VerifyConfigMapContents(c, "thanos-store-inmemory-config", namespace, "config.yaml", store.InMemoryConfig)
				}, time.Minute*5, time.Second*10).Should(BeTrue())
			})
		})
	})

	Describe("Thanos Compact", Ordered, func() {
		Context("When ThanosCompact is created", func() {
			It("should bring up the compact components", func() {
				cr := &v1alpha1.ThanosCompact{
					ObjectMeta: metav1.ObjectMeta{
						Name:      compactName,
						Namespace: namespace,
					},
					Spec: v1alpha1.ThanosCompactSpec{
						CommonThanosFields: v1alpha1.CommonThanosFields{},
						StorageSize:        "100Mi",
						ObjectStorageConfig: v1alpha1.ObjectStorageConfig{
							LocalObjectReference: corev1.LocalObjectReference{
								Name: objStoreSecret,
							},
							Key: objStoreSecretKey,
						},
					},
				}

				err := c.Create(context.Background(), cr, &client.CreateOptions{})
				Expect(err).To(BeNil())

				stsName := controller.CompactNameFromParent(compactName)
				Eventually(func() bool {
					return utils.VerifyStatefulSetReplicasRunning(c, 1, stsName, namespace)
				}, time.Minute*5, time.Second*10).Should(BeTrue())
			})
		})
	})
})
