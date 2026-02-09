package controller

import (
	"github.com/go-logr/logr"
	"github.com/prometheus/client_golang/prometheus"
	"k8s.io/client-go/tools/events"

	"github.com/thanos-community/thanos-operator/internal/pkg/featuregate"
	"github.com/thanos-community/thanos-operator/internal/pkg/metrics"
)

// Config holds the configuration for all controllers.
type Config struct {
	// FeatureGate holds information about enabled features.
	FeatureGate featuregate.Config
	// InstrumentationConfig contains the common instrumentation configuration for all controllers.
	InstrumentationConfig InstrumentationConfig
}

// InstrumentationConfig contains the common instrumentation configuration for all controllers.
type InstrumentationConfig struct {
	Logger        logr.Logger
	EventRecorder events.EventRecorder

	MetricsRegistry prometheus.Registerer

	CommonMetrics *metrics.CommonMetrics
}
