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

package main

import (
	"context"
	"crypto/tls"
	"flag"
	"fmt"
	"net/http"
	"net/http/pprof"
	"net/url"
	"os"
	"strings"
	"time"

	"github.com/go-logr/logr"
	monitoringv1 "github.com/prometheus-operator/prometheus-operator/pkg/apis/monitoring/v1"
	"github.com/prometheus/client_golang/prometheus"
	versioncollector "github.com/prometheus/client_golang/prometheus/collectors/version"
	"github.com/prometheus/common/promslog"
	psflag "github.com/prometheus/common/promslog/flag"
	"github.com/prometheus/common/version"
	clientgometrics "k8s.io/client-go/tools/metrics"

	monitoringthanosiov1alpha1 "github.com/thanos-community/thanos-operator/api/v1alpha1"
	"github.com/thanos-community/thanos-operator/internal/controller"
	"github.com/thanos-community/thanos-operator/internal/pkg/featuregate"
	"github.com/thanos-community/thanos-operator/internal/pkg/manifests"
	manifestscompact "github.com/thanos-community/thanos-operator/internal/pkg/manifests/compact"
	manifestquery "github.com/thanos-community/thanos-operator/internal/pkg/manifests/query"
	manifestreceive "github.com/thanos-community/thanos-operator/internal/pkg/manifests/receive"
	manifestruler "github.com/thanos-community/thanos-operator/internal/pkg/manifests/ruler"
	manifestsstore "github.com/thanos-community/thanos-operator/internal/pkg/manifests/store"
	"github.com/thanos-community/thanos-operator/internal/pkg/metrics"

	// Import all Kubernetes client auth plugins (e.g. Azure, GCP, OIDC, etc.)
	// to ensure that exec-entrypoint and run can make use of them.
	"k8s.io/apimachinery/pkg/runtime"
	utilruntime "k8s.io/apimachinery/pkg/util/runtime"
	clientgoscheme "k8s.io/client-go/kubernetes/scheme"
	_ "k8s.io/client-go/plugin/pkg/client/auth"

	ctrl "sigs.k8s.io/controller-runtime"
	"sigs.k8s.io/controller-runtime/pkg/healthz"
	ctrlmetrics "sigs.k8s.io/controller-runtime/pkg/metrics"
	metricsserver "sigs.k8s.io/controller-runtime/pkg/metrics/server"
	"sigs.k8s.io/controller-runtime/pkg/webhook"
	//+kubebuilder:scaffold:imports
)

var (
	scheme   = runtime.NewScheme()
	setupLog = ctrl.Log.WithName("setup")
)

func init() {
	utilruntime.Must(clientgoscheme.AddToScheme(scheme))

	utilruntime.Must(monitoringthanosiov1alpha1.AddToScheme(scheme))
	//+kubebuilder:scaffold:scheme
	utilruntime.Must(monitoringv1.AddToScheme(scheme))
}

// registerClientGoMetrics registers client-go metrics adapters to expose
// REST client metrics including request duration, size, and other metrics.
func registerClientGoMetrics() {
	requestLatency := prometheus.NewHistogramVec(
		prometheus.HistogramOpts{
			Name:    "rest_client_request_duration_seconds",
			Help:    "Request latency in seconds. Broken down by verb, and host.",
			Buckets: prometheus.ExponentialBuckets(0.001, 2, 10),
		},
		[]string{"verb", "host"},
	)
	ctrlmetrics.Registry.MustRegister(requestLatency)

	requestSize := prometheus.NewHistogramVec(
		prometheus.HistogramOpts{
			Name:    "rest_client_request_size_bytes",
			Help:    "Request size in bytes. Broken down by verb and host.",
			Buckets: prometheus.ExponentialBuckets(1024, 2, 10),
		},
		[]string{"verb", "host"},
	)
	ctrlmetrics.Registry.MustRegister(requestSize)

	responseSize := prometheus.NewHistogramVec(
		prometheus.HistogramOpts{
			Name:    "rest_client_response_size_bytes",
			Help:    "Response size in bytes. Broken down by verb and host.",
			Buckets: prometheus.ExponentialBuckets(1024, 2, 10),
		},
		[]string{"verb", "host"},
	)
	ctrlmetrics.Registry.MustRegister(responseSize)

	rateLimiterLatency := prometheus.NewHistogramVec(
		prometheus.HistogramOpts{
			Name:    "rest_client_rate_limiter_duration_seconds",
			Help:    "Client side rate limiter latency in seconds. Broken down by verb, and host.",
			Buckets: prometheus.ExponentialBuckets(0.001, 2, 10),
		},
		[]string{"verb", "host"},
	)
	ctrlmetrics.Registry.MustRegister(rateLimiterLatency)

	clientgometrics.Register(
		clientgometrics.RegisterOpts{
			RequestLatency:     &latencyAdapter{metric: requestLatency},
			RequestSize:        &sizeAdapter{metric: requestSize},
			ResponseSize:       &sizeAdapter{metric: responseSize},
			RateLimiterLatency: &latencyAdapter{metric: rateLimiterLatency},
		},
	)
}

// latencyAdapter implements the clientgometrics.LatencyMetric interface
type latencyAdapter struct {
	metric *prometheus.HistogramVec
}

func (l *latencyAdapter) Observe(ctx context.Context, verb string, u url.URL, latency time.Duration) {
	l.metric.WithLabelValues(verb, u.Host).Observe(latency.Seconds())
}

// sizeAdapter implements the clientgometrics.SizeMetric interface
type sizeAdapter struct {
	metric *prometheus.HistogramVec
}

func (s *sizeAdapter) Observe(ctx context.Context, verb string, host string, size float64) {
	s.metric.WithLabelValues(verb, host).Observe(size)
}

func main() {
	var metricsAddr string
	var enableLeaderElection bool
	var probeAddr string
	var secureMetrics bool
	var enableHTTP2 bool

	var enabledFeatures featuregate.Flag

	var logLevelStr string
	var logFormatStr string

	flag.StringVar(&metricsAddr, "metrics-bind-address", ":8080", "The address the metric endpoint binds to.")
	flag.StringVar(&probeAddr, "health-probe-bind-address", ":8081", "The address the probe endpoint binds to.")
	flag.BoolVar(&enableLeaderElection, "leader-elect", false,
		"Enable leader election for controller manager. "+
			"Enabling this will ensure there is only one active controller manager.")
	flag.BoolVar(&secureMetrics, "metrics-secure", false,
		"If set the metrics endpoint is served securely")
	flag.BoolVar(&enableHTTP2, "enable-http2", false,
		"If set, HTTP/2 will be enabled for the metrics and webhook servers")
	flag.Var(&enabledFeatures, "enable-feature", fmt.Sprintf("Experimental feature to enable. Repeat for multiple features. Available features: %s.", strings.Join(featuregate.AllFeatures(), ", ")))
	flag.StringVar(&logLevelStr, "log.level", "info", psflag.LevelFlagHelp)
	flag.StringVar(&logFormatStr, "log.format", "logfmt", psflag.FormatFlagHelp)
	flag.Parse()

	logLevel := promslog.NewLevel()
	if err := logLevel.Set(logLevelStr); err != nil {
		setupLog.Error(err, "invalid log level")
		os.Exit(1)
	}

	logFormat := promslog.NewFormat()
	if err := logFormat.Set(logFormatStr); err != nil {
		setupLog.Error(err, "invalid log format")
		os.Exit(1)
	}

	ctrl.SetLogger(
		logr.FromSlogHandler(
			promslog.New(&promslog.Config{
				Level:  logLevel,
				Format: logFormat,
				Style:  promslog.GoKitStyle,
				Writer: os.Stderr,
			}).Handler(),
		),
	)
	setupLog := ctrl.Log.WithName("setup")
	// if the enable-http2 flag is false (the default), http/2 should be disabled
	// due to its vulnerabilities. More specifically, disabling http/2 will
	// prevent from being vulnerable to the HTTP/2 Stream Cancelation and
	// Rapid Reset CVEs. For more information see:
	// - https://github.com/advisories/GHSA-qppj-fm5r-hxr3
	// - https://github.com/advisories/GHSA-4374-p667-p6c8
	disableHTTP2 := func(c *tls.Config) {
		setupLog.Info("disabling http/2")
		c.NextProtos = []string{"http/1.1"}
	}

	tlsOpts := []func(*tls.Config){}
	if !enableHTTP2 {
		tlsOpts = append(tlsOpts, disableHTTP2)
	}

	webhookServer := webhook.NewServer(webhook.Options{
		TLSOpts: tlsOpts,
	})

	mgr, err := ctrl.NewManager(ctrl.GetConfigOrDie(), ctrl.Options{
		Scheme: scheme,
		Metrics: metricsserver.Options{
			BindAddress:   metricsAddr,
			SecureServing: secureMetrics,
			TLSOpts:       tlsOpts,
			ExtraHandlers: map[string]http.Handler{
				"/debug/pprof/":        http.HandlerFunc(pprof.Index),
				"/debug/pprof/cmdline": http.HandlerFunc(pprof.Cmdline),
				"/debug/pprof/profile": http.HandlerFunc(pprof.Profile),
				"/debug/pprof/symbol":  http.HandlerFunc(pprof.Symbol),
				"/debug/pprof/trace":   http.HandlerFunc(pprof.Trace),
			},
		},
		WebhookServer:          webhookServer,
		HealthProbeBindAddress: probeAddr,
		LeaderElection:         enableLeaderElection,
		LeaderElectionID:       "92ee6155.monitoring.thanos.io",
		// LeaderElectionReleaseOnCancel defines if the leader should step down voluntarily
		// when the Manager ends. This requires the binary to immediately end when the
		// Manager is stopped, otherwise, this setting is unsafe. Setting this significantly
		// speeds up voluntary leader transitions as the new leader don't have to wait
		// LeaseDuration time first.
		//
		// In the default scaffold provided, the program ends immediately after
		// the manager stops, so would be fine to enable this option. However,
		// if you are doing or is intended to do any operation such as perform cleanups
		// after the manager stops then its usage might be unsafe.
		// LeaderElectionReleaseOnCancel: true,
	})
	if err != nil {
		setupLog.Error(err, "unable to start manager")
		os.Exit(1)
	}

	ctrlmetrics.Registry.MustRegister(
		versioncollector.NewCollector("thanos-operator"),
	)

	// Register client-go REST client metrics adapters
	// This ensures all REST client metrics (duration, request size, response size) are exposed
	registerClientGoMetrics()

	setupLog.Info("starting thanos operator", "build_info", version.Info(), "build_context", version.BuildContext())

	prometheus.DefaultRegisterer = ctrlmetrics.Registry
	baseLogger := ctrl.Log.WithName(manifests.DefaultManagedByLabel)

	const (
		defaultKubeResourceSyncImage = "quay.io/philipgough/kube-resource-sync:0.1.0"
	)

	commonMetrics := metrics.NewCommonMetrics(ctrlmetrics.Registry)
	featureGateConfig := enabledFeatures.ToFeatureGate()
	if featureGateConfig.ServiceMonitorEnabled() {
		commonMetrics.FeatureGatesInfo.WithLabelValues(featuregate.ServiceMonitor).Set(1)
	}
	if featureGateConfig.PrometheusRuleEnabled() {
		commonMetrics.FeatureGatesInfo.WithLabelValues(featuregate.PrometheusRule).Set(1)
	}
	if featureGateConfig.KubeResourceSyncEnabled() {
		featureGateConfig.KubeResourceSyncImage = defaultKubeResourceSyncImage
		if image, ok := os.LookupEnv("KUBE_RESOURCE_SYNC_IMAGE"); ok {
			featureGateConfig.KubeResourceSyncImage = image
		}
	}

	buildConfig := func(component string) controller.Config {
		return controller.Config{
			FeatureGate: featureGateConfig,
			InstrumentationConfig: controller.InstrumentationConfig{
				Logger:          baseLogger.WithName(component),
				EventRecorder:   mgr.GetEventRecorder(fmt.Sprintf("%s-controller", component)),
				MetricsRegistry: ctrlmetrics.Registry,
				CommonMetrics:   commonMetrics,
			},
		}
	}

	if err = controller.NewThanosQueryReconciler(
		buildConfig(manifestquery.Name),
		mgr.GetClient(),
		mgr.GetScheme(),
	).SetupWithManager(mgr); err != nil {
		setupLog.Error(err, "unable to create controller", "controller", "ThanosQuery")
		os.Exit(1)
	}

	if err = controller.NewThanosReceiveReconciler(
		buildConfig(manifestreceive.Name),
		mgr.GetClient(),
		mgr.GetScheme(),
	).SetupWithManager(mgr); err != nil {
		setupLog.Error(err, "unable to create controller", "controller", "ThanosReceive")
		os.Exit(1)
	}

	if err = controller.NewThanosStoreReconciler(
		buildConfig(manifestsstore.Name),
		mgr.GetClient(),
		mgr.GetScheme(),
	).SetupWithManager(mgr); err != nil {
		setupLog.Error(err, "unable to create controller", "controller", "ThanosStore")
		os.Exit(1)
	}

	if err = controller.NewThanosCompactReconciler(
		buildConfig(manifestscompact.Name),
		mgr.GetClient(),
		mgr.GetScheme(),
	).SetupWithManager(mgr); err != nil {
		setupLog.Error(err, "unable to create controller", "controller", "ThanosCompact")
		os.Exit(1)
	}

	if err = controller.NewThanosRulerReconciler(
		buildConfig(manifestruler.Name),
		mgr.GetClient(),
		mgr.GetScheme(),
	).SetupWithManager(mgr); err != nil {
		setupLog.Error(err, "unable to create controller", "controller", "ThanosRuler")
		os.Exit(1)
	}

	if err = controller.NewObjectStatusReconciler(
		buildConfig("object-status"),
		mgr.GetClient(),
		mgr.GetScheme(),
	).SetupWithManager(mgr); err != nil {
		setupLog.Error(err, "unable to create controller", "controller", "ObjectStatus")
		os.Exit(1)
	}

	//+kubebuilder:scaffold:builder

	if err := mgr.AddHealthzCheck("healthz", healthz.Ping); err != nil {
		setupLog.Error(err, "unable to set up health check")
		os.Exit(1)
	}
	if err := mgr.AddReadyzCheck("readyz", healthz.Ping); err != nil {
		setupLog.Error(err, "unable to set up ready check")
		os.Exit(1)
	}

	setupLog.Info("starting manager")
	if err := mgr.Start(ctrl.SetupSignalHandler()); err != nil {
		setupLog.Error(err, "problem running manager")
		os.Exit(1)
	}
}
