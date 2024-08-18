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

	. "github.com/onsi/ginkgo/v2"
	. "github.com/onsi/gomega"

	"github.com/thanos-community/thanos-operator/api/v1alpha1"
	"github.com/thanos-community/thanos-operator/internal/pkg/manifests"
	"github.com/thanos-community/thanos-operator/internal/pkg/manifests/receive"
	"github.com/thanos-community/thanos-operator/test/utils"

	appsv1 "k8s.io/api/apps/v1"
	corev1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/runtime"
	"k8s.io/apimachinery/pkg/util/intstr"

	"sigs.k8s.io/controller-runtime/pkg/client"
	"sigs.k8s.io/controller-runtime/pkg/client/config"
	logf "sigs.k8s.io/controller-runtime/pkg/log"
	"sigs.k8s.io/controller-runtime/pkg/log/zap"
)

const (
	namespace = "thanos-operator-system"

	objStoreSecret    = "thanos-object-storage"
	objStoreSecretKey = "thanos.yaml"

	receiveName  = "example-receive"
	storeName    = "example-store"
	hashringName = "default"

	queryName = "example-query"
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

		cl, err := client.New(config.GetConfigOrDie(), client.Options{
			Scheme: scheme,
		})
		if err != nil {
			fmt.Println("failed to create client")
			os.Exit(1)
		}
		c = cl
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

	Context("Thanos Receive", func() {
		It("should bring up the ingest components", func() {
			cr := &v1alpha1.ThanosReceive{
				ObjectMeta: metav1.ObjectMeta{
					Name:      receiveName,
					Namespace: namespace,
				},
				Spec: v1alpha1.ThanosReceiveSpec{
					CommonThanosFields: v1alpha1.CommonThanosFields{},
					Ingester: v1alpha1.IngesterSpec{
						DefaultObjectStorageConfig: v1alpha1.ObjectStorageConfig{
							LocalObjectReference: corev1.LocalObjectReference{
								Name: objStoreSecret,
							},
							Key: objStoreSecretKey,
						},
						Hashrings: []v1alpha1.IngestorHashringSpec{
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

			stsName := receive.IngesterNameFromParent(receiveName, hashringName)
			Eventually(func() bool {
				return utils.VerifyStatefulSetReplicasRunning(c, 1, stsName, namespace)
			}, time.Minute*5, time.Second*10).Should(BeTrue())
		})

		It("should create a ConfigMap with the correct hashring configuration", func() {
			//nolint:lll
			expect := `[
    {
        "hashring": "default",
        "tenant_matcher_type": "exact",
        "endpoints": [
            {
                "address": "example-receive-default-0.example-receive-default.thanos-operator-system.svc.cluster.local:10901",
                "az": ""
            }
        ]
    }
]`
			Eventually(func() bool {
				return utils.VerifyConfigMapContents(c, receiveName, namespace, receive.HashringConfigKey, expect)
			}, time.Minute*5, time.Second*10).Should(BeTrue())
		})
	})

	It("should bring up the router", func() {
		Eventually(func() bool {
			return utils.VerifyDeploymentReplicasRunning(c, 1, receiveName, namespace)
		}, time.Minute*5, time.Second*10).Should(BeTrue())
	})

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

	Context("Thanos Query", func() {
		It("should bring up the querier", func() {
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

			deploymentName := queryName
			Eventually(func() bool {
				return utils.VerifyDeploymentReplicasRunning(c, 1, deploymentName, namespace)
			}, time.Minute*1, time.Second*10).Should(BeTrue())

			Eventually(func() bool {
				return utils.VerifyDeploymentArgs(c,
					deploymentName,
					namespace,
					0,
					"--endpoint=dnssrv+_grpc._tcp.example-receive-default.thanos-operator-system.svc.cluster.local",
				)
			}, time.Minute*1, time.Second*10).Should(BeTrue())
		})
	})

	Context("Thanos Store", func() {
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
					StorageSize: "1Gi",
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

			Eventually(func() bool {
				return utils.VerifyStatefulSetReplicasRunning(c, 2, storeName+"-shard-0", namespace)
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

				return utils.VerifyStatefulSetArgs(c, storeName+"-shard-0", namespace, 0, expect)
			}, time.Minute*5, time.Second*10).Should(BeTrue())
		})

		It("should create a ConfigMap with the correct cache configuration", func() {
			expect := `type: IN-MEMORY
config:
  max_size: 512MiB
  max_item_size: 5MiB`

			Eventually(func() bool {
				return utils.VerifyConfigMapContents(c, storeName, namespace, "thanos-store-inmemory-config", expect)
			}, time.Minute*5, time.Second*10).Should(BeTrue())
		})
	})
})
