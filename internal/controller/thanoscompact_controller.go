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

	monitoringv1 "github.com/prometheus-operator/prometheus-operator/pkg/apis/monitoring/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"

	"github.com/go-logr/logr"

	monitoringthanosiov1alpha1 "github.com/thanos-community/thanos-operator/api/v1alpha1"
	"github.com/thanos-community/thanos-operator/internal/pkg/handlers"
	"github.com/thanos-community/thanos-operator/internal/pkg/manifests"
	manifestcompact "github.com/thanos-community/thanos-operator/internal/pkg/manifests/compact"
	controllermetrics "github.com/thanos-community/thanos-operator/internal/pkg/metrics"

	corev1 "k8s.io/api/core/v1"
	apierrors "k8s.io/apimachinery/pkg/api/errors"
	"k8s.io/apimachinery/pkg/runtime"
	"k8s.io/client-go/tools/record"
	"k8s.io/utils/ptr"
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
func NewThanosCompactReconciler(conf Config, client client.Client, scheme *runtime.Scheme) *ThanosCompactReconciler {
	handler := handlers.NewHandler(client, scheme, conf.InstrumentationConfig.Logger)
	featureGates := conf.FeatureGate.ToGVK()
	if len(featureGates) > 0 {
		handler.SetFeatureGates(featureGates)
	}

	return &ThanosCompactReconciler{
		Client:   client,
		Scheme:   scheme,
		logger:   conf.InstrumentationConfig.Logger,
		metrics:  controllermetrics.NewThanosCompactMetrics(conf.InstrumentationConfig.MetricsRegistry, conf.InstrumentationConfig.BaseMetrics),
		recorder: conf.InstrumentationConfig.EventRecorder,
		handler:  handler,
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
	options := r.specToOptions(compact)

	// for compactor, we want to make sure we clean up any resources that are no longer needed first
	// as we don't want multiple compactor instances potentially compacting the same data.
	expectResources := make([]string, len(options))
	for _, opt := range options {
		expectResources = append(expectResources, opt.GetGeneratedResourceName())
	}

	errCount = r.pruneOrphanedResources(ctx, compact.GetNamespace(), compact.GetName(), expectResources)
	if errCount > 0 {
		r.metrics.ClientErrorsTotal.WithLabelValues(manifestcompact.Name).Add(float64(errCount))
		return fmt.Errorf("failed to prune %d orphaned resources for compact or compact shard(s)", errCount)
	}

	// now we can create what we expect to be built based on the spec
	for _, opt := range options {
		errCount += r.handler.CreateOrUpdate(ctx, compact.GetNamespace(), &compact, opt.Build())
	}

	if errCount > 0 {
		r.metrics.ClientErrorsTotal.WithLabelValues(manifestcompact.Name).Add(float64(errCount))
		return fmt.Errorf("failed to create or update %d resources for compact or compact shard(s)", errCount)
	}

	if !r.hasServiceMonitorsEnabled(compact) {
		objs := make([]client.Object, len(expectResources))
		for i, resource := range expectResources {
			objs[i] = &monitoringv1.ServiceMonitor{ObjectMeta: metav1.ObjectMeta{Name: resource, Namespace: compact.GetNamespace()}}
		}
		if errCount = r.handler.DeleteResource(ctx, objs); errCount > 0 {
			r.metrics.ClientErrorsTotal.WithLabelValues(manifestcompact.Name).Add(float64(errCount))
			return fmt.Errorf("failed to delete %d ServiceMonitors for the store shard(s)", errCount)
		}
	}

	return nil
}

func (r *ThanosCompactReconciler) hasServiceMonitorsEnabled(compact monitoringthanosiov1alpha1.ThanosCompact) bool {
	return compact.Spec.ServiceMonitorConfig != nil && compact.Spec.ServiceMonitorConfig.Enable != nil && *compact.Spec.ServiceMonitorConfig.Enable
}

func (r *ThanosCompactReconciler) pruneOrphanedResources(ctx context.Context, ns, owner string, expectShards []string) int {
	listOpt := manifests.GetLabelSelectorForOwner(manifestcompact.Options{Options: manifests.Options{Owner: owner}})
	listOpts := []client.ListOption{listOpt, client.InNamespace(ns)}

	pruner := r.handler.NewResourcePruner().WithServiceAccount().WithService().WithStatefulSet().WithServiceMonitor()
	return pruner.Prune(ctx, expectShards, listOpts...)
}

func (r *ThanosCompactReconciler) specToOptions(compact monitoringthanosiov1alpha1.ThanosCompact) []manifests.Buildable {
	if compact.Spec.ShardingConfig == nil || compact.Spec.ShardingConfig.ExternalLabelSharding == nil {
		return []manifests.Buildable{compactV1Alpha1ToOptions(compact)}
	}

	var buildable []manifests.Buildable
	for _, shard := range compact.Spec.ShardingConfig.ExternalLabelSharding {
		for i, v := range shard.Values {
			opts := compactV1Alpha1ToOptions(compact)
			opts.ShardName = ptr.To(shard.ShardName)
			opts.ShardIndex = ptr.To(i)
			opts.RelabelConfigs = manifests.RelabelConfigs{
				{
					Action:      "keep",
					SourceLabel: shard.Label,
					Regex:       v,
				},
			}
			buildable = append(buildable, opts)
		}
	}
	return buildable
}
