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

	monitoringthanosiov1alpha1 "github.com/thanos-community/thanos-operator/api/v1alpha1"
	"github.com/thanos-community/thanos-operator/internal/pkg/manifests"
	"github.com/thanos-community/thanos-operator/internal/pkg/manifests/receive"

	"github.com/go-logr/logr"

	appsv1 "k8s.io/api/apps/v1"
	corev1 "k8s.io/api/core/v1"
	apierrors "k8s.io/apimachinery/pkg/api/errors"
	"k8s.io/apimachinery/pkg/api/resource"
	"k8s.io/apimachinery/pkg/runtime"
	"k8s.io/client-go/tools/record"

	ctrl "sigs.k8s.io/controller-runtime"
	"sigs.k8s.io/controller-runtime/pkg/builder"
	"sigs.k8s.io/controller-runtime/pkg/client"
	"sigs.k8s.io/controller-runtime/pkg/controller/controllerutil"
	"sigs.k8s.io/controller-runtime/pkg/log"
)

const (
	receiveFinalizer = "monitoring.thanos.io/receive-finalizer"
)

// ThanosReceiveReconciler reconciles a ThanosReceive object
type ThanosReceiveReconciler struct {
	client.Client
	Scheme   *runtime.Scheme
	Recorder record.EventRecorder
}

// Reconcile is part of the main kubernetes reconciliation loop which aims to
// move the current state of the cluster closer to the desired state.
// For more details, check Reconcile and its Result here:
// - https://pkg.go.dev/sigs.k8s.io/controller-runtime@v0.17.3/pkg/reconcile
func (r *ThanosReceiveReconciler) Reconcile(ctx context.Context, req ctrl.Request) (ctrl.Result, error) {
	logger := log.FromContext(ctx)

	// Fetch the ThanosReceive instance to validate it is applied on the cluster.
	receiver := &monitoringthanosiov1alpha1.ThanosReceive{}
	err := r.Get(ctx, req.NamespacedName, receiver)
	if err != nil {
		if apierrors.IsNotFound(err) {
			logger.Info("thanos receive resource not found. ignoring since object may be deleted")
			return ctrl.Result{}, nil
		}
		logger.Error(err, "failed to get ThanosReceive")
		return ctrl.Result{}, err
	}

	// handle object being deleted - inferred from the existence of DeletionTimestamp
	if !receiver.GetDeletionTimestamp().IsZero() {
		return r.handleDeletionTimestamp(logger, receiver)
	}

	err = r.syncResources(ctx, *receiver)
	if err != nil {
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
	return r.buildController(*bld)
}

// buildController sets up the controller with the Manager.
func (r *ThanosReceiveReconciler) buildController(bld builder.Builder) error {
	bld.
		For(&monitoringthanosiov1alpha1.ThanosReceive{}).
		Owns(&corev1.ConfigMap{}).
		Owns(&corev1.ServiceAccount{}).
		Owns(&corev1.Service{}).
		Owns(&appsv1.Deployment{}).
		Owns(&appsv1.StatefulSet{})

	return bld.Complete(r)
}

// syncResources syncs the resources for the ThanosReceive resource.
// It creates or updates the resources for the hashrings and the router.
func (r *ThanosReceiveReconciler) syncResources(ctx context.Context, receive monitoringthanosiov1alpha1.ThanosReceive) error {
	var objs []client.Object
	logger := log.FromContext(ctx)

	objs = append(objs, r.buildHashrings(receive)...)

	var errCount int32
	for _, obj := range objs {
		if manifests.IsNamespacedResource(obj) {
			obj.SetNamespace(receive.Namespace)
			if err := ctrl.SetControllerReference(&receive, obj, r.Scheme); err != nil {
				logger.Error(err, "failed to set controller owner reference to resource")
				errCount++
				continue
			}
		}

		desired := obj.DeepCopyObject().(client.Object)
		mutateFn := manifests.MutateFuncFor(obj, desired)

		op, err := ctrl.CreateOrUpdate(ctx, r.Client, obj, mutateFn)
		if err != nil {
			logger.Error(
				err, "failed to create or update resource",
				"gvk", obj.GetObjectKind().GroupVersionKind().String(),
				"resource", obj.GetName(),
				"namespace", obj.GetNamespace(),
			)
			errCount++
			continue
		}

		logger.V(1).Info(
			"resource configured",
			"operation", op, "gvk", obj.GetObjectKind().GroupVersionKind().String(),
			"resource", obj.GetName(), "namespace", obj.GetNamespace(),
		)
	}

	if errCount > 0 {
		return fmt.Errorf("failed to create or update %d resources for the hashrings", errCount)
	}

	return nil
}

// build hashring builds out the ingesters for the ThanosReceive resource.
func (r *ThanosReceiveReconciler) buildHashrings(receiver monitoringthanosiov1alpha1.ThanosReceive) []client.Object {
	opts := make([]receive.IngesterOptions, 0)
	baseLabels := receiver.GetLabels()
	baseSecret := receiver.Spec.Ingester.DefaultObjectStorageConfig.ToSecretKeySelector()

	for _, hashring := range receiver.Spec.Ingester.Hashrings {
		objStoreSecret := baseSecret
		if hashring.ObjectStorageConfig != nil {
			objStoreSecret = hashring.ObjectStorageConfig.ToSecretKeySelector()
		}

		metaOpts := manifests.Options{
			Name:      receive.IngesterNameFromParent(receiver.GetName(), hashring.Name),
			Namespace: receiver.GetNamespace(),
			Replicas:  hashring.Replicas,
			Labels:    manifests.MergeLabels(baseLabels, hashring.Labels),
			Image:     receiver.Spec.Image,
			LogLevel:  receiver.Spec.LogLevel,
			LogFormat: receiver.Spec.LogFormat,
		}.ApplyDefaults()

		opt := receive.IngesterOptions{
			Options:        metaOpts,
			Retention:      string(*hashring.Retention),
			StorageSize:    resource.MustParse(hashring.StorageSize),
			ObjStoreSecret: objStoreSecret,
		}
		opts = append(opts, opt)
	}

	return receive.BuildIngesters(opts)
}

func (r *ThanosReceiveReconciler) handleDeletionTimestamp(log logr.Logger, receiveHashring *monitoringthanosiov1alpha1.ThanosReceive) (ctrl.Result, error) {
	if controllerutil.ContainsFinalizer(receiveHashring, receiveFinalizer) {
		log.Info("performing Finalizer Operations for ThanosReceiveHashring before delete CR")

		r.Recorder.Event(receiveHashring, "Warning", "Deleting",
			fmt.Sprintf("Custom Resource %s is being deleted from the namespace %s",
				receiveHashring.Name,
				receiveHashring.Namespace))
	}
	return ctrl.Result{}, nil
}
