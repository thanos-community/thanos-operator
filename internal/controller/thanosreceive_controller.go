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
	"errors"
	"fmt"

	"github.com/prometheus/client_golang/prometheus"
	monitoringthanosiov1alpha1 "github.com/thanos-community/thanos-operator/api/v1alpha1"
	"github.com/thanos-community/thanos-operator/internal/pkg/manifests"
	manifestreceive "github.com/thanos-community/thanos-operator/internal/pkg/manifests/receive"
	controllermetrics "github.com/thanos-community/thanos-operator/internal/pkg/metrics"

	"github.com/go-logr/logr"

	appsv1 "k8s.io/api/apps/v1"
	corev1 "k8s.io/api/core/v1"
	discoveryv1 "k8s.io/api/discovery/v1"
	apierrors "k8s.io/apimachinery/pkg/api/errors"
	"k8s.io/apimachinery/pkg/api/resource"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/runtime"
	"k8s.io/apimachinery/pkg/types"
	"k8s.io/client-go/tools/record"

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
	Scheme   *runtime.Scheme
	Recorder record.EventRecorder

	logger logr.Logger

	reg                   prometheus.Registerer
	ControllerBaseMetrics *controllermetrics.BaseMetrics
	thanosReceiveMetrics  controllermetrics.ThanosReceiveMetrics
}

// NewThanosReceiveReconciler returns a reconciler for ThanosReceive resources.
func NewThanosReceiveReconciler(logger logr.Logger, client client.Client, scheme *runtime.Scheme, recorder record.EventRecorder, reg prometheus.Registerer, controllerBaseMetrics *controllermetrics.BaseMetrics) *ThanosReceiveReconciler {
	return &ThanosReceiveReconciler{
		Client:   client,
		Scheme:   scheme,
		Recorder: recorder,

		logger: logger,

		reg:                   reg,
		ControllerBaseMetrics: controllerBaseMetrics,
		thanosReceiveMetrics:  controllermetrics.NewThanosReceiveMetrics(reg, controllerBaseMetrics),
	}
}

// Reconcile is part of the main kubernetes reconciliation loop which aims to
// move the current state of the cluster closer to the desired state.
// For more details, check Reconcile and its Result here:
// - https://pkg.go.dev/sigs.k8s.io/controller-runtime@v0.17.3/pkg/reconcile
func (r *ThanosReceiveReconciler) Reconcile(ctx context.Context, req ctrl.Request) (ctrl.Result, error) {
	r.ControllerBaseMetrics.ReconciliationsTotal.WithLabelValues(manifestreceive.Name).Inc()

	// Fetch the ThanosReceive instance to validate it is applied on the cluster.
	receiver := &monitoringthanosiov1alpha1.ThanosReceive{}
	err := r.Get(ctx, req.NamespacedName, receiver)
	if err != nil {
		r.ControllerBaseMetrics.ClientErrorsTotal.WithLabelValues(manifestreceive.Name).Inc()
		if apierrors.IsNotFound(err) {
			r.logger.Info("thanos receive resource not found. ignoring since object may be deleted")
			return ctrl.Result{}, nil
		}
		r.logger.Error(err, "failed to get ThanosReceive")
		r.ControllerBaseMetrics.ReconciliationsFailedTotal.WithLabelValues(manifestreceive.Name).Inc()
		r.Recorder.Event(receiver, corev1.EventTypeWarning, "GetFailed", "Failed to get ThanosReceive resource")
		return ctrl.Result{}, err
	}

	// handle object being deleted - inferred from the existence of DeletionTimestamp
	if !receiver.GetDeletionTimestamp().IsZero() {
		return r.handleDeletionTimestamp(receiver)
	}

	if receiver.Spec.Paused != nil {
		if *receiver.Spec.Paused {
			r.logger.Info("reconciliation is paused for ThanosReceive resource")
			r.Recorder.Event(receiver, corev1.EventTypeNormal, "Paused", "Reconciliation is paused for ThanosReceive resource")
			return ctrl.Result{}, nil
		}
	}

	err = r.syncResources(ctx, *receiver)
	if err != nil {
		r.ControllerBaseMetrics.ReconciliationsFailedTotal.WithLabelValues(manifestreceive.Name).Inc()
		r.Recorder.Event(receiver, corev1.EventTypeWarning, "SyncFailed", fmt.Sprintf("Failed to sync resources: %v", err))
		return ctrl.Result{}, err
	}

	return ctrl.Result{}, nil
}

// +kubebuilder:rbac:groups=monitoring.thanos.io,resources=thanosreceives,verbs=get;list;watch;create;update;patch;delete
// +kubebuilder:rbac:groups=monitoring.thanos.io,resources=thanosreceives/status,verbs=get;update;patch
// +kubebuilder:rbac:groups=monitoring.thanos.io,resources=thanosreceives/finalizers,verbs=update
// +kubebuilder:rbac:groups=apps,resources=statefulsets;deployments,verbs=get;list;watch;create;update;patch;delete
// +kubebuilder:rbac:groups="",resources=services;configmaps;serviceaccounts,verbs=get;list;watch;create;update;patch;delete
// +kubebuilder:rbac:groups="discovery.k8s.io",resources=endpointslices,verbs=get;list;watch

// SetupWithManager sets up the controller with the Manager.
func (r *ThanosReceiveReconciler) SetupWithManager(mgr ctrl.Manager) error {
	bld := ctrl.NewControllerManagedBy(mgr)
	err := r.buildController(*bld)
	if err != nil {
		r.Recorder.Event(&monitoringthanosiov1alpha1.ThanosReceive{}, corev1.EventTypeWarning, "SetupFailed", fmt.Sprintf("Failed to set up controller: %v", err))
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
	var objs []client.Object
	objs = append(objs, r.buildHashrings(receiver)...)

	hashringConf, err := r.buildHashringConfig(ctx, receiver)
	if err != nil {
		if !errors.Is(err, manifestreceive.ErrHashringsEmpty) {
			r.Recorder.Event(&receiver, corev1.EventTypeWarning, "HashringConfigBuildFailed", fmt.Sprintf("Failed to build hashring configuration: %v", err))
			return fmt.Errorf("failed to build hashring configuration: %w", err)
		}
		// we can create the config map even if there are no hashrings
		objs = append(objs, hashringConf)
	} else {
		objs = append(objs, hashringConf)
		// bring up the router components only if there are ready hashrings to avoid crash looping the router
		objs = append(objs, r.buildRouter(receiver)...)
	}

	var errCount int32
	for _, obj := range objs {
		if manifests.IsNamespacedResource(obj) {
			obj.SetNamespace(receiver.Namespace)
			if err := ctrl.SetControllerReference(&receiver, obj, r.Scheme); err != nil {
				r.logger.Error(err, "failed to set controller owner reference to resource")
				errCount++
				continue
			}
		}

		desired := obj.DeepCopyObject().(client.Object)
		mutateFn := manifests.MutateFuncFor(obj, desired)

		op, err := ctrl.CreateOrUpdate(ctx, r.Client, obj, mutateFn)
		if err != nil {
			r.logger.Error(
				err, "failed to create or update resource",
				"gvk", obj.GetObjectKind().GroupVersionKind().String(),
				"resource", obj.GetName(),
				"namespace", obj.GetNamespace(),
			)
			errCount++
			continue
		}

		r.logger.V(1).Info(
			"resource configured",
			"operation", op, "gvk", obj.GetObjectKind().GroupVersionKind().String(),
			"resource", obj.GetName(), "namespace", obj.GetNamespace(),
		)
	}

	if errCount > 0 {
		r.ControllerBaseMetrics.ClientErrorsTotal.WithLabelValues(manifestreceive.Name).Add(float64(errCount))
		return fmt.Errorf("failed to create or update %d resources for the hashrings", errCount)
	}

	return nil
}

// build hashring builds out the ingesters for the ThanosReceive resource.
func (r *ThanosReceiveReconciler) buildHashrings(receiver monitoringthanosiov1alpha1.ThanosReceive) []client.Object {
	opts := make([]manifestreceive.IngesterOptions, 0)
	baseLabels := receiver.GetLabels()
	baseSecret := receiver.Spec.Ingester.DefaultObjectStorageConfig.ToSecretKeySelector()

	for _, hashring := range receiver.Spec.Ingester.Hashrings {
		objStoreSecret := baseSecret
		if hashring.ObjectStorageConfig != nil {
			objStoreSecret = hashring.ObjectStorageConfig.ToSecretKeySelector()
		}

		metaOpts := manifests.Options{
			Name:      manifestreceive.IngesterNameFromParent(receiver.GetName(), hashring.Name),
			Namespace: receiver.GetNamespace(),
			Replicas:  hashring.Replicas,
			Labels:    manifests.MergeLabels(baseLabels, hashring.Labels),
			Image:     receiver.Spec.Image,
			LogLevel:  receiver.Spec.LogLevel,
			LogFormat: receiver.Spec.LogFormat,
		}.ApplyDefaults()

		opt := manifestreceive.IngesterOptions{
			Options: metaOpts,
			TSDBOpts: manifestreceive.TSDBOpts{
				Retention: string(hashring.TSDBConfig.Retention),
			},
			StorageSize:    resource.MustParse(string(hashring.StorageSize)),
			ObjStoreSecret: objStoreSecret,
			ExternalLabels: hashring.ExternalLabels,
		}

		opt.Additional = manifests.Additional{
			Args:         receiver.Spec.Ingester.Additional.Args,
			Containers:   receiver.Spec.Ingester.Additional.Containers,
			Volumes:      receiver.Spec.Ingester.Additional.Volumes,
			VolumeMounts: receiver.Spec.Ingester.Additional.VolumeMounts,
			Ports:        receiver.Spec.Ingester.Additional.Ports,
			Env:          receiver.Spec.Ingester.Additional.Env,
			ServicePorts: receiver.Spec.Ingester.Additional.ServicePorts,
		}

		opts = append(opts, opt)
	}

	return manifestreceive.BuildIngesters(opts)
}

// buildHashringConfig builds the hashring configuration for the ThanosReceive resource.
func (r *ThanosReceiveReconciler) buildHashringConfig(ctx context.Context, receiver monitoringthanosiov1alpha1.ThanosReceive) (client.Object, error) {
	cm := &corev1.ConfigMap{}
	err := r.Client.Get(ctx, client.ObjectKey{Namespace: receiver.GetNamespace(), Name: receiver.GetName()}, cm)
	if err != nil {
		if !apierrors.IsNotFound(err) {
			return nil, fmt.Errorf("failed to get config map for resource %s: %w", receiver.GetName(), err)
		}
	}

	opts := manifestreceive.HashringOptions{
		Options: manifests.Options{
			Name:      receiver.GetName(),
			Namespace: receiver.GetNamespace(),
			Labels:    receiver.GetLabels(),
		},
		DesiredReplicationFactor: receiver.Spec.Router.ReplicationFactor,
		HashringSettings:         make(map[string]manifestreceive.HashringMeta, len(receiver.Spec.Ingester.Hashrings)),
	}

	totalHashrings := len(receiver.Spec.Ingester.Hashrings)
	for i, hashring := range receiver.Spec.Ingester.Hashrings {
		labelValue := manifestreceive.IngesterNameFromParent(receiver.GetName(), hashring.Name)
		// kubernetes sets this label on the endpoint slices - we want to match the generated name
		selectorListOpt := client.MatchingLabels{discoveryv1.LabelServiceName: labelValue}

		eps := discoveryv1.EndpointSliceList{}
		if err = r.Client.List(ctx, &eps, selectorListOpt, client.InNamespace(receiver.GetNamespace())); err != nil {
			return nil, fmt.Errorf("failed to list endpoint slices for resource %s: %w", receiver.GetName(), err)
		}

		opts.HashringSettings[labelValue] = manifestreceive.HashringMeta{
			DesiredReplicasReplicas:  hashring.Replicas,
			OriginalName:             hashring.Name,
			Tenants:                  hashring.Tenants,
			TenantMatcherType:        manifestreceive.TenantMatcher(hashring.TenantMatcherType),
			AssociatedEndpointSlices: eps,
			// set the priority by slice order for now
			Priority: totalHashrings - i,
		}
	}

	r.thanosReceiveMetrics.HashringsConfigured.WithLabelValues(receiver.GetName(), receiver.GetNamespace()).Set(float64(totalHashrings))
	r.Recorder.Event(&receiver, corev1.EventTypeNormal, "HashringConfigBuilt", fmt.Sprintf("Built hashring configuration with %d hashrings", totalHashrings))
	return manifestreceive.BuildHashrings(r.logger, cm, opts)
}

// build hashring builds out the ingesters for the ThanosReceive resource.
func (r *ThanosReceiveReconciler) buildRouter(receiver monitoringthanosiov1alpha1.ThanosReceive) []client.Object {
	baseLabels := receiver.GetLabels()

	metaOpts := manifests.Options{
		Name:      receiver.GetName(),
		Namespace: receiver.GetNamespace(),
		Replicas:  receiver.Spec.Router.Replicas,
		Labels:    manifests.MergeLabels(baseLabels, receiver.Spec.Router.Labels),
		Image:     receiver.Spec.Image,
		LogLevel:  receiver.Spec.LogLevel,
		LogFormat: receiver.Spec.LogFormat,
	}.ApplyDefaults()

	opts := manifestreceive.RouterOptions{
		Options:           metaOpts,
		ReplicationFactor: receiver.Spec.Router.ReplicationFactor,
		ExternalLabels:    receiver.Spec.Router.ExternalLabels,
	}

	opts.Additional = manifests.Additional{
		Args:         receiver.Spec.Router.Additional.Args,
		Containers:   receiver.Spec.Router.Additional.Containers,
		Volumes:      receiver.Spec.Router.Additional.Volumes,
		VolumeMounts: receiver.Spec.Router.Additional.VolumeMounts,
		Ports:        receiver.Spec.Router.Additional.Ports,
		Env:          receiver.Spec.Router.Additional.Env,
		ServicePorts: receiver.Spec.Router.Additional.ServicePorts,
	}

	return manifestreceive.BuildRouter(opts)
}

func (r *ThanosReceiveReconciler) handleDeletionTimestamp(receiveHashring *monitoringthanosiov1alpha1.ThanosReceive) (ctrl.Result, error) {
	if controllerutil.ContainsFinalizer(receiveHashring, receiveFinalizer) {
		r.logger.Info("performing Finalizer Operations for ThanosReceiveHashring before delete CR")

		r.Recorder.Event(receiveHashring, "Warning", "Deleting",
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

		r.thanosReceiveMetrics.EndpointWatchesReconciliationsTotal.Inc()
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
