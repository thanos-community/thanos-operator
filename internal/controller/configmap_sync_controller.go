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
	"crypto/md5"
	"encoding/binary"
	"os"

	"github.com/go-logr/logr"
	"github.com/prometheus/client_golang/prometheus"
	"github.com/prometheus/client_golang/prometheus/promauto"

	corev1 "k8s.io/api/core/v1"
	apierrors "k8s.io/apimachinery/pkg/api/errors"
	"k8s.io/apimachinery/pkg/runtime"

	ctrl "sigs.k8s.io/controller-runtime"
	"sigs.k8s.io/controller-runtime/pkg/builder"
	"sigs.k8s.io/controller-runtime/pkg/client"
	"sigs.k8s.io/controller-runtime/pkg/predicate"
)

// ConfigMapToDiskReconciler watches for ConfigMaps and syncs their contents to disk.
// This controller is intended to sit as a sidecar to a workload that requires near real-time updates to a file on disk.
type ConfigMapToDiskReconciler struct {
	client.Client
	Scheme *runtime.Scheme

	logger logr.Logger

	conf ConfigMapOptions

	reg                           prometheus.Registerer
	reconciliationsTotal          prometheus.Counter
	reconciliationsFailedTotal    prometheus.Counter
	configMapLastWriteSuccessTime *prometheus.GaugeVec
	configMapHash                 *prometheus.GaugeVec
	clientErrorsTotal             prometheus.Counter
}

// ConfigMapOptions defines the configuration for a ConfigMapToDiskReconciler.
type ConfigMapOptions struct {
	// Name of the ConfigMap to watch.
	Name string
	// Key of the ConfigMap to watch.
	Key string
	// Path to write the ConfigMap contents to.
	Path string
}

// NewConfigMapToDiskReconciler returns a reconciler for a single ConfigMap and key.
func NewConfigMapToDiskReconciler(logger logr.Logger, client client.Client, scheme *runtime.Scheme, reg prometheus.Registerer, opts ConfigMapOptions) *ConfigMapToDiskReconciler {
	return &ConfigMapToDiskReconciler{
		Client: client,
		Scheme: scheme,
		conf:   opts,
		logger: logger,
		reg:    reg,
		reconciliationsTotal: promauto.With(reg).NewCounter(prometheus.CounterOpts{
			Name: "thanos_configmap_syncer_reconciliations_total",
			Help: "Total number of reconciliations for ThanosReceive resources",
		}),
		reconciliationsFailedTotal: promauto.With(reg).NewCounter(prometheus.CounterOpts{
			Name: "thanos_configmap_syncer_reconciliations_failed_total",
			Help: "Total number of failed reconciliations for ThanosReceive resources",
		}),
		configMapLastWriteSuccessTime: promauto.With(reg).NewGaugeVec(prometheus.GaugeOpts{
			Name: "thanos_configmap_syncer_last_write_success_timestamp_seconds",
			Help: "Unix timestamp of the last successful write to disk",
		}, []string{"resource", "namespace"}),
		configMapHash: promauto.With(reg).NewGaugeVec(prometheus.GaugeOpts{
			Name: "thanos_configmap_syncer_configmap_hash",
			Help: "Hash of the last created or updated configmap.",
		}, []string{"resource", "namespace"}),
	}
}

// +kubebuilder:rbac:groups="",resources=configmaps,verbs=get;list;watch

// Reconcile reads that state of the cluster for a ConfigMap object and syncs it to disk.
func (r *ConfigMapToDiskReconciler) Reconcile(ctx context.Context, req ctrl.Request) (ctrl.Result, error) {
	r.reconciliationsTotal.Inc()

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
		r.reconciliationsFailedTotal.Inc()
		return ctrl.Result{}, err
	}

	if cm.Data == nil || cm.Data[r.conf.Key] == "" {
		r.logger.Info("ConfigMap has no data or missing key, skipping")
		return ctrl.Result{}, nil
	}

	data := []byte(cm.Data[r.conf.Key])
	if err := os.WriteFile(r.conf.Path, data, 0644); err != nil {
		r.logger.Error(err, "failed to write file")
		r.reconciliationsFailedTotal.Inc()
		return ctrl.Result{}, err
	}

	r.configMapLastWriteSuccessTime.WithLabelValues(req.Name, req.Namespace).SetToCurrentTime()
	r.configMapHash.WithLabelValues(req.Name, req.Namespace).Set(hashAsMetricValue(data))

	return ctrl.Result{}, nil
}

// SetupWithManager sets up the controller with the Manager.
func (r *ConfigMapToDiskReconciler) SetupWithManager(mgr ctrl.Manager) error {
	return ctrl.NewControllerManagedBy(mgr).
		For(&corev1.ConfigMap{}, builder.WithPredicates(
			predicate.GenerationChangedPredicate{},
		)).
		Complete(r)
}

// hashAsMetricValue generates metric value from hash of data.
func hashAsMetricValue(data []byte) float64 {
	sum := md5.Sum(data)
	// We only want 48 bits as a float64 only has a 53 bit mantissa.
	smallSum := sum[0:6]
	bytes := make([]byte, 8)
	copy(bytes, smallSum)

	return float64(binary.LittleEndian.Uint64(bytes))
}
