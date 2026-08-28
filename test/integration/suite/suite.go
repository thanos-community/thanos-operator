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

// Option tweaks how Setup wires the controllers.
type Option func(*options)

type options struct {
	enableConditionUpdates bool
}

// WithConditionUpdates keeps the status-condition writes enabled instead of
// disabling them. Most suites do not care about status and turn it off to avoid
// the extra writes, but the pause suite relies on Status.Paused as its signal
// that a reconcile ran and took the paused branch.
func WithConditionUpdates() Option {
	return func(o *options) { o.enableConditionUpdates = true }
}

// Setup boots an envtest control plane with all five controllers registered under
// the given feature-gate configuration, starts the manager, and returns the
// running env plus the context/cancel that stops it. Intended to be called from a
// suite's BeforeSuite; teardown is cancel() followed by env.Stop().
//
// The repo root is located by walking up from the caller's working directory to
// the go.mod, so suites can nest at any depth under test/integration/. Binary
// assets come from KUBEBUILDER_ASSETS (set by the Makefile test target).
func Setup(gates featuregate.Config, opts ...Option) (*Env, context.Context, context.CancelFunc) {
	var o options
	for _, apply := range opts {
		apply(&o)
	}
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

	receive := controller.NewThanosReceiveReconciler(
		buildConfig("receive"), env.Manager.GetClient(), env.Manager.GetScheme())
	query := controller.NewThanosQueryReconciler(
		buildConfig("query"), env.Manager.GetClient(), env.Manager.GetScheme())
	store := controller.NewThanosStoreReconciler(
		buildConfig("store"), env.Manager.GetClient(), env.Manager.GetScheme())
	ruler := controller.NewThanosRulerReconciler(
		buildConfig("ruler"), configReloaderImage, env.Manager.GetClient(), env.Manager.GetScheme())
	compact := controller.NewThanosCompactReconciler(
		buildConfig("compact"), env.Manager.GetClient(), env.Manager.GetScheme())

	// Status conditions are noise for most suites, so default to off. The pause
	// suite opts back in because Status.Paused is how it observes that a paused
	// reconcile actually ran.
	if !o.enableConditionUpdates {
		receive = receive.DisableConditionUpdate()
		query = query.DisableConditionUpdate()
		store = store.DisableConditionUpdate()
		ruler = ruler.DisableConditionUpdate()
		compact = compact.DisableConditionUpdate()
	}

	gomega.Expect(receive.SetupWithManager(env.Manager)).To(gomega.Succeed())
	gomega.Expect(query.SetupWithManager(env.Manager)).To(gomega.Succeed())
	gomega.Expect(store.SetupWithManager(env.Manager)).To(gomega.Succeed())
	gomega.Expect(ruler.SetupWithManager(env.Manager)).To(gomega.Succeed())
	gomega.Expect(compact.SetupWithManager(env.Manager)).To(gomega.Succeed())

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
