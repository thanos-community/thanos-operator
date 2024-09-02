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

	"github.com/thanos-community/thanos-operator/internal/pkg/controllers_metrics"

	"github.com/go-logr/logr"
	"github.com/prometheus/client_golang/prometheus"
	monitoringthanosiov1alpha1 "github.com/thanos-community/thanos-operator/api/v1alpha1"
	"github.com/thanos-community/thanos-operator/internal/pkg/manifests"
	manifestquery "github.com/thanos-community/thanos-operator/internal/pkg/manifests/query"

	appsv1 "k8s.io/api/apps/v1"
	corev1 "k8s.io/api/core/v1"
	apierrors "k8s.io/apimachinery/pkg/api/errors"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/labels"
	"k8s.io/apimachinery/pkg/runtime"
	"k8s.io/apimachinery/pkg/types"
	"k8s.io/client-go/tools/record"

	ctrl "sigs.k8s.io/controller-runtime"
	"sigs.k8s.io/controller-runtime/pkg/builder"
	"sigs.k8s.io/controller-runtime/pkg/client"
	"sigs.k8s.io/controller-runtime/pkg/handler"
	"sigs.k8s.io/controller-runtime/pkg/predicate"
	"sigs.k8s.io/controller-runtime/pkg/reconcile"
)

const (
	queryComponent = "query"
)

// ThanosQueryReconciler reconciles a ThanosQuery object
type ThanosQueryReconciler struct {
	client.Client
	Scheme   *runtime.Scheme
	Recorder record.EventRecorder

	logger logr.Logger

	reg                   prometheus.Registerer
	ControllerBaseMetrics *controllers_metrics.BaseMetrics
	thanosQueryMetrics    controllers_metrics.ThanosQueryMetrics
}

// NewThanosQueryReconciler returns a reconciler for ThanosQuery resources.
func NewThanosQueryReconciler(logger logr.Logger, client client.Client, scheme *runtime.Scheme, recorder record.EventRecorder, reg prometheus.Registerer, controllerBaseMetrics *controllers_metrics.BaseMetrics) *ThanosQueryReconciler {
	return &ThanosQueryReconciler{
		Client:   client,
		Scheme:   scheme,
		Recorder: recorder,

		logger:                logger,
		reg:                   reg,
		ControllerBaseMetrics: controllerBaseMetrics,
		thanosQueryMetrics:    controllers_metrics.NewThanosQueryMetrics(reg),
	}
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
	r.ControllerBaseMetrics.ReconciliationsTotal.WithLabelValues(queryComponent).Inc()

	query := &monitoringthanosiov1alpha1.ThanosQuery{}
	err := r.Get(ctx, req.NamespacedName, query)
	if err != nil {
		r.ControllerBaseMetrics.ClientErrorsTotal.WithLabelValues(queryComponent).Inc()
		if apierrors.IsNotFound(err) {
			r.logger.Info("thanos query resource not found. ignoring since object may be deleted")
			return ctrl.Result{}, nil
		}
		r.logger.Error(err, "failed to get ThanosQuery")
		r.ControllerBaseMetrics.ReconciliationsFailedTotal.WithLabelValues(queryComponent).Inc()
		return ctrl.Result{}, err
	}

	if query.Spec.Paused != nil {
		if *query.Spec.Paused {
			r.logger.Info("reconciliation is paused for ThanosQuery resource")
			return ctrl.Result{}, nil
		}
	}

	err = r.syncResources(ctx, *query)
	if err != nil {
		r.ControllerBaseMetrics.ReconciliationsFailedTotal.WithLabelValues(queryComponent).Inc()
		return ctrl.Result{}, err
	}

	return ctrl.Result{}, nil
}

func (r *ThanosQueryReconciler) syncResources(ctx context.Context, query monitoringthanosiov1alpha1.ThanosQuery) error {
	var objs []client.Object

	desiredObjs := r.buildQuerier(ctx, query)
	objs = append(objs, desiredObjs...)

	var errCount int32
	for _, obj := range objs {
		if manifests.IsNamespacedResource(obj) {
			obj.SetNamespace(query.Namespace)
			if err := ctrl.SetControllerReference(&query, obj, r.Scheme); err != nil {
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
		r.ControllerBaseMetrics.ClientErrorsTotal.WithLabelValues(queryComponent).Add(float64(errCount))
		return fmt.Errorf("failed to create or update %d resources for the querier", errCount)
	}

	return nil
}

func (r *ThanosQueryReconciler) buildQuerier(ctx context.Context, query monitoringthanosiov1alpha1.ThanosQuery) []client.Object {
	metaOpts := manifests.Options{
		Name:      query.GetName(),
		Namespace: query.GetNamespace(),
		Replicas:  query.Spec.Replicas,
		Labels:    manifests.MergeLabels(query.GetLabels(), query.Spec.Labels),
		Image:     query.Spec.Image,
		LogLevel:  query.Spec.LogLevel,
		LogFormat: query.Spec.LogFormat,
	}.ApplyDefaults()

	endpoints := r.getStoreAPIServiceEndpoints(ctx, query)

	additional := manifests.Additional{
		Args:         query.Spec.Additional.Args,
		Containers:   query.Spec.Additional.Containers,
		Volumes:      query.Spec.Additional.Volumes,
		VolumeMounts: query.Spec.Additional.VolumeMounts,
		Ports:        query.Spec.Additional.Ports,
		Env:          query.Spec.Additional.Env,
		ServicePorts: query.Spec.Additional.ServicePorts,
	}

	return manifestquery.BuildQuerier(manifestquery.QuerierOptions{
		Options:       metaOpts,
		ReplicaLabels: query.Spec.QuerierReplicaLabels,
		Timeout:       "15m",
		LookbackDelta: "5m",
		MaxConcurrent: 20,
		Endpoints:     endpoints,
		Additional:    additional,
	})
}

// getStoreAPIServiceEndpoints returns the list of endpoints for the StoreAPI services that match the ThanosQuery storeLabelSelector.
func (r *ThanosQueryReconciler) getStoreAPIServiceEndpoints(ctx context.Context, query monitoringthanosiov1alpha1.ThanosQuery) []manifestquery.Endpoint {
	services := &corev1.ServiceList{}
	listOpts := []client.ListOption{
		client.MatchingLabels(query.Spec.StoreLabelSelector.MatchLabels),
		client.InNamespace(query.Namespace),
	}
	if err := r.List(ctx, services, listOpts...); err != nil {
		return []manifestquery.Endpoint{}
	}

	if len(services.Items) == 0 {
		return []manifestquery.Endpoint{}
	}

	endpoints := make([]manifestquery.Endpoint, len(services.Items))
	for i, svc := range services.Items {
		etype := manifestquery.RegularLabel

		if metav1.HasLabel(svc.ObjectMeta, string(manifestquery.StrictLabel)) {
			etype = manifestquery.StrictLabel
		} else if metav1.HasLabel(svc.ObjectMeta, string(manifestquery.GroupStrictLabel)) {
			etype = manifestquery.GroupStrictLabel
		} else if metav1.HasLabel(svc.ObjectMeta, string(manifestquery.GroupLabel)) {
			etype = manifestquery.GroupLabel
		}

		for _, port := range svc.Spec.Ports {
			if port.Name == "grpc" {
				endpoints[i].Port = port.Port
				break
			}
		}

		r.thanosQueryMetrics.EndpointsConfigured.WithLabelValues(string(etype), query.GetName(), query.GetNamespace()).Inc()
		endpoints[i] = manifestquery.Endpoint{
			ServiceName: svc.GetName(),
			Namespace:   svc.GetNamespace(),
			Type:        etype,
		}
	}

	return endpoints
}

// SetupWithManager sets up the controller with the Manager.
func (r *ThanosQueryReconciler) SetupWithManager(mgr ctrl.Manager) error {
	servicePredicate, err := predicate.LabelSelectorPredicate(metav1.LabelSelector{
		MatchLabels: map[string]string{
			manifests.PartOfLabel:          manifests.DefaultPartOfLabel,
			manifests.DefaultStoreAPILabel: manifests.DefaultStoreAPIValue,
		},
	})
	if err != nil {
		return err
	}

	return ctrl.NewControllerManagedBy(mgr).
		For(&monitoringthanosiov1alpha1.ThanosQuery{}).
		Owns(&corev1.ConfigMap{}).
		Owns(&corev1.ServiceAccount{}).
		Owns(&corev1.Service{}).
		Owns(&appsv1.Deployment{}).
		Watches(
			&corev1.Service{},
			r.enqueueForService(),
			builder.WithPredicates(predicate.GenerationChangedPredicate{}, servicePredicate),
		).
		Complete(r)
}

// enqueueForService returns an EventHandler that will enqueue a request for the ThanosQuery instances
// that matches the Service.
func (r *ThanosQueryReconciler) enqueueForService() handler.EventHandler {
	return handler.EnqueueRequestsFromMapFunc(func(ctx context.Context, obj client.Object) []reconcile.Request {
		if obj.GetLabels()[manifests.DefaultStoreAPILabel] != manifests.DefaultStoreAPIValue {
			return nil
		}

		listOpts := []client.ListOption{
			client.InNamespace(obj.GetNamespace()),
		}

		queriers := &monitoringthanosiov1alpha1.ThanosQueryList{}
		err := r.List(ctx, queriers, listOpts...)
		if err != nil {
			return []reconcile.Request{}
		}

		requests := []reconcile.Request{}
		for _, query := range queriers.Items {
			if labels.SelectorFromSet(query.Spec.StoreLabelSelector.MatchLabels).Matches(labels.Set(obj.GetLabels())) {
				requests = append(requests, reconcile.Request{
					NamespacedName: types.NamespacedName{
						Name:      query.GetName(),
						Namespace: query.GetNamespace(),
					},
				})
			}
		}

		r.thanosQueryMetrics.ServiceWatchesReconciliationsTotal.Add(float64(len(requests)))
		return requests
	})
}
