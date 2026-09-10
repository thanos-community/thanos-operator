package controller

import (
	"fmt"
	"strings"

	"github.com/go-logr/logr"
	"github.com/prometheus/client_golang/prometheus"
	"k8s.io/apimachinery/pkg/util/validation"
	"k8s.io/client-go/tools/events"
	"sigs.k8s.io/controller-runtime/pkg/cache"

	"github.com/thanos-community/thanos-operator/internal/pkg/featuregate"
	"github.com/thanos-community/thanos-operator/internal/pkg/metrics"
)

// CacheOptionsForNamespace limits namespaced watches and reads. Empty watches all namespaces.
func CacheOptionsForNamespace(namespace string) (cache.Options, error) {
	if namespace == "" {
		return cache.Options{}, nil
	}
	if errs := validation.IsDNS1123Label(namespace); len(errs) > 0 {
		return cache.Options{}, fmt.Errorf("invalid watch namespace %q: %s", namespace, strings.Join(errs, ", "))
	}
	return cache.Options{DefaultNamespaces: map[string]cache.Config{namespace: {}}}, nil
}

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
