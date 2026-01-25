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
	"github.com/thanos-community/thanos-operator/internal/pkg/featuregate"
	"github.com/thanos-community/thanos-operator/internal/pkg/handlers"
	"github.com/thanos-community/thanos-operator/internal/pkg/manifests"
	manifestcompact "github.com/thanos-community/thanos-operator/internal/pkg/manifests/compact"
	controllermetrics "github.com/thanos-community/thanos-operator/internal/pkg/metrics"

	corev1 "k8s.io/api/core/v1"
	apierrors "k8s.io/apimachinery/pkg/api/errors"
	"k8s.io/apimachinery/pkg/runtime"
	"k8s.io/client-go/tools/record"
	"k8s.io/utils/ptr"

	"k8s.io/apimachinery/pkg/api/meta"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
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

	handler                *handlers.Handler
	disableConditionUpdate bool

	featureGate featuregate.Config
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
	compact := &monitoringthanosiov1alpha1.ThanosCompact{}
	err := r.Get(ctx, req.NamespacedName, compact)
	if err != nil {
		if apierrors.IsNotFound(err) {
			r.logger.Info("thanos compact resource not found. ignoring since object may be deleted")
			return ctrl.Result{}, nil
		}
		r.logger.Error(err, "failed to get ThanosCompact")
		r.recorder.Event(compact, corev1.EventTypeWarning, "GetFailed", "Failed to get ThanosCompact resource")
		return ctrl.Result{}, err
	}

	if compact.Spec.Paused != nil && *compact.Spec.Paused {
		r.logger.Info("reconciliation is paused for ThanosCompact resource")
		r.metrics.Paused.WithLabelValues("compact", compact.GetName(), compact.GetNamespace()).Set(1)
		r.recorder.Event(compact, corev1.EventTypeNormal, "Paused", "Reconciliation is paused for ThanosCompact resource")
		r.updateCondition(ctx, compact, metav1.Condition{
			Type:    ConditionPaused,
			Status:  metav1.ConditionTrue,
			Reason:  ReasonPaused,
			Message: "Reconciliation is paused",
		})
		return ctrl.Result{}, nil
	}

	r.metrics.Paused.WithLabelValues("compact", compact.GetName(), compact.GetNamespace()).Set(0)

	err = r.syncResources(ctx, *compact)
	if err != nil {
		r.logger.Error(err, "failed to sync resources", "resource", compact.GetName(), "namespace", compact.GetNamespace())
		r.recorder.Event(compact, corev1.EventTypeWarning, "SyncFailed", fmt.Sprintf("Failed to sync resources: %v", err))
		r.updateCondition(ctx, compact, metav1.Condition{
			Type:    ConditionReconcileFailed,
			Status:  metav1.ConditionTrue,
			Reason:  ReasonReconcileError,
			Message: err.Error(),
		})
		return ctrl.Result{}, err
	}

	r.updateCondition(ctx, compact, metav1.Condition{
		Type:    ConditionReconcileSuccess,
		Status:  metav1.ConditionTrue,
		Reason:  ReasonReconcileComplete,
		Message: "Reconciliation completed successfully",
	})

	return ctrl.Result{}, nil
}

// NewThanosCompactReconciler returns a reconciler for ThanosCompact resources.
func NewThanosCompactReconciler(conf Config, client client.Client, scheme *runtime.Scheme) *ThanosCompactReconciler {
	reconciler := &ThanosCompactReconciler{
		Client:      client,
		Scheme:      scheme,
		logger:      conf.InstrumentationConfig.Logger,
		metrics:     controllermetrics.NewThanosCompactMetrics(conf.InstrumentationConfig.MetricsRegistry, conf.InstrumentationConfig.CommonMetrics),
		recorder:    conf.InstrumentationConfig.EventRecorder,
		featureGate: conf.FeatureGate,
		handler:     handlers.NewHandler(client, scheme, conf.InstrumentationConfig.Logger).SetFeatureGates(conf.FeatureGate.ToGVK()),
	}

	return reconciler
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
	r.metrics.ShardsConfigured.WithLabelValues(compact.GetName(), compact.GetNamespace()).Set(float64(len(options)))

	// for compactor, we want to make sure we clean up any resources that are no longer needed first
	// as we don't want multiple compactor instances potentially compacting the same data.
	expectResources := make([]string, len(options))
	for _, opt := range options {
		expectResources = append(expectResources, opt.GetGeneratedResourceName())
	}

	errCount = r.pruneOrphanedResources(ctx, compact.GetNamespace(), compact.GetName(), expectResources)
	if errCount > 0 {
		return fmt.Errorf("failed to prune %d orphaned resources for compact or compact shard(s)", errCount)
	}

	// now we can create what we expect to be built based on the spec
	for _, opt := range options {
		errCount += r.handler.CreateOrUpdate(ctx, compact.GetNamespace(), &compact, opt.Build())
	}

	if errCount > 0 {
		r.metrics.ShardCreationUpdateFailures.WithLabelValues(compact.GetName(), compact.GetNamespace()).Add(float64(errCount))
		return fmt.Errorf("failed to create or update %d resources for compact or compact shard(s)", errCount)
	}

	if errCount = r.handler.DeleteResource(ctx,
		getDisabledFeatureGatedResources(r.featureGate, expectResources, compact.GetNamespace())); errCount > 0 {
		return fmt.Errorf("failed to delete %d feature gated resources for the compactor", errCount)
	}

	return nil
}

func (r *ThanosCompactReconciler) pruneOrphanedResources(ctx context.Context, ns, owner string, expectShards []string) int {
	listOpt := manifests.GetLabelSelectorForOwner(manifestcompact.Options{Options: manifests.Options{Owner: owner}})
	listOpts := []client.ListOption{listOpt, client.InNamespace(ns)}

	pruner := r.handler.NewResourcePruner().WithServiceAccount().WithService().WithStatefulSet().WithServiceMonitor()
	return pruner.Prune(ctx, expectShards, listOpts...)
}

func (r *ThanosCompactReconciler) specToOptions(compact monitoringthanosiov1alpha1.ThanosCompact) []manifests.Buildable {
	if len(compact.Spec.ShardingConfig) == 0 {
		return []manifests.Buildable{compactV1Alpha1ToOptions(compact, r.featureGate, r.logger)}
	}

	buildable := make([]manifests.Buildable, 0, len(compact.Spec.ShardingConfig))
	for _, shard := range compact.Spec.ShardingConfig {
		relabelsConfigs := make(manifests.RelabelConfigs, 0, len(shard.ExternalLabelSharding))
		for _, v := range shard.ExternalLabelSharding {
			relabelsConfigs = append(relabelsConfigs, manifests.RelabelConfig{
				Action:      "keep",
				SourceLabel: v.Label,
				Regex:       v.Value,
			})
		}
		opts := compactV1Alpha1ToOptions(compact, r.featureGate, r.logger)
		opts.ShardName = ptr.To(shard.ShardName)
		opts.RelabelConfigs = relabelsConfigs
		buildable = append(buildable, opts)
	}

	return buildable
}

func (r *ThanosCompactReconciler) DisableConditionUpdate() *ThanosCompactReconciler {
	r.disableConditionUpdate = true
	return r
}

// updateCondition updates the status conditions of the ThanosCompact resource
func (r *ThanosCompactReconciler) updateCondition(ctx context.Context, compact *monitoringthanosiov1alpha1.ThanosCompact, condition metav1.Condition) {
	if r.disableConditionUpdate {
		return
	}
	conditions := compact.Status.Conditions
	meta.SetStatusCondition(&conditions, condition)
	compact.Status.Conditions = conditions
	if condition.Type == ConditionPaused {
		compact.Status.Paused = ptr.To(true)
	}

	if err := r.Status().Update(ctx, compact); err != nil {
		r.logger.Error(err, "failed to update status for ThanosCompact", "name", compact.Name)
	}
}
