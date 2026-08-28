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
	"path/filepath"
	"runtime"
	"testing"

	. "github.com/onsi/ginkgo/v2"
	. "github.com/onsi/gomega"

	"github.com/thanos-community/thanos-operator/internal/pkg/featuregate"
	"github.com/thanos-community/thanos-operator/internal/pkg/metrics"
	"github.com/thanos-community/thanos-operator/test/integration/testenv"

	"k8s.io/client-go/tools/events"

	ctrl "sigs.k8s.io/controller-runtime"
	"sigs.k8s.io/controller-runtime/pkg/client"
	logf "sigs.k8s.io/controller-runtime/pkg/log"
	"sigs.k8s.io/controller-runtime/pkg/log/zap"
	//+kubebuilder:scaffold:imports
)

// These tests use Ginkgo (BDD-style Go testing framework). Refer to
// http://onsi.github.io/ginkgo/ to learn more about Ginkgo.

var (
	k8sClient client.Client
	env       *testenv.Env
	ctx       context.Context
	cancel    context.CancelFunc
)

const (
	skipValue = "true"
)

func TestControllers(t *testing.T) {
	RegisterFailHandler(Fail)

	RunSpecs(t, "Controller Suite")
}

var _ = BeforeSuite(func() {
	logf.SetLogger(zap.New(zap.WriteTo(GinkgoWriter), zap.UseDevMode(true)))
	ctx, cancel = context.WithCancel(context.TODO())

	By("bootstrapping test environment")
	var err error
	env, err = testenv.Start(
		// The BinaryAssetsDirectory fallback is only used when running the tests
		// directly (without KUBEBUILDER_ASSETS set by the makefile target).
		filepath.Join("..", "..", "bin", "k8s",
			fmt.Sprintf("1.29.0-%s-%s", runtime.GOOS, runtime.GOARCH)),
		filepath.Join("..", "..", "config", "crd", "bases"),
		filepath.Join("..", "..", "test", "configs", "service-monitor.yaml"),
		filepath.Join("..", "..", "test", "configs", "prometheus-rule.yaml"),
	)
	Expect(err).NotTo(HaveOccurred())
	k8sClient = env.Client
	Expect(k8sClient).NotTo(BeNil())

	//+kubebuilder:scaffold:scheme

	logger := ctrl.Log.WithName("suite-test")
	buildConfig := func(component string) Config {
		return Config{
			FeatureGate: featuregate.Config{
				// Feature-gated coverage lives in the matching packages under
				// test/integration (service-monitor, prometheus-rule, ...); the
				// core suite runs with the operator's default (all gates off).
				EnableServiceMonitor:          false,
				EnablePrometheusRuleDiscovery: false,
			},
			InstrumentationConfig: InstrumentationConfig{
				Logger:          logger.WithName(component),
				EventRecorder:   events.NewFakeRecorder(100).WithLogger(logger),
				MetricsRegistry: env.Registry,
				CommonMetrics:   metrics.NewCommonMetrics(env.Registry),
			},
		}
	}

	err = NewThanosReceiveReconciler(
		buildConfig("receive"),
		env.Manager.GetClient(),
		env.Manager.GetScheme(),
	).DisableConditionUpdate().SetupWithManager(env.Manager)
	Expect(err).ToNot(HaveOccurred())

	err = NewThanosQueryReconciler(
		buildConfig("query"),
		env.Manager.GetClient(),
		env.Manager.GetScheme(),
	).DisableConditionUpdate().SetupWithManager(env.Manager)
	Expect(err).ToNot(HaveOccurred())

	err = NewThanosStoreReconciler(
		buildConfig("store"),
		env.Manager.GetClient(),
		env.Manager.GetScheme(),
	).DisableConditionUpdate().SetupWithManager(env.Manager)
	Expect(err).ToNot(HaveOccurred())

	err = NewThanosRulerReconciler(
		buildConfig("ruler"),
		"quay.io/prometheus-operator/prometheus-config-reloader:v0.89.0",
		env.Manager.GetClient(),
		env.Manager.GetScheme(),
	).DisableConditionUpdate().SetupWithManager(env.Manager)
	Expect(err).ToNot(HaveOccurred())

	err = NewThanosCompactReconciler(
		buildConfig("compact"),
		env.Manager.GetClient(),
		env.Manager.GetScheme(),
	).DisableConditionUpdate().SetupWithManager(env.Manager)
	Expect(err).ToNot(HaveOccurred())

	env.StartManager(ctx)
})

var _ = AfterSuite(func() {
	cancel()
	By("tearing down the test environment")
	Expect(env.Stop()).To(Succeed())
})
