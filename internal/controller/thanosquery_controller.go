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

	appsv1 "k8s.io/api/apps/v1"
	corev1 "k8s.io/api/core/v1"
	apierrors "k8s.io/apimachinery/pkg/api/errors"
	"k8s.io/apimachinery/pkg/runtime"
	"k8s.io/client-go/tools/record"
	ctrl "sigs.k8s.io/controller-runtime"
	"sigs.k8s.io/controller-runtime/pkg/client"
	"sigs.k8s.io/controller-runtime/pkg/log"

	monitoringthanosiov1alpha1 "github.com/thanos-community/thanos-operator/api/v1alpha1"
	"github.com/thanos-community/thanos-operator/internal/pkg/manifests"
	manifestquery "github.com/thanos-community/thanos-operator/internal/pkg/manifests/query"
)

// ThanosQueryReconciler reconciles a ThanosQuery object
type ThanosQueryReconciler struct {
	client.Client
	Scheme   *runtime.Scheme
	Recorder record.EventRecorder
}

//+kubebuilder:rbac:groups=monitoring.thanos.io,resources=thanosqueries,verbs=get;list;watch;create;update;patch;delete
//+kubebuilder:rbac:groups=monitoring.thanos.io,resources=thanosqueries/status,verbs=get;update;patch
//+kubebuilder:rbac:groups=monitoring.thanos.io,resources=thanosqueries/finalizers,verbs=update
//+kubebuilder:rbac:groups=apps,resources=deployments,verbs=get;list;watch;create;update;patch;delete
//+kubebuilder:rbac:groups="",resources=services;configmaps;serviceaccounts,verbs=get;list;watch;create;update;patch;delete

// Reconcile is part of the main kubernetes reconciliation loop which aims to
// move the current state of the cluster closer to the desired state.
//
// For more details, check Reconcile and its Result here:
// - https://pkg.go.dev/sigs.k8s.io/controller-runtime@v0.17.0/pkg/reconcile
func (r *ThanosQueryReconciler) Reconcile(ctx context.Context, req ctrl.Request) (ctrl.Result, error) {
	logger := log.FromContext(ctx)

	query := &monitoringthanosiov1alpha1.ThanosQuery{}
	err := r.Get(ctx, req.NamespacedName, query)
	if err != nil {
		if apierrors.IsNotFound(err) {
			logger.Info("thanos query resource not found. ignoring since object may be deleted")
			return ctrl.Result{}, nil
		}
		logger.Error(err, "failed to get ThanosQuery")
		return ctrl.Result{}, err
	}

	err = r.syncResources(ctx, *query)
	if err != nil {
		return ctrl.Result{}, err
	}

	return ctrl.Result{}, nil
}

func (r *ThanosQueryReconciler) syncResources(ctx context.Context, query monitoringthanosiov1alpha1.ThanosQuery) error {
	logger := log.FromContext(ctx)
	var objs []client.Object

	objs = append(objs, r.buildQuerier(query)...)

	var errCount int32
	for _, obj := range objs {
		if manifests.IsNamespacedResource(obj) {
			obj.SetNamespace(query.Namespace)
			if err := ctrl.SetControllerReference(&query, obj, r.Scheme); err != nil {
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

func (r *ThanosQueryReconciler) buildQuerier(query monitoringthanosiov1alpha1.ThanosQuery) []client.Object {
	metaOpts := manifests.Options{
		Name:      query.GetName(),
		Namespace: query.GetNamespace(),
		Replicas:  query.Spec.Replicas,
		Labels:    query.GetLabels(),
		Image:     query.Spec.Image,
		LogLevel:  query.Spec.LogLevel,
		LogFormat: query.Spec.LogFormat,
	}.ApplyDefaults()

	return manifestquery.BuildQuerier(manifestquery.QuerierOptions{
		Options:       metaOpts,
		ReplicaLabels: query.Spec.QuerierReplicaLabels,
		Timeout:       "15m",
		LookbackDelta: "5m",
		MaxConcurrent: 20,

		// TODO(saswatamcode): Add logic to for automated tracking of endpoints.
		Endpoints: []manifestquery.Endpoint{{
			ServiceName: "thanos-receive",
			Namespace:   query.GetNamespace(),
			Type:        manifestquery.Regular,
		}},
	})
}

// SetupWithManager sets up the controller with the Manager.
func (r *ThanosQueryReconciler) SetupWithManager(mgr ctrl.Manager) error {
	return ctrl.NewControllerManagedBy(mgr).
		For(&monitoringthanosiov1alpha1.ThanosQuery{}).
		Owns(&corev1.ConfigMap{}).
		Owns(&corev1.ServiceAccount{}).
		Owns(&corev1.Service{}).
		Owns(&appsv1.Deployment{}).
		Complete(r)
}
