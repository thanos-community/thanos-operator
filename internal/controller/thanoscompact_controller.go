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
	monitoringthanosiov1alpha1 "github.com/thanos-community/thanos-operator/api/v1alpha1"
	"github.com/thanos-community/thanos-operator/internal/pkg/handlers"
	"github.com/thanos-community/thanos-operator/internal/pkg/manifests"
	manifestcompact "github.com/thanos-community/thanos-operator/internal/pkg/manifests/compact"
	manifestsstore "github.com/thanos-community/thanos-operator/internal/pkg/manifests/store"
	controllermetrics "github.com/thanos-community/thanos-operator/internal/pkg/metrics"

	"golang.org/x/exp/maps"

	v1 "k8s.io/api/apps/v1"
	corev1 "k8s.io/api/core/v1"
	apierrors "k8s.io/apimachinery/pkg/api/errors"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/runtime"
	"k8s.io/client-go/tools/record"
	"k8s.io/utils/strings/slices"

	ctrl "sigs.k8s.io/controller-runtime"
	"sigs.k8s.io/controller-runtime/pkg/client"
)

// ThanosCompactReconciler reconciles a ThanosCompact object
type ThanosCompactReconciler struct {
	client.Client
	Scheme *runtime.Scheme

	logger   logr.Logger
	metrics  controllermetrics.ThanosCompactMetrics
	recorder record.EventRecorder

	handler *handlers.Handler
}

//+kubebuilder:rbac:groups=monitoring.thanos.io,resources=thanoscompacts,verbs=get;list;watch;create;update;patch;delete
//+kubebuilder:rbac:groups=monitoring.thanos.io,resources=thanoscompacts/status,verbs=get;update;patch
//+kubebuilder:rbac:groups=monitoring.thanos.io,resources=thanoscompacts/finalizers,verbs=update

// Reconcile is part of the main kubernetes reconciliation loop which aims to
// move the current state of the cluster closer to the desired state.
// TODO(user): Modify the Reconcile function to compare the state specified by
// the ThanosCompact object against the actual cluster state, and then
// perform operations to make the cluster state reflect the state specified by
// the user.
//
// For more details, check Reconcile and its Result here:
// - https://pkg.go.dev/sigs.k8s.io/controller-runtime@v0.17.3/pkg/reconcile
func (r *ThanosCompactReconciler) Reconcile(ctx context.Context, req ctrl.Request) (ctrl.Result, error) {
	r.metrics.ReconciliationsTotal.WithLabelValues(manifestcompact.Name).Inc()

	compact := &monitoringthanosiov1alpha1.ThanosCompact{}
	err := r.Get(ctx, req.NamespacedName, compact)
	if err != nil {
		r.metrics.ClientErrorsTotal.WithLabelValues(manifestcompact.Name).Inc()
		if apierrors.IsNotFound(err) {
			r.logger.Info("thanos compact resource not found. ignoring since object may be deleted")
			return ctrl.Result{}, nil
		}
		r.logger.Error(err, "failed to get ThanosCompact")
		r.metrics.ReconciliationsFailedTotal.WithLabelValues(manifestcompact.Name).Inc()
		r.recorder.Event(compact, corev1.EventTypeWarning, "GetFailed", "Failed to get ThanosCompact resource")
		return ctrl.Result{}, err
	}

	if compact.Spec.Paused != nil && *compact.Spec.Paused {
		r.logger.Info("reconciliation is paused for ThanosCompact resource")
		r.recorder.Event(compact, corev1.EventTypeNormal, "Paused", "Reconciliation is paused for ThanosCompact resource")
		return ctrl.Result{}, nil
	}

	err = r.syncResources(ctx, *compact)
	if err != nil {
		r.metrics.ReconciliationsFailedTotal.WithLabelValues(manifestcompact.Name).Inc()
		r.recorder.Event(compact, corev1.EventTypeWarning, "SyncFailed", fmt.Sprintf("Failed to sync resources: %v", err))
		return ctrl.Result{}, err
	}

	return ctrl.Result{}, nil
}

// NewThanosCompactReconciler returns a reconciler for ThanosCompact resources.
func NewThanosCompactReconciler(instrumentationConf InstrumentationConfig, client client.Client, scheme *runtime.Scheme) *ThanosCompactReconciler {
	return &ThanosCompactReconciler{
		Client:   client,
		Scheme:   scheme,
		logger:   instrumentationConf.Logger,
		metrics:  controllermetrics.NewThanosCompactMetrics(instrumentationConf.MetricsRegistry, instrumentationConf.BaseMetrics),
		recorder: instrumentationConf.EventRecorder,
		handler:  handlers.NewHandler(client, scheme, instrumentationConf.Logger),
	}
}

// SetupWithManager sets up the controller with the Manager.
func (r *ThanosCompactReconciler) SetupWithManager(mgr ctrl.Manager) error {
	return ctrl.NewControllerManagedBy(mgr).
		For(&monitoringthanosiov1alpha1.ThanosCompact{}).
		Complete(r)
}

func (r *ThanosCompactReconciler) syncResources(ctx context.Context, compact monitoringthanosiov1alpha1.ThanosCompact) error {
	var errCount int
	shardedObjects := r.buildCompact(compact)
	instanceLabel := CompactNameFromParent(compact.GetName())
	if err := r.pruneOrphanedResources(ctx, compact.GetNamespace(), instanceLabel, maps.Keys(shardedObjects)); err != nil {
		return err
	}

	for _, shardObjs := range shardedObjects {
		errCount += r.handler.CreateOrUpdate(ctx, compact.GetNamespace(), &compact, shardObjs)
	}

	if errCount > 0 {
		r.metrics.ClientErrorsTotal.WithLabelValues(manifestsstore.Name).Add(float64(errCount))
		return fmt.Errorf("failed to create or update %d resources for compact or compact shard(s)", errCount)
	}

	return nil
}

func (r *ThanosCompactReconciler) pruneOrphanedResources(ctx context.Context, ns, instanceLabel string, expectShards []string) error {
	ls := &metav1.LabelSelector{MatchLabels: map[string]string{manifests.InstanceLabel: instanceLabel}}
	labelSelector, err := manifests.BuildLabelSelectorFrom(ls, manifestcompact.GetRequiredLabels())
	if err != nil {
		return err
	}

	deleteOrphanedShardResources := func(obj client.Object) error {
		if !slices.Contains(expectShards, obj.GetName()) {
			err := r.Delete(ctx, obj)
			if err != nil && !apierrors.IsNotFound(err) {
				return err
			}
		}
		return nil
	}

	compactSTS := &v1.StatefulSetList{}
	opts := []client.ListOption{client.MatchingLabelsSelector{Selector: labelSelector}, client.InNamespace(ns)}
	if err := r.List(ctx, compactSTS, opts...); err != nil {
		return err
	}

	for _, s := range compactSTS.Items {
		if err := deleteOrphanedShardResources(&s); err != nil {
			return fmt.Errorf("failed to delete orphaned StatefulSets: %w", err)
		}
	}

	compactServices := &corev1.ServiceList{}
	if err := r.List(ctx, compactServices, opts...); err != nil {
		return err
	}

	for _, s := range compactServices.Items {
		if err := deleteOrphanedShardResources(&s); err != nil {
			return fmt.Errorf("failed to delete orphaned Service: %w", err)
		}
	}

	return nil
}

// buildCompact returns a slice of slices of client.Object that represents the desired state of the Thanos Compact resources.
// each key represents a named shard and each slice value represents the resources for that shard.
func (r *ThanosCompactReconciler) buildCompact(compact monitoringthanosiov1alpha1.ThanosCompact) map[string][]client.Object {
	opts := r.specToOptions(compact)
	shardedObjects := make(map[string][]client.Object, len(opts))

	for i, opt := range opts {
		compactorObjs := manifestcompact.Build(opt)
		shardedObjects[i] = append(shardedObjects[i], compactorObjs...)
	}
	return shardedObjects
}

func (r *ThanosCompactReconciler) specToOptions(compact monitoringthanosiov1alpha1.ThanosCompact) map[string]manifestcompact.Options {
	if compact.Spec.ShardingConfig == nil || compact.Spec.ShardingConfig.ExternalLabelSharding == nil {
		return map[string]manifestcompact.Options{compact.GetName(): compactV1Alpha1ToOptions(compact)}
	}

	shardedOptions := make(map[string]manifestcompact.Options)
	for _, shard := range compact.Spec.ShardingConfig.ExternalLabelSharding {
		for i, v := range shard.Values {
			shardName := CompactShardName(compact.GetName(), shard.ShardName, i)
			opts := compactV1Alpha1ToOptions(compact)
			opts.Name = shardName
			opts.InstanceName = compact.GetName()
			opts.RelabelConfigs = manifests.RelabelConfigs{
				{
					Action:      "keep",
					SourceLabel: shard.Label,
					Regex:       v,
				},
			}
			shardedOptions[shardName] = opts
		}
	}
	return shardedOptions
}
