package suite

import (
	"context"
	"time"

	"github.com/onsi/ginkgo/v2"
	"github.com/onsi/gomega"

	monitoringv1 "github.com/prometheus-operator/prometheus-operator/pkg/apis/monitoring/v1"
	"github.com/prometheus/client_golang/prometheus"

	monitoringthanosiov1alpha1 "github.com/thanos-community/thanos-operator/api/v1alpha1"

	"k8s.io/apimachinery/pkg/runtime"
	"k8s.io/client-go/kubernetes/scheme"
	"k8s.io/client-go/rest"

	ctrl "sigs.k8s.io/controller-runtime"
	"sigs.k8s.io/controller-runtime/pkg/client"
	"sigs.k8s.io/controller-runtime/pkg/envtest"
	metricsserver "sigs.k8s.io/controller-runtime/pkg/metrics/server"
)

// Env holds a running envtest control plane along with the manager and client
// wired to it. Callers register reconcilers on Manager, then call StartManager.
type Env struct {
	Cfg      *rest.Config
	Scheme   *runtime.Scheme
	Manager  ctrl.Manager
	Client   client.Client
	Registry *prometheus.Registry

	testEnv *envtest.Environment
}

// Start boots an envtest control plane loading the given CRD directory paths
// (relative to the calling test package's directory), registers the operator and
// prometheus-operator schemes, builds a manager and a direct client, and applies
// the shared Gomega polling defaults. binaryAssetsDir may be empty, in which case
// envtest falls back to KUBEBUILDER_ASSETS or its default lookup.
func Start(binaryAssetsDir string, crdPaths ...string) (*Env, error) {
	// Keep timeouts generous (CI reconciles can lag) but poll often: the
	// reconcilers are watch-driven with no requeue on these paths, so readiness
	// lands well under a second and a tight interval just removes dead air.
	gomega.SetDefaultEventuallyTimeout(time.Minute)
	gomega.SetDefaultEventuallyPollingInterval(time.Millisecond * 200)
	gomega.SetDefaultConsistentlyDuration(time.Second * 10)
	gomega.SetDefaultConsistentlyPollingInterval(time.Millisecond * 200)

	te := &envtest.Environment{
		CRDDirectoryPaths:     crdPaths,
		ErrorIfCRDPathMissing: true,
	}
	if binaryAssetsDir != "" {
		te.BinaryAssetsDirectory = binaryAssetsDir
	}

	cfg, err := te.Start()
	if err != nil {
		return nil, err
	}

	if err := monitoringthanosiov1alpha1.AddToScheme(scheme.Scheme); err != nil {
		return nil, err
	}
	if err := monitoringv1.AddToScheme(scheme.Scheme); err != nil {
		return nil, err
	}

	// Disable the manager's metrics listener. Suites read metrics off the
	// injected registry, not the HTTP endpoint, and a fixed bind address would
	// collide when several envtest binaries run concurrently under `go test`.
	mgr, err := ctrl.NewManager(cfg, ctrl.Options{
		Scheme:  scheme.Scheme,
		Metrics: metricsserver.Options{BindAddress: "0"},
	})
	if err != nil {
		return nil, err
	}

	cl, err := client.New(cfg, client.Options{Scheme: scheme.Scheme})
	if err != nil {
		return nil, err
	}

	return &Env{
		Cfg:     cfg,
		Scheme:  scheme.Scheme,
		Manager: mgr,
		Client:  cl,
		// A fresh registry per env avoids double-registration panics when more
		// than one suite bootstraps in the same process.
		Registry: prometheus.NewRegistry(),
		testEnv:  te,
	}, nil
}

// StartManager runs the manager in a background goroutine until ctx is cancelled.
func (e *Env) StartManager(ctx context.Context) {
	go func() {
		defer ginkgo.GinkgoRecover()
		gomega.Expect(e.Manager.Start(ctx)).To(gomega.Succeed())
	}()
}

// Stop tears down the envtest control plane.
func (e *Env) Stop() error {
	return e.testEnv.Stop()
}
