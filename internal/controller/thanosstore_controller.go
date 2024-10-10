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
	"fmt"

	"github.com/go-logr/logr"
	monitoringv1 "github.com/prometheus-operator/prometheus-operator/pkg/apis/monitoring/v1"

	monitoringthanosiov1alpha1 "github.com/thanos-community/thanos-operator/api/v1alpha1"
	"github.com/thanos-community/thanos-operator/internal/pkg/handlers"
	"github.com/thanos-community/thanos-operator/internal/pkg/manifests"
	manifestsstore "github.com/thanos-community/thanos-operator/internal/pkg/manifests/store"
	controllermetrics "github.com/thanos-community/thanos-operator/internal/pkg/metrics"

	appsv1 "k8s.io/api/apps/v1"
	corev1 "k8s.io/api/core/v1"
	apierrors "k8s.io/apimachinery/pkg/api/errors"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/runtime"
	"k8s.io/client-go/tools/record"
	"k8s.io/utils/ptr"

	ctrl "sigs.k8s.io/controller-runtime"
	"sigs.k8s.io/controller-runtime/pkg/client"
)

// ThanosStoreReconciler reconciles a ThanosStore object
type ThanosStoreReconciler struct {
	client.Client
	Scheme *runtime.Scheme

	logger   logr.Logger
	metrics  controllermetrics.ThanosStoreMetrics
	recorder record.EventRecorder

	handler *handlers.Handler
}

// NewThanosStoreReconciler returns a reconciler for ThanosStore resources.
func NewThanosStoreReconciler(conf Config, client client.Client, scheme *runtime.Scheme) *ThanosStoreReconciler {
	handler := handlers.NewHandler(client, scheme, conf.InstrumentationConfig.Logger)
	featureGates := conf.FeatureGate.ToGVK()
	if len(featureGates) > 0 {
		handler.SetFeatureGates(featureGates)
	}

	return &ThanosStoreReconciler{
		Client:   client,
		Scheme:   scheme,
		logger:   conf.InstrumentationConfig.Logger,
		metrics:  controllermetrics.NewThanosStoreMetrics(conf.InstrumentationConfig.MetricsRegistry, conf.InstrumentationConfig.BaseMetrics),
		recorder: conf.InstrumentationConfig.EventRecorder,
		handler:  handler,
	}
}

//+kubebuilder:rbac:groups=monitoring.thanos.io,resources=thanosstores,verbs=get;list;watch;create;update;patch;delete
//+kubebuilder:rbac:groups=monitoring.thanos.io,resources=thanosstores/status,verbs=get;update;patch
//+kubebuilder:rbac:groups=monitoring.thanos.io,resources=thanosstores/finalizers,verbs=update
//+kubebuilder:rbac:groups=apps,resources=statefulsets,verbs=get;list;watch;create;update;patch;delete
//+kubebuilder:rbac:groups="",resources=services;configmaps;serviceaccounts,verbs=get;list;watch;create;update;patch;delete

// Reconcile is part of the main kubernetes reconciliation loop which aims to
// move the current state of the cluster closer to the desired state.
//
// For more details, check Reconcile and its Result here:
// - https://pkg.go.dev/sigs.k8s.io/controller-runtime@v0.17.0/pkg/reconcile
func (r *ThanosStoreReconciler) Reconcile(ctx context.Context, req ctrl.Request) (ctrl.Result, error) {
	r.metrics.ReconciliationsTotal.WithLabelValues(manifestsstore.Name).Inc()

	store := &monitoringthanosiov1alpha1.ThanosStore{}
	err := r.Get(ctx, req.NamespacedName, store)
	if err != nil {
		r.metrics.ClientErrorsTotal.WithLabelValues(manifestsstore.Name).Inc()
		if apierrors.IsNotFound(err) {
			r.logger.Info("thanos store resource not found. ignoring since object may be deleted")
			return ctrl.Result{}, nil
		}
		r.logger.Error(err, "failed to get ThanosStore")
		r.metrics.ReconciliationsFailedTotal.WithLabelValues(manifestsstore.Name).Inc()
		r.recorder.Event(store, corev1.EventTypeWarning, "GetFailed", "Failed to get ThanosStore resource")
		return ctrl.Result{}, err
	}

	if store.Spec.Paused != nil {
		if *store.Spec.Paused {
			r.logger.Info("reconciliation is paused for ThanosStore")
			r.recorder.Event(store, corev1.EventTypeNormal, "Paused", "Reconciliation is paused for ThanosStore resource")
			return ctrl.Result{}, nil
		}
	}

	err = r.syncResources(ctx, *store)
	if err != nil {
		r.metrics.ReconciliationsFailedTotal.WithLabelValues(manifestsstore.Name).Inc()
		r.recorder.Event(store, corev1.EventTypeWarning, "SyncFailed", fmt.Sprintf("Failed to sync resources: %v", err))
		return ctrl.Result{}, err
	}

	return ctrl.Result{}, nil
}

func (r *ThanosStoreReconciler) syncResources(ctx context.Context, store monitoringthanosiov1alpha1.ThanosStore) error {
	var errCount int
	opts := r.specToOptions(store)

	expectShards := make([]string, len(opts))
	for i, opt := range opts {
		if opt == nil {
			continue
		}
		expectShards[i] = opt.GetGeneratedResourceName()
		errCount += r.handler.CreateOrUpdate(ctx, store.GetNamespace(), &store, opt.Build())
	}

	if errCount > 0 {
		r.metrics.ClientErrorsTotal.WithLabelValues(manifestsstore.Name).Add(float64(errCount))
		return fmt.Errorf("failed to create or update %d resources for store or store shard(s)", errCount)
	}

	// prune the store resources that are no longer needed/have changed
	errCount = r.pruneOrphanedResources(ctx, store.GetNamespace(), store.GetName(), expectShards)
	if errCount > 0 {
		r.metrics.ClientErrorsTotal.WithLabelValues(manifestsstore.Name).Add(float64(errCount))
		return fmt.Errorf("failed to prune %d orphaned resources for store shard(s)", errCount)
	}

	if !r.hasServiceMonitorsEnabled(store) {
		objs := make([]client.Object, len(expectShards))
		for i, shard := range expectShards {
			objs[i] = &monitoringv1.ServiceMonitor{ObjectMeta: metav1.ObjectMeta{Name: shard, Namespace: store.GetNamespace()}}
		}

		if errCount = r.handler.DeleteResource(ctx, objs); errCount > 0 {
			r.metrics.ClientErrorsTotal.WithLabelValues(manifestsstore.Name).Add(float64(errCount))
			return fmt.Errorf("failed to delete %d ServiceMonitors for the store shard(s)", errCount)
		}
	}
	return nil
}

func (r *ThanosStoreReconciler) hasServiceMonitorsEnabled(store monitoringthanosiov1alpha1.ThanosStore) bool {
	return store.Spec.ServiceMonitorConfig != nil && store.Spec.ServiceMonitorConfig.Enabled != nil && *store.Spec.ServiceMonitorConfig.Enabled
}

func (r *ThanosStoreReconciler) specToOptions(store monitoringthanosiov1alpha1.ThanosStore) []manifests.Buildable {
	// no sharding strategy, or sharding strategy with 1 shard, return a single store
	if store.Spec.ShardingStrategy.Shards == 0 || store.Spec.ShardingStrategy.Shards == 1 {
		return []manifests.Buildable{storeV1Alpha1ToOptions(store)}
	}

	shardCount := int(store.Spec.ShardingStrategy.Shards)
	buildables := make([]manifests.Buildable, shardCount)
	for i := range store.Spec.ShardingStrategy.Shards {
		storeShardOpts := storeV1Alpha1ToOptions(store)
		storeShardOpts.RelabelConfigs = manifests.RelabelConfigs{
			{
				Action:      "hashmod",
				SourceLabel: "__block_id",
				TargetLabel: "shard",
				Modulus:     shardCount,
			},
			{
				Action:      "keep",
				SourceLabel: "shard",
				Regex:       fmt.Sprintf("%d", i),
			},
		}
		storeShardOpts.ShardIndex = ptr.To(i)
		buildables[i] = storeShardOpts
	}
	return buildables
}

func (r *ThanosStoreReconciler) pruneOrphanedResources(ctx context.Context, ns, owner string, expectShards []string) int {
	listOpt := manifests.GetLabelSelectorForOwner(manifestsstore.Options{Options: manifests.Options{Owner: owner}})
	listOpts := []client.ListOption{listOpt, client.InNamespace(ns)}

	pruner := r.handler.NewResourcePruner().WithServiceAccount().WithService().WithStatefulSet().WithPodDisruptionBudget().WithServiceMonitor()
	return pruner.Prune(ctx, expectShards, listOpts...)
}

// SetupWithManager sets up the controller with the Manager.
func (r *ThanosStoreReconciler) SetupWithManager(mgr ctrl.Manager) error {
	err := ctrl.NewControllerManagedBy(mgr).
		For(&monitoringthanosiov1alpha1.ThanosStore{}).
		Owns(&corev1.ConfigMap{}).
		Owns(&corev1.ServiceAccount{}).
		Owns(&corev1.Service{}).
		Owns(&appsv1.StatefulSet{}).
		Owns(&monitoringv1.ServiceMonitor{}).
		Complete(r)

	if err != nil {
		r.recorder.Event(&monitoringthanosiov1alpha1.ThanosStore{}, corev1.EventTypeWarning, "SetupFailed", fmt.Sprintf("Failed to set up controller: %v", err))
		return err
	}

	return nil
}
