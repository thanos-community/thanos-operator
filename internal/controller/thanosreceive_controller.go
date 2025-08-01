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
	"encoding/json"
	"fmt"

	"github.com/go-logr/logr"

	monitoringthanosiov1alpha1 "github.com/thanos-community/thanos-operator/api/v1alpha1"
	"github.com/thanos-community/thanos-operator/internal/pkg/handlers"
	"github.com/thanos-community/thanos-operator/internal/pkg/manifests"
	manifestreceive "github.com/thanos-community/thanos-operator/internal/pkg/manifests/receive"
	controllermetrics "github.com/thanos-community/thanos-operator/internal/pkg/metrics"
	"github.com/thanos-community/thanos-operator/internal/pkg/receive"

	appsv1 "k8s.io/api/apps/v1"
	corev1 "k8s.io/api/core/v1"
	discoveryv1 "k8s.io/api/discovery/v1"
	apierrors "k8s.io/apimachinery/pkg/api/errors"
	"k8s.io/apimachinery/pkg/api/meta"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/runtime"
	"k8s.io/apimachinery/pkg/types"
	"k8s.io/client-go/tools/record"
	"k8s.io/utils/ptr"

	ctrl "sigs.k8s.io/controller-runtime"
	"sigs.k8s.io/controller-runtime/pkg/builder"
	"sigs.k8s.io/controller-runtime/pkg/client"
	"sigs.k8s.io/controller-runtime/pkg/controller/controllerutil"
	"sigs.k8s.io/controller-runtime/pkg/handler"
	"sigs.k8s.io/controller-runtime/pkg/predicate"
	"sigs.k8s.io/controller-runtime/pkg/reconcile"
)

const (
	receiveFinalizer = "monitoring.thanos.io/receive-finalizer"
)

// ThanosReceiveReconciler reconciles a ThanosReceive object
type ThanosReceiveReconciler struct {
	client.Client
	Scheme *runtime.Scheme

	logger   logr.Logger
	metrics  controllermetrics.ThanosReceiveMetrics
	recorder record.EventRecorder

	handler                *handlers.Handler
	disableConditionUpdate bool
}

// NewThanosReceiveReconciler returns a reconciler for ThanosReceive resources.
func NewThanosReceiveReconciler(conf Config, client client.Client, scheme *runtime.Scheme) *ThanosReceiveReconciler {
	reconciler := &ThanosReceiveReconciler{
		Client:   client,
		Scheme:   scheme,
		logger:   conf.InstrumentationConfig.Logger,
		metrics:  controllermetrics.NewThanosReceiveMetrics(conf.InstrumentationConfig.MetricsRegistry, conf.InstrumentationConfig.CommonMetrics),
		recorder: conf.InstrumentationConfig.EventRecorder,
	}

	handler := handlers.NewHandler(client, scheme, conf.InstrumentationConfig.Logger)
	featureGates := conf.FeatureGate.ToGVK()
	if len(featureGates) > 0 {
		handler.SetFeatureGates(featureGates)
		reconciler.metrics.FeatureGatesEnabled.WithLabelValues("receive").Set(float64(len(featureGates)))
	}
	reconciler.handler = handler

	return reconciler
}

// Reconcile is part of the main kubernetes reconciliation loop which aims to
// move the current state of the cluster closer to the desired state.
// For more details, check Reconcile and its Result here:
// - https://pkg.go.dev/sigs.k8s.io/controller-runtime@v0.17.3/pkg/reconcile
func (r *ThanosReceiveReconciler) Reconcile(ctx context.Context, req ctrl.Request) (ctrl.Result, error) {
	receiver := &monitoringthanosiov1alpha1.ThanosReceive{}
	err := r.Get(ctx, req.NamespacedName, receiver)
	if err != nil {
		if apierrors.IsNotFound(err) {
			r.logger.Info("thanos receive resource not found. ignoring since object may be deleted")
			return ctrl.Result{}, nil
		}
		r.logger.Error(err, "failed to get ThanosReceive")
		r.recorder.Event(receiver, corev1.EventTypeWarning, "GetFailed", "Failed to get ThanosReceive resource")
		return ctrl.Result{}, err
	}

	if receiver.Spec.Paused != nil && *receiver.Spec.Paused {
		r.logger.Info("receiver is paused")
		r.recorder.Event(receiver, corev1.EventTypeNormal, "Paused",
			"Reconciliation is paused for ThanosReceive resource")
		r.metrics.Paused.WithLabelValues("receive", receiver.GetName(), receiver.GetNamespace()).Set(1)
		r.updateCondition(ctx, receiver, metav1.Condition{
			Type:    ConditionPaused,
			Status:  metav1.ConditionTrue,
			Reason:  ReasonPaused,
			Message: "Reconciliation is paused",
		})
		return ctrl.Result{}, nil
	}

	r.metrics.Paused.WithLabelValues("receive", receiver.GetName(), receiver.GetNamespace()).Set(0)

	if !receiver.GetDeletionTimestamp().IsZero() {
		return r.handleDeletionTimestamp(receiver)
	}

	err = r.syncResources(ctx, *receiver)
	if err != nil {
		r.logger.Error(err, "failed to sync resources", "resource", receiver.GetName(), "namespace", receiver.GetNamespace())
		r.recorder.Event(receiver, corev1.EventTypeWarning, "SyncFailed", fmt.Sprintf("Failed to sync resources: %v", err))
		r.updateCondition(ctx, receiver, metav1.Condition{
			Type:    ConditionReconcileFailed,
			Status:  metav1.ConditionTrue,
			Reason:  ReasonReconcileError,
			Message: err.Error(),
		})
		return ctrl.Result{}, err
	}

	r.updateCondition(ctx, receiver, metav1.Condition{
		Type:    ConditionReconcileSuccess,
		Status:  metav1.ConditionTrue,
		Reason:  ReasonReconcileComplete,
		Message: "Reconciliation completed successfully",
	})

	return ctrl.Result{}, nil
}

// +kubebuilder:rbac:groups=monitoring.thanos.io,resources=thanosreceives,verbs=get;list;watch;create;update;patch;delete
// +kubebuilder:rbac:groups=monitoring.thanos.io,resources=thanosreceives/status,verbs=get;update;patch
// +kubebuilder:rbac:groups=monitoring.thanos.io,resources=thanosreceives/finalizers,verbs=update
// +kubebuilder:rbac:groups=apps,resources=statefulsets;deployments,verbs=get;list;watch;create;update;patch;delete
// +kubebuilder:rbac:groups="",resources=services;configmaps;serviceaccounts,verbs=get;list;watch;create;update;patch;delete
// +kubebuilder:rbac:groups="discovery.k8s.io",resources=endpointslices,verbs=get;list;watch
// +kubebuilder:rbac:groups=policy,resources=poddisruptionbudgets,verbs=get;list;watch;create;update;patch;delete

// SetupWithManager sets up the controller with the Manager.
func (r *ThanosReceiveReconciler) SetupWithManager(mgr ctrl.Manager) error {
	bld := ctrl.NewControllerManagedBy(mgr)
	err := r.buildController(*bld)
	if err != nil {
		r.recorder.Event(&monitoringthanosiov1alpha1.ThanosReceive{}, corev1.EventTypeWarning, "SetupFailed", fmt.Sprintf("Failed to set up controller: %v", err))
		return err
	}

	return nil
}

// buildController sets up the controller with the Manager.
func (r *ThanosReceiveReconciler) buildController(bld builder.Builder) error {
	// add a selector to watch for the endpointslices that are owned by the ThanosReceive ingest Service(s).
	endpointSliceLS := metav1.LabelSelector{
		MatchLabels: map[string]string{manifests.ComponentLabel: manifestreceive.IngestComponentName},
	}
	endpointSlicePredicate, err := predicate.LabelSelectorPredicate(endpointSliceLS)
	if err != nil {
		return err
	}

	bld.
		For(&monitoringthanosiov1alpha1.ThanosReceive{}).
		Owns(&corev1.ConfigMap{}).
		Owns(&corev1.ServiceAccount{}).
		Owns(&corev1.Service{}).
		Owns(&appsv1.Deployment{}).
		Owns(&appsv1.StatefulSet{}).
		Watches(
			&discoveryv1.EndpointSlice{},
			r.enqueueForEndpointSlice(r.Client),
			builder.WithPredicates(predicate.GenerationChangedPredicate{}, endpointSlicePredicate),
		)

	return bld.Complete(r)
}

// syncResources syncs the resources for the ThanosReceive resource.
// It creates or updates the resources for the hashrings and the router.
func (r *ThanosReceiveReconciler) syncResources(ctx context.Context, receiver monitoringthanosiov1alpha1.ThanosReceive) error {
	var errCount int

	ingestOpts := r.specToIngestOptions(receiver)
	expectIngesters := make([]string, len(ingestOpts))
	for i, opt := range ingestOpts {
		expectIngesters[i] = opt.GetGeneratedResourceName()
		errCount += r.handler.CreateOrUpdate(ctx, receiver.GetNamespace(), &receiver, opt.Build())
	}

	if errCount > 0 {
		return fmt.Errorf("failed to create or update %d resources for receive hashring(s)", errCount)
	}

	// prune the ingest resources that are no longer needed/have changed
	errCount = r.pruneOrphanedResources(ctx, receiver.GetNamespace(), receiver.GetName(), expectIngesters)
	if errCount > 0 {
		return fmt.Errorf("failed to prune %d orphaned resources for receive ingester(s)", errCount)
	}

	hashringConfig, err := r.buildHashringConfig(ctx, receiver)
	if err != nil {
		return fmt.Errorf("failed to build hashring config: %w", err)
	}
	routerOpts := r.specToRouterOptions(receiver, string(hashringConfig))

	if errs := r.handler.CreateOrUpdate(ctx, receiver.GetNamespace(), &receiver, routerOpts.Build()); errs > 0 {
		return fmt.Errorf("failed to create or update %d resources for the receive router", errs)
	}

	if errCount = r.handler.DeleteResource(ctx,
		getDisabledFeatureGatedResources(receiver.Spec.FeatureGates, append(expectIngesters, routerOpts.GetGeneratedResourceName()), receiver.GetNamespace())); errCount > 0 {
		return fmt.Errorf("failed to delete %d feature gated resources for the receiver", errCount)
	}

	return nil
}

func (r *ThanosReceiveReconciler) specToIngestOptions(receiver monitoringthanosiov1alpha1.ThanosReceive) []manifests.Buildable {
	opts := make([]manifests.Buildable, len(receiver.Spec.Ingester.Hashrings))
	for i, v := range receiver.Spec.Ingester.Hashrings {
		opt := receiverV1Alpha1ToIngesterOptions(receiver, v)
		opt.HashringName = v.Name
		opts[i] = opt
	}
	return opts
}

func (r *ThanosReceiveReconciler) specToRouterOptions(receiver monitoringthanosiov1alpha1.ThanosReceive, hashringConfig string) manifests.Buildable {
	opts := receiverV1Alpha1ToRouterOptions(receiver)
	opts.HashringConfig = hashringConfig
	return opts
}

func (r *ThanosReceiveReconciler) pruneOrphanedResources(ctx context.Context, ns, owner string, expectShards []string) int {
	listOpt := manifests.GetLabelSelectorForOwner(manifestreceive.IngesterOptions{Options: manifests.Options{Owner: owner}})
	listOpts := []client.ListOption{listOpt, client.InNamespace(ns)}

	pruner := r.handler.NewResourcePruner().WithServiceAccount().WithService().WithStatefulSet().WithPodDisruptionBudget().WithServiceMonitor()
	return pruner.Prune(ctx, expectShards, listOpts...)
}

// buildHashringConfig builds the hashring configuration for the ThanosReceive resource.
func (r *ThanosReceiveReconciler) buildHashringConfig(ctx context.Context, receiver monitoringthanosiov1alpha1.ThanosReceive) ([]byte, error) {
	cm := &corev1.ConfigMap{}
	name := ReceiveRouterNameFromParent(receiver.GetName())
	err := r.Client.Get(ctx, client.ObjectKey{Namespace: receiver.GetNamespace(), Name: name}, cm)
	if err != nil {
		if !apierrors.IsNotFound(err) {
			return nil, fmt.Errorf("failed to get config map for resource %s: %w", name, err)
		}
	}

	var currentHashringState receive.Hashrings
	if cm.Data != nil && cm.Data[manifestreceive.HashringConfigKey] != "" {
		if err := json.Unmarshal([]byte(cm.Data[manifestreceive.HashringConfigKey]), &currentHashringState); err != nil {
			return nil, fmt.Errorf("failed to unmarshal current state from ConfigMap: %w", err)
		}
	}

	fetchedReadyState := make(receive.HashringState, len(receiver.Spec.Ingester.Hashrings))
	for _, hashring := range receiver.Spec.Ingester.Hashrings {
		filters := []receive.EndpointFilter{receive.FilterEndpointReady()}
		labelValue := ReceiveIngesterNameFromParent(receiver.GetName(), hashring.Name)
		eps, err := r.handler.GetEndpointSlices(ctx, labelValue, receiver.GetNamespace())
		if err != nil {
			return nil, fmt.Errorf("failed to get endpoint slices for resource %s: %w", receiver.GetName(), err)
		}

		converter := receive.DefaultEndpointConverter
		if receiver.Spec.Router.ReplicationProtocol != nil && *receiver.Spec.Router.ReplicationProtocol == monitoringthanosiov1alpha1.ReplicationProtocolCapnProto {
			converter = receive.CapnProtoEndpointConverter
		}

		hc := receive.HashringConfig{
			Name:      hashring.Name,
			Endpoints: receive.EndpointSliceListToEndpoints(converter, *eps, filters...),
		}

		if hashring.TenancyConfig != nil {
			hc.Tenants = hashring.TenancyConfig.Tenants
			hc.TenantMatcherType = receive.TenantMatcher(hashring.TenancyConfig.TenantMatcherType)
		}

		fetchedReadyState[hashring.Name] = receive.HashringMeta{
			DesiredReplicas: int(hashring.Replicas),
			Config:          hc,
		}
	}

	var out receive.Hashrings
	if receiver.Spec.Router.HashringPolicy != nil && *receiver.Spec.Router.HashringPolicy == monitoringthanosiov1alpha1.HashringPolicyStatic {
		out = receive.StaticMerge(currentHashringState, fetchedReadyState, int(receiver.Spec.Router.ReplicationFactor))
	} else {
		out = receive.DynamicMerge(currentHashringState, fetchedReadyState, int(receiver.Spec.Router.ReplicationFactor))
	}

	if len(out) == 0 {
		return []byte(""), nil
	}

	for _, hashring := range out {
		r.metrics.HashringTenantsConfigured.WithLabelValues(receiver.GetName(), receiver.GetNamespace(), hashring.Name).Set(float64(len(hashring.Tenants)))
		r.metrics.HashringEndpointsConfigured.WithLabelValues(receiver.GetName(), receiver.GetNamespace(), hashring.Name).Set(float64(len(hashring.Endpoints)))
	}

	b, err := json.MarshalIndent(out, "", "    ")
	if err != nil {
		return nil, fmt.Errorf("failed to marshal hashring config: %w", err)
	}

	r.metrics.HashringHash.WithLabelValues(receiver.GetName(), receiver.GetNamespace()).Set(receive.HashAsMetricValue(b))
	r.metrics.HashringsConfigured.WithLabelValues(receiver.GetName(), receiver.GetNamespace()).Set(float64(len(out)))
	return b, nil
}

func (r *ThanosReceiveReconciler) handleDeletionTimestamp(receiveHashring *monitoringthanosiov1alpha1.ThanosReceive) (ctrl.Result, error) {
	if controllerutil.ContainsFinalizer(receiveHashring, receiveFinalizer) {
		r.logger.Info("performing Finalizer Operations for ThanosReceiveHashring before delete CR")

		r.recorder.Event(receiveHashring, "Warning", "Deleting",
			fmt.Sprintf("Custom Resource %s is being deleted from the namespace %s",
				receiveHashring.Name,
				receiveHashring.Namespace))
	}
	return ctrl.Result{}, nil
}

// enqueueForEndpointSlice enqueues requests for the ThanosReceive resource when an EndpointSlice event is triggered.
func (r *ThanosReceiveReconciler) enqueueForEndpointSlice(c client.Client) handler.EventHandler {
	return handler.EnqueueRequestsFromMapFunc(func(ctx context.Context, obj client.Object) []reconcile.Request {

		if len(obj.GetOwnerReferences()) != 1 || obj.GetOwnerReferences()[0].Kind != "Service" {
			return nil
		}

		svc := &corev1.Service{}
		if err := c.Get(ctx, types.NamespacedName{Namespace: obj.GetNamespace(), Name: obj.GetOwnerReferences()[0].Name}, svc); err != nil {
			return nil
		}

		if len(svc.GetOwnerReferences()) != 1 || svc.GetOwnerReferences()[0].Kind != "ThanosReceive" {
			return nil
		}

		r.metrics.EndpointWatchesReconciliationsTotal.WithLabelValues(svc.GetOwnerReferences()[0].Name, obj.GetNamespace()).Inc()
		return []reconcile.Request{
			{
				NamespacedName: types.NamespacedName{
					Namespace: obj.GetNamespace(),
					Name:      svc.GetOwnerReferences()[0].Name,
				},
			},
		}
	})
}

func (r *ThanosReceiveReconciler) DisableConditionUpdate() *ThanosReceiveReconciler {
	r.disableConditionUpdate = true
	return r
}

// updateCondition updates the status conditions of the ThanosReceive resource
func (r *ThanosReceiveReconciler) updateCondition(ctx context.Context, receiver *monitoringthanosiov1alpha1.ThanosReceive, condition metav1.Condition) {
	if r.disableConditionUpdate {
		return
	}
	conditions := receiver.Status.Conditions
	meta.SetStatusCondition(&conditions, condition)
	receiver.Status.Conditions = conditions
	if condition.Type == ConditionPaused {
		receiver.Status.Paused = ptr.To(true)
	}
	if err := r.Status().Update(ctx, receiver); err != nil {
		r.logger.Error(err, "failed to update status for ThanosReceive", "name", receiver.Name)
	}
}
