package main

import (
	"errors"
	"flag"
	corev1 "k8s.io/api/core/v1"
	"k8s.io/apimachinery/pkg/fields"
	"os"
	"sigs.k8s.io/controller-runtime/pkg/cache"
	"sigs.k8s.io/controller-runtime/pkg/client"

	"github.com/prometheus/client_golang/prometheus"
	versioncollector "github.com/prometheus/client_golang/prometheus/collectors/version"
	"github.com/thanos-community/thanos-operator/internal/controller"

	"k8s.io/apimachinery/pkg/runtime"
	utilruntime "k8s.io/apimachinery/pkg/util/runtime"
	clientgoscheme "k8s.io/client-go/kubernetes/scheme"

	ctrl "sigs.k8s.io/controller-runtime"
	"sigs.k8s.io/controller-runtime/pkg/healthz"
	ctrlmetrics "sigs.k8s.io/controller-runtime/pkg/metrics"
	metricsserver "sigs.k8s.io/controller-runtime/pkg/metrics/server"

	"sigs.k8s.io/controller-runtime/pkg/log/zap"
)

var (
	scheme   = runtime.NewScheme()
	setupLog = ctrl.Log.WithName("setup")
)

func init() {
	utilruntime.Must(clientgoscheme.AddToScheme(scheme))
	// +kubebuilder:scaffold:scheme
}

var (
	metricsAddr string
	probeAddr   string

	configMapName string
	configMapKey  string
	pathToWrite   string
)

func main() {
	flag.StringVar(&metricsAddr, "metrics-bind-address", ":8080", "The address the metric endpoint binds to.")
	flag.StringVar(&probeAddr, "health-probe-bind-address", ":8081", "The address the probe endpoint binds to.")
	flag.StringVar(&configMapName, "name", "", "The name of the ConfigMap to watch")
	flag.StringVar(&configMapKey, "key", "", "The ConfigMap key to read")
	flag.StringVar(&pathToWrite, "path", "/", "The path to write to on disk")

	opts := zap.Options{
		Development: true,
	}
	opts.BindFlags(flag.CommandLine)
	flag.Parse()
	ctrl.SetLogger(zap.New(zap.UseFlagOptions(&opts)))

	if configMapName == "" || configMapKey == "" {
		setupLog.Error(errors.New("name and key of the ConfigMap are required"), "could not create manager")
		os.Exit(1)
	}

	mgr, err := ctrl.NewManager(ctrl.GetConfigOrDie(), ctrl.Options{
		Scheme:                 scheme,
		HealthProbeBindAddress: probeAddr,
		Metrics:                metricsserver.Options{BindAddress: metricsAddr},
		Cache: cache.Options{
			ByObject: map[client.Object]cache.ByObject{
				&corev1.ConfigMap{}: {
					Field: fields.OneTermEqualSelector("metadata.name", configMapName),
				},
			},
		},
	})
	if err != nil {
		setupLog.Error(err, "could not create manager")
		os.Exit(1)
	}
	logger := ctrl.Log.WithName("configmap-syncer")

	ctrlmetrics.Registry.MustRegister(
		versioncollector.NewCollector("configmap-sync"),
	)
	prometheus.DefaultRegisterer = ctrlmetrics.Registry

	if err = controller.NewConfigMapToDiskReconciler(
		logger,
		mgr.GetClient(),
		mgr.GetScheme(),
		ctrlmetrics.Registry,
		controller.ConfigMapOptions{
			Name: configMapName,
			Key:  configMapKey,
			Path: pathToWrite,
		},
	).SetupWithManager(mgr); err != nil {
		setupLog.Error(err, "unable to create controller", "controller", "configmap-syncer")
		os.Exit(1)
	}

	if err := mgr.AddHealthzCheck("health", healthz.Ping); err != nil {
		setupLog.Error(err, "unable to set up health check")
		os.Exit(1)
	}
	if err := mgr.AddReadyzCheck("check", healthz.Ping); err != nil {
		setupLog.Error(err, "unable to set up ready check")
		os.Exit(1)
	}

	setupLog.Info("starting manager")
	if err := mgr.Start(ctrl.SetupSignalHandler()); err != nil {
		setupLog.Error(err, "could not start manager")
		os.Exit(1)
	}
}
