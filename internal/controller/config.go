package controller

import (
	"github.com/go-logr/logr"
	"github.com/prometheus/client_golang/prometheus"
	"github.com/thanos-community/thanos-operator/internal/pkg/metrics"

	"k8s.io/client-go/tools/record"
)

// InstrumentationConfig contains the common instrumentation configuration for all controllers.
type InstrumentationConfig struct {
	Logger        logr.Logger
	EventRecorder record.EventRecorder

	MetricsRegistry prometheus.Registerer
	BaseMetrics     *metrics.BaseMetrics
}
