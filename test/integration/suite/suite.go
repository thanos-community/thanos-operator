// Package suite provides the shared envtest bootstrap for the controller
// integration suites. It has two layers: the low-level Env primitive (env.go)
// boots an isolated API server, manager and client with sane Gomega defaults,
// and Setup (this file) registers all five controllers on that manager under a
// chosen feature-gate configuration.
//
// Each suite runs its own manager with its own configuration, which is why they
// live in separate packages: a feature gate is a reconciler-global setting (set
// once at construction), not something that can be varied per namespace. Running
// each configuration in its own test binary also lets the suites run in parallel
// with each other and with the core ordered suite under `go test`.
//
// The isolated behavioral suites (feature gates, PodDisruptionBudget, pause)
// cover order-independent behavior only. Cross-controller service discovery and
// watch coupling remain the concern of the core ordered suite under
// test/integration/controller.
package suite

import (
	"context"
	"os"
	"path/filepath"

	"github.com/onsi/ginkgo/v2"
	"github.com/onsi/gomega"

	"github.com/thanos-community/thanos-operator/internal/controller"
	"github.com/thanos-community/thanos-operator/internal/pkg/featuregate"
	"github.com/thanos-community/thanos-operator/internal/pkg/metrics"

	"k8s.io/client-go/tools/events"

	ctrl "sigs.k8s.io/controller-runtime"
	logf "sigs.k8s.io/controller-runtime/pkg/log"
	"sigs.k8s.io/controller-runtime/pkg/log/zap"
)

const configReloaderImage = "quay.io/prometheus-operator/prometheus-config-reloader:v0.89.0"

// Setup boots an envtest control plane with all five controllers registered under
// the given feature-gate configuration, starts the manager, and returns the
// running env plus the context/cancel that stops it. Intended to be called from a
// suite's BeforeSuite; teardown is cancel() followed by env.Stop().
//
// The repo root is located by walking up from the caller's working directory to
// the go.mod, so suites can nest at any depth under test/integration/. Binary
// assets come from KUBEBUILDER_ASSETS (set by the Makefile test target).
func Setup(gates featuregate.Config) (*Env, context.Context, context.CancelFunc) {
	logf.SetLogger(zap.New(zap.WriteTo(ginkgo.GinkgoWriter), zap.UseDevMode(true)))
	ctx, cancel := context.WithCancel(context.Background())

	root := repoRoot()
	env, err := Start(
		"",
		filepath.Join(root, "config", "crd", "bases"),
		filepath.Join(root, "test", "configs", "service-monitor.yaml"),
		filepath.Join(root, "test", "configs", "prometheus-rule.yaml"),
	)
	gomega.Expect(err).NotTo(gomega.HaveOccurred())

	logger := ctrl.Log.WithName("integration")
	buildConfig := func(component string) controller.Config {
		return controller.Config{
			FeatureGate: gates,
			InstrumentationConfig: controller.InstrumentationConfig{
				Logger:          logger.WithName(component),
				EventRecorder:   events.NewFakeRecorder(100).WithLogger(logger),
				MetricsRegistry: env.Registry,
				CommonMetrics:   metrics.NewCommonMetrics(env.Registry),
			},
		}
	}

	gomega.Expect(controller.NewThanosReceiveReconciler(
		buildConfig("receive"), env.Manager.GetClient(), env.Manager.GetScheme(),
	).DisableConditionUpdate().SetupWithManager(env.Manager)).To(gomega.Succeed())

	gomega.Expect(controller.NewThanosQueryReconciler(
		buildConfig("query"), env.Manager.GetClient(), env.Manager.GetScheme(),
	).DisableConditionUpdate().SetupWithManager(env.Manager)).To(gomega.Succeed())

	gomega.Expect(controller.NewThanosStoreReconciler(
		buildConfig("store"), env.Manager.GetClient(), env.Manager.GetScheme(),
	).DisableConditionUpdate().SetupWithManager(env.Manager)).To(gomega.Succeed())

	gomega.Expect(controller.NewThanosRulerReconciler(
		buildConfig("ruler"), configReloaderImage, env.Manager.GetClient(), env.Manager.GetScheme(),
	).DisableConditionUpdate().SetupWithManager(env.Manager)).To(gomega.Succeed())

	gomega.Expect(controller.NewThanosCompactReconciler(
		buildConfig("compact"), env.Manager.GetClient(), env.Manager.GetScheme(),
	).DisableConditionUpdate().SetupWithManager(env.Manager)).To(gomega.Succeed())

	env.StartManager(ctx)
	return env, ctx, cancel
}

// repoRoot walks up from the current working directory (the test binary runs in
// its package directory) until it finds the go.mod, so CRD paths resolve
// regardless of how deeply a suite nests under test/integration/.
func repoRoot() string {
	dir, err := os.Getwd()
	gomega.Expect(err).NotTo(gomega.HaveOccurred())
	for {
		if _, err := os.Stat(filepath.Join(dir, "go.mod")); err == nil {
			return dir
		}
		parent := filepath.Dir(dir)
		if parent == dir {
			ginkgo.Fail("could not locate repo root (go.mod) from working directory")
		}
		dir = parent
	}
}
