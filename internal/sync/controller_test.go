/*
Copyright 2024 The Kubernetes authors.

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
package sync

import (
	"context"
	"fmt"
	"log"
	"os"
	"path/filepath"
	"runtime"
	"strings"
	"testing"
	"time"

	. "github.com/onsi/ginkgo/v2"
	. "github.com/onsi/gomega"

	"github.com/prometheus/client_golang/prometheus"
	"github.com/thanos-community/thanos-operator/internal/controller"
	controllermetrics "github.com/thanos-community/thanos-operator/internal/pkg/metrics"

	corev1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/client-go/kubernetes/scheme"
	"k8s.io/client-go/rest"
	"k8s.io/client-go/tools/record"

	ctrl "sigs.k8s.io/controller-runtime"
	"sigs.k8s.io/controller-runtime/pkg/client"
	"sigs.k8s.io/controller-runtime/pkg/envtest"
	logf "sigs.k8s.io/controller-runtime/pkg/log"
	"sigs.k8s.io/controller-runtime/pkg/log/zap"
	ctrlmetrics "sigs.k8s.io/controller-runtime/pkg/metrics"
	"sigs.k8s.io/controller-runtime/pkg/metrics/server"
	// +kubebuilder:scaffold:imports
)

const (
	resourceName = "test-resource"
	ns           = "configmap-sync"

	key      = "test-key"
	value    = "test-value"
	filePath = "/test.txt"
)

var (
	cfg       *rest.Config
	k8sClient client.Client // You'll be using this client in your tests.
	testEnv   *envtest.Environment

	ctx    context.Context
	cancel context.CancelFunc

	dir string
)

func TestController(t *testing.T) {
	RegisterFailHandler(Fail)

	RunSpecs(t, "Controller Suite")
}

var _ = BeforeSuite(func() {
	logf.SetLogger(zap.New(zap.WriteTo(GinkgoWriter), zap.UseDevMode(true)))

	ctx, cancel = context.WithCancel(context.TODO())
	By("bootstrapping test environment")
	testEnv = &envtest.Environment{
		BinaryAssetsDirectory: filepath.Join("..", "..", "bin", "k8s",
			fmt.Sprintf("1.31.0-%s-%s", runtime.GOOS, runtime.GOARCH)),
	}

	var err error
	cfg, err = testEnv.Start()
	Expect(err).NotTo(HaveOccurred())
	Expect(cfg).NotTo(BeNil())
	k8sClient, err = client.New(cfg, client.Options{Scheme: scheme.Scheme})
	Expect(err).NotTo(HaveOccurred())
	Expect(k8sClient).NotTo(BeNil())

	k8sManager, err := ctrl.NewManager(cfg, ctrl.Options{
		Scheme: scheme.Scheme,
		Metrics: server.Options{
			SecureServing: false,
			BindAddress:   ":5432",
		},
	})
	Expect(err).ToNot(HaveOccurred())

	reg := prometheus.NewRegistry()
	controllerBaseMetrics := controllermetrics.NewBaseMetrics(reg)
	logger := ctrl.Log.WithName("configmap-syncer")
	ic := controller.InstrumentationConfig{
		Logger:          logger,
		EventRecorder:   record.NewFakeRecorder(100).WithLogger(logger),
		MetricsRegistry: ctrlmetrics.Registry,
		BaseMetrics:     controllerBaseMetrics,
	}

	dir, err = os.MkdirTemp("", "test")
	if err != nil {
		log.Fatal(err)
	}

	opts := ConfigMapOptions{
		Name: resourceName,
		Key:  key,
		Path: dir + filePath,
	}

	conf := ConfigMapSyncerOptions{
		ConfigMapOptions:      opts,
		InstrumentationConfig: ic,
	}

	err = NewController(conf, k8sManager.GetClient(), k8sManager.GetScheme()).SetupWithManager(k8sManager)
	Expect(err).ToNot(HaveOccurred())

	go func() {
		defer GinkgoRecover()
		err = k8sManager.Start(ctx)
		Expect(err).ToNot(HaveOccurred(), "failed to run manager")
	}()

})

var _ = AfterSuite(func() {
	By("tearing down the test environment")
	cancel()
	err := testEnv.Stop()
	Expect(err).NotTo(HaveOccurred())
	Expect(os.RemoveAll(dir)).NotTo(HaveOccurred())
})

var _ = Describe("Controller", Ordered, func() {
	Context("When reconciling a resource", func() {
		BeforeAll(func() {
			By("creating the namespace")
			Expect(k8sClient.Create(ctx, &corev1.Namespace{
				ObjectMeta: metav1.ObjectMeta{
					Name: ns,
				},
			})).Should(Succeed())
		})

		AfterEach(func() {
		})

		It("should reconcile correctly", func() {
			resource := &corev1.ConfigMap{
				ObjectMeta: metav1.ObjectMeta{
					Name:      resourceName,
					Namespace: ns,
				},
				Data: map[string]string{
					key: value,
				},
			}
			By("creating the ConfigMap and expecting the value to be written to disk", func() {
				Expect(k8sClient.Create(context.Background(), resource)).Should(Succeed())
				fmt.Println("directory: ", dir)
				EventuallyWithOffset(1, func() bool {
					b, err := os.ReadFile(dir + filePath)
					if err != nil {
						return false
					}
					return strings.TrimSpace(string(b)) == value
				}, time.Second*10, time.Second*2).Should(BeTrue())
			})

		})
	})
})
