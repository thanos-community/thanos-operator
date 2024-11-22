package sync

import (
	"context"
	"os"

	"github.com/go-logr/logr"
	"github.com/thanos-community/thanos-operator/internal/controller"
	controllermetrics "github.com/thanos-community/thanos-operator/internal/pkg/metrics"
	"github.com/thanos-community/thanos-operator/internal/pkg/receive"

	corev1 "k8s.io/api/core/v1"
	apierrors "k8s.io/apimachinery/pkg/api/errors"
	"k8s.io/apimachinery/pkg/runtime"
	ctrl "sigs.k8s.io/controller-runtime"
	"sigs.k8s.io/controller-runtime/pkg/builder"
	"sigs.k8s.io/controller-runtime/pkg/client"
	"sigs.k8s.io/controller-runtime/pkg/predicate"
)

// Controller watches for ConfigMaps and syncs their contents to disk.
// This controller is intended to sit as a sidecar to a workload that requires near real-time updates to a file on disk.
type Controller struct {
	client.Client
	Scheme *runtime.Scheme

	logger  logr.Logger
	metrics controllermetrics.ConfigMapSyncerMetrics
	conf    ConfigMapOptions
}

type ConfigMapSyncerOptions struct {
	ConfigMapOptions      ConfigMapOptions
	InstrumentationConfig controller.InstrumentationConfig
}

// ConfigMapOptions defines the configuration for a Controller.
type ConfigMapOptions struct {
	// Name of the ConfigMap to watch.
	Name string
	// Key of the ConfigMap to watch.
	Key string
	// Path to write the ConfigMap contents to.
	Path string
}

// NewController returns a Controller for a single ConfigMap and key.
func NewController(conf ConfigMapSyncerOptions, client client.Client, scheme *runtime.Scheme) *Controller {
	return &Controller{
		Client:  client,
		Scheme:  scheme,
		conf:    conf.ConfigMapOptions,
		logger:  conf.InstrumentationConfig.Logger,
		metrics: controllermetrics.NewConfigMapSyncerMetrics(conf.InstrumentationConfig.MetricsRegistry),
	}
}

// +kubebuilder:rbac:groups="",resources=configmaps,verbs=get;list;watch

// Reconcile reads that state of the cluster for a ConfigMap object and syncs it to disk.
func (r *Controller) Reconcile(ctx context.Context, req ctrl.Request) (ctrl.Result, error) {
	if r.conf.Name != req.NamespacedName.Name {
		r.logger.Info("ignoring ConfigMap", "name", req.NamespacedName.Name)
		return ctrl.Result{}, nil
	}

	var cm corev1.ConfigMap
	if err := r.Get(ctx, req.NamespacedName, &cm); err != nil {
		if apierrors.IsNotFound(err) {
			// we'll ignore not-found errors, since they can't be fixed by an immediate requeue
			return ctrl.Result{}, nil
		}
		r.logger.Error(err, "unable to fetch ConfigMap")
		return ctrl.Result{}, err
	}

	if cm.Data == nil || cm.Data[r.conf.Key] == "" {
		r.logger.Info("ConfigMap has no data or missing key, skipping")
		return ctrl.Result{}, nil
	}

	data := []byte(cm.Data[r.conf.Key])
	if err := os.WriteFile(r.conf.Path, data, 0644); err != nil {
		r.logger.Error(err, "failed to write file")
		return ctrl.Result{}, err
	}

	r.metrics.LastWriteSuccessTime.WithLabelValues(cm.GetName(), cm.GetNamespace()).SetToCurrentTime()
	r.metrics.ConfigMapHash.WithLabelValues(cm.GetName(), cm.GetNamespace()).Set(receive.HashAsMetricValue(data))
	return ctrl.Result{}, nil
}

// SetupWithManager sets up the controller with the Manager.
func (r *Controller) SetupWithManager(mgr ctrl.Manager) error {
	return ctrl.NewControllerManagedBy(mgr).
		For(&corev1.ConfigMap{}, builder.WithPredicates(
			predicate.GenerationChangedPredicate{},
		)).
		Complete(r)
}
