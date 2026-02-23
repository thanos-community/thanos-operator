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
	"sync"
	"sync/atomic"

	"github.com/go-logr/logr"
	policyv1 "k8s.io/api/policy/v1"
	"k8s.io/client-go/tools/events"

	monitoringthanosiov1alpha1 "github.com/thanos-community/thanos-operator/api/v1alpha1"
	"github.com/thanos-community/thanos-operator/internal/pkg/featuregate"
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
	"k8s.io/utils/ptr"

	ctrl "sigs.k8s.io/controller-runtime"
	"sigs.k8s.io/controller-runtime/pkg/builder"
	"sigs.k8s.io/controller-runtime/pkg/client"
	"sigs.k8s.io/controller-runtime/pkg/controller/controllerutil"
	"sigs.k8s.io/controller-runtime/pkg/handler"
	"sigs.k8s.io/controller-runtime/pkg/predicate"
	"sigs.k8s.io/controller-runtime/pkg/reconcile"
)

//+kubebuilder:rbac:groups=monitoring.thanos.io,resources=thanosreceives,verbs=get;list;watch;create;update;patch;delete
//+kubebuilder:rbac:groups=monitoring.thanos.io,resources=thanosreceives/status,verbs=get;update;patch
//+kubebuilder:rbac:groups=monitoring.thanos.io,resources=thanosreceives/finalizers,verbs=update
//+kubebuilder:rbac:groups=apps,resources=statefulsets,verbs=get;list;watch;create;update;patch;delete
//+kubebuilder:rbac:groups="",resources=services;configmaps;serviceaccounts,verbs=get;list;watch;create;update;patch;delete
//+kubebuilder:rbac:groups=policy,resources=poddisruptionbudgets,verbs=get;list;watch;create;update;patch;delete
//+kubebuilder:rbac:groups=monitoring.coreos.com,resources=servicemonitors,verbs=get;list;watch;create;update;patch;delete
//+kubebuilder:rbac:groups=discovery.k8s.io,resources=endpointslices,verbs=get;list;watch
//+kubebuilder:rbac:groups=rbac.authorization.k8s.io,resources=roles,verbs=get;list;watch;create;update;patch;delete
//+kubebuilder:rbac:groups=rbac.authorization.k8s.io,resources=rolebindings,verbs=get;list;watch;create;update;patch;delete

const (
	receiveFinalizer = "monitoring.thanos.io/receive-finalizer"
)

// ThanosReceiveReconciler reconciles a ThanosReceive object
type ThanosReceiveReconciler struct {
	client.Client
	Scheme *runtime.Scheme

	logger   logr.Logger
	metrics  controllermetrics.ThanosReceiveMetrics
	recorder events.EventRecorder

	handler                *handlers.Handler
	disableConditionUpdate bool
	featureGate            featuregate.Config
}

// NewThanosReceiveReconciler returns a reconciler for ThanosReceive resources.
func NewThanosReceiveReconciler(conf Config, client client.Client, scheme *runtime.Scheme) *ThanosReceiveReconciler {
	reconciler := &ThanosReceiveReconciler{
		Client:      client,
		Scheme:      scheme,
		logger:      conf.InstrumentationConfig.Logger,
		metrics:     controllermetrics.NewThanosReceiveMetrics(conf.InstrumentationConfig.MetricsRegistry, conf.InstrumentationConfig.CommonMetrics),
		recorder:    conf.InstrumentationConfig.EventRecorder,
		featureGate: conf.FeatureGate,
		handler:     handlers.NewHandler(client, scheme, conf.InstrumentationConfig.Logger).SetFeatureGates(conf.FeatureGate.ToGVK()),
	}

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
		r.recorder.Eventf(receiver, nil, corev1.EventTypeWarning, "GetFailed", "Reconcile", "Failed to get ThanosReceive resource")
		return ctrl.Result{}, err
	}

	if receiver.Spec.Paused != nil && *receiver.Spec.Paused {
		r.logger.Info("receiver is paused")
		r.recorder.Eventf(receiver, nil, corev1.EventTypeNormal, "Paused", "Reconcile",
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
		r.recorder.Eventf(receiver, nil, corev1.EventTypeWarning, "SyncFailed", "Reconcile", "Failed to sync resources: %v", err)
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
// +kubebuilder:rbac:groups="",resources=pods,verbs=get;list;watch;patch
// +kubebuilder:rbac:groups=policy,resources=poddisruptionbudgets,verbs=get;list;watch;create;update;patch;delete
// SetupWithManager sets up the controller with the Manager.
func (r *ThanosReceiveReconciler) SetupWithManager(mgr ctrl.Manager) error {
	bld := ctrl.NewControllerManagedBy(mgr)
	err := r.buildController(*bld)
	if err != nil {
		r.recorder.Eventf(&monitoringthanosiov1alpha1.ThanosReceive{}, nil, corev1.EventTypeWarning, "SetupFailed", "Setup", "Failed to set up controller: %v", err)
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
	// we won't error out here yet as we don't want to delay updating the router configmap

	hashringResult, err := r.buildHashringConfig(ctx, receiver)
	if err != nil {
		return fmt.Errorf("failed to build hashring config: %w", err)
	}
	routerOpts := r.specToRouterOptions(receiver, hashringResult.Config)

	if errs := r.handler.CreateOrUpdate(ctx, receiver.GetNamespace(), &receiver, routerOpts.Build()); errs > 0 {
		return fmt.Errorf("failed to create or update %d resources for the receive router", errs)
	}

	// Annotate router pods with hashring config hash
	if errCount := r.annotateRouterPods(ctx, receiver, hashringResult.Hash, routerOpts); errCount > 0 {
		return fmt.Errorf("failed to annotate %d router pods", errCount)
	}

	// we go back and force a reconcile now on the original errors from the ingesters
	if errCount > 0 {
		return fmt.Errorf("failed to create or update %d resources for receive hashring(s)", errCount)
	}

	cleanupErrCount := r.cleanup(ctx, receiver, expectIngesters, routerOpts.GetGeneratedResourceName())
	if cleanupErrCount > 0 {
		return fmt.Errorf("failed to clean up %d orphaned resources for the receiver", cleanupErrCount)
	}

	return nil

}

func (r *ThanosReceiveReconciler) specToIngestOptions(receiver monitoringthanosiov1alpha1.ThanosReceive) []manifests.Buildable {
	opts := make([]manifests.Buildable, len(receiver.Spec.Ingester.Hashrings))
	for i, v := range receiver.Spec.Ingester.Hashrings {
		opt := receiverV1Alpha1ToIngesterOptions(receiver, v, r.featureGate)
		opt.HashringName = v.Name
		opts[i] = opt
	}
	return opts
}

func (r *ThanosReceiveReconciler) specToRouterOptions(receiver monitoringthanosiov1alpha1.ThanosReceive, hashringConfig string) manifests.Buildable {
	opts := receiverV1Alpha1ToRouterOptions(receiver, r.featureGate)
	opts.HashringConfig = hashringConfig
	return opts
}

// hashringConfigResult contains the hashring configuration and its hash value
type hashringConfigResult struct {
	Config string  // The hashring configuration as a string
	Hash   float64 // The hash value of the configuration
}

// buildHashringConfig builds the hashring configuration for the ThanosReceive resource.
func (r *ThanosReceiveReconciler) buildHashringConfig(ctx context.Context, receiver monitoringthanosiov1alpha1.ThanosReceive) (*hashringConfigResult, error) {
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
		return &hashringConfigResult{Config: "", Hash: 0}, nil
	}

	for _, hashring := range out {
		r.metrics.HashringTenantsConfigured.WithLabelValues(receiver.GetName(), receiver.GetNamespace(), hashring.Name).Set(float64(len(hashring.Tenants)))
		r.metrics.HashringEndpointsConfigured.WithLabelValues(receiver.GetName(), receiver.GetNamespace(), hashring.Name).Set(float64(len(hashring.Endpoints)))
	}

	b, err := json.MarshalIndent(out, "", "    ")
	if err != nil {
		return nil, fmt.Errorf("failed to marshal hashring config: %w", err)
	}

	hashValue := receive.HashAsMetricValue(b)
	r.metrics.HashringHash.WithLabelValues(receiver.GetName(), receiver.GetNamespace()).Set(hashValue)
	r.metrics.HashringsConfigured.WithLabelValues(receiver.GetName(), receiver.GetNamespace()).Set(float64(len(out)))

	return &hashringConfigResult{
		Config: string(b),
		Hash:   hashValue,
	}, nil
}

func (r *ThanosReceiveReconciler) handleDeletionTimestamp(receiveHashring *monitoringthanosiov1alpha1.ThanosReceive) (ctrl.Result, error) {
	if controllerutil.ContainsFinalizer(receiveHashring, receiveFinalizer) {
		r.logger.Info("performing Finalizer Operations for ThanosReceiveHashring before delete CR")

		r.recorder.Eventf(receiveHashring, nil, "Warning", "Deleting", "Cleanup",
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

func (r *ThanosReceiveReconciler) cleanup(ctx context.Context, resource monitoringthanosiov1alpha1.ThanosReceive, expectedIngesters []string, routerName string) int {
	var errCount int
	ns := resource.GetNamespace()
	owner := resource.GetName()

	errCount = r.pruneOrphanedResources(ctx, ns, owner, expectedIngesters)
	errCount += r.handler.DeleteResource(ctx, getDisabledFeatureGatedResources(r.featureGate, append(expectedIngesters, routerName), ns))

	if resource.Spec.Router.Replicas < 2 {
		listOpt := manifests.GetLabelSelectorForOwner(manifestreceive.RouterOptions{Options: manifests.Options{Owner: owner}})
		listOpts := []client.ListOption{listOpt, client.InNamespace(ns)}
		errCount += r.handler.NewResourcePruner().WithPodDisruptionBudget().Prune(ctx, []string{}, listOpts...)
	}

	for _, hashring := range resource.Spec.Ingester.Hashrings {
		var objs []client.Object
		if hashring.Replicas < 2 {
			objs = append(objs, &policyv1.PodDisruptionBudget{ObjectMeta: metav1.ObjectMeta{Name: ReceiveIngesterNameFromParent(resource.GetName(), hashring.Name), Namespace: ns}})
		}
		errCount += r.handler.DeleteResource(ctx, objs)
	}

	return errCount
}

func (r *ThanosReceiveReconciler) pruneOrphanedResources(ctx context.Context, ns, owner string, expectShards []string) int {
	listOpt := manifests.GetLabelSelectorForOwner(manifestreceive.IngesterOptions{Options: manifests.Options{Owner: owner}})
	listOpts := []client.ListOption{listOpt, client.InNamespace(ns)}

	pruner := r.handler.NewResourcePruner().WithServiceAccount().WithService().WithStatefulSet().WithPodDisruptionBudget().WithServiceMonitor()
	return pruner.Prune(ctx, expectShards, listOpts...)
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

// annotateRouterPods annotates router pods with the hashring configuration hash
// returns the number of errors encountered during annotation process
func (r *ThanosReceiveReconciler) annotateRouterPods(ctx context.Context, receiver monitoringthanosiov1alpha1.ThanosReceive, hashValue float64, routerOpts manifests.Buildable) int {
	configHash := fmt.Sprintf("%.0f", hashValue)

	podList := &corev1.PodList{}
	listOpts := []client.ListOption{
		manifests.GetLabelSelectorForOwner(routerOpts),
		client.InNamespace(receiver.GetNamespace()),
	}

	if err := r.Client.List(ctx, podList, listOpts...); err != nil {
		r.logger.Error(err, "failed to list router pods for annotation")
		return 1
	}

	var podsToAnnotate []corev1.Pod
	for _, pod := range podList.Items {
		if pod.DeletionTimestamp.IsZero() {
			podsToAnnotate = append(podsToAnnotate, pod)
		}
	}

	if len(podsToAnnotate) == 0 {
		return 0
	}

	var wg sync.WaitGroup
	var errorCount int32
	annotationKey := "thanos.io/hashring-config-hash"

	for _, pod := range podsToAnnotate {
		wg.Add(1)
		go func(p corev1.Pod) {
			defer wg.Done()

			podPatch := &corev1.Pod{
				TypeMeta: metav1.TypeMeta{
					APIVersion: "v1",
					Kind:       "Pod",
				},
				ObjectMeta: metav1.ObjectMeta{
					Name:      p.Name,
					Namespace: p.Namespace,
					Annotations: map[string]string{
						annotationKey: configHash,
					},
				},
			}

			if err := r.Patch(ctx, podPatch, client.Apply, client.FieldOwner("thanos-operator"), client.ForceOwnership); err != nil {
				r.logger.Error(err, "failed to annotate pod", "pod", p.Name, "hash", configHash)
				atomic.AddInt32(&errorCount, 1)
			} else {
				r.logger.Info("annotated router pod with hashring config hash", "pod", p.Name, "hash", configHash)
			}
		}(pod)
	}

	wg.Wait()
	return int(errorCount)
}
