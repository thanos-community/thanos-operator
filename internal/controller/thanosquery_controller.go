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
	manifestquery "github.com/thanos-community/thanos-operator/internal/pkg/manifests/query"
	manifestsstore "github.com/thanos-community/thanos-operator/internal/pkg/manifests/store"
	controllermetrics "github.com/thanos-community/thanos-operator/internal/pkg/metrics"

	appsv1 "k8s.io/api/apps/v1"
	corev1 "k8s.io/api/core/v1"
	policyv1 "k8s.io/api/policy/v1"
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

// ThanosQueryReconciler reconciles a ThanosQuery object
type ThanosQueryReconciler struct {
	client.Client
	Scheme *runtime.Scheme

	logger   logr.Logger
	metrics  controllermetrics.ThanosQueryMetrics
	recorder record.EventRecorder

	handler *handlers.Handler
}

// NewThanosQueryReconciler returns a reconciler for ThanosQuery resources.
func NewThanosQueryReconciler(conf Config, client client.Client, scheme *runtime.Scheme) *ThanosQueryReconciler {
	handler := handlers.NewHandler(client, scheme, conf.InstrumentationConfig.Logger)
	featureGates := conf.FeatureGate.ToGVK()
	if len(featureGates) > 0 {
		handler.SetFeatureGates(featureGates)
	}

	return &ThanosQueryReconciler{
		Client:   client,
		Scheme:   scheme,
		logger:   conf.InstrumentationConfig.Logger,
		metrics:  controllermetrics.NewThanosQueryMetrics(conf.InstrumentationConfig.MetricsRegistry, conf.InstrumentationConfig.BaseMetrics),
		recorder: conf.InstrumentationConfig.EventRecorder,
		handler:  handler,
	}
}

//+kubebuilder:rbac:groups=monitoring.thanos.io,resources=thanosqueries,verbs=get;list;watch;create;update;patch;delete
//+kubebuilder:rbac:groups=monitoring.thanos.io,resources=thanosqueries/status,verbs=get;update;patch
//+kubebuilder:rbac:groups=monitoring.thanos.io,resources=thanosqueries/finalizers,verbs=update
//+kubebuilder:rbac:groups=apps,resources=deployments,verbs=get;list;watch;create;update;patch;delete
//+kubebuilder:rbac:groups="",resources=services;configmaps;serviceaccounts,verbs=get;list;watch;create;update;patch;delete
//+kubebuilder:rbac:groups=monitoring.coreos.com,resources=servicemonitors,verbs=get;list;watch;create;update;patch;delete
// +kubebuilder:rbac:groups=policy,resources=poddisruptionbudgets,verbs=get;list;watch;create;update

// Reconcile is part of the main kubernetes reconciliation loop which aims to
// move the current state of the cluster closer to the desired state.
//
// For more details, check Reconcile and its Result here:
// - https://pkg.go.dev/sigs.k8s.io/controller-runtime@v0.17.0/pkg/reconcile
func (r *ThanosQueryReconciler) Reconcile(ctx context.Context, req ctrl.Request) (ctrl.Result, error) {
	r.metrics.ReconciliationsTotal.WithLabelValues(manifestquery.Name).Inc()

	query := &monitoringthanosiov1alpha1.ThanosQuery{}
	err := r.Get(ctx, req.NamespacedName, query)
	if err != nil {
		r.metrics.ClientErrorsTotal.WithLabelValues(manifestquery.Name).Inc()
		if apierrors.IsNotFound(err) {
			r.logger.Info("thanos query resource not found. ignoring since object may be deleted")
			return ctrl.Result{}, nil
		}
		r.logger.Error(err, "failed to get ThanosQuery")
		r.metrics.ReconciliationsFailedTotal.WithLabelValues(manifestquery.Name).Inc()
		r.recorder.Event(query, corev1.EventTypeWarning, "GetFailed", "Failed to get ThanosQuery resource")
		return ctrl.Result{}, err
	}

	if query.Spec.Paused != nil && *query.Spec.Paused {
		r.logger.Info("reconciliation is paused for ThanosQuery resource")
		r.recorder.Event(query, corev1.EventTypeNormal, "Paused", "Reconciliation is paused for ThanosQuery resource")
		return ctrl.Result{}, nil
	}

	err = r.syncResources(ctx, *query)
	if err != nil {
		r.metrics.ReconciliationsFailedTotal.WithLabelValues(manifestquery.Name).Inc()
		r.recorder.Event(query, corev1.EventTypeWarning, "SyncFailed", fmt.Sprintf("Failed to sync resources: %v", err))
		return ctrl.Result{}, err
	}

	return ctrl.Result{}, nil
}

func (r *ThanosQueryReconciler) syncResources(ctx context.Context, query monitoringthanosiov1alpha1.ThanosQuery) error {
	var objs []client.Object

	querierObjs, err := r.buildQuery(ctx, query)
	if err != nil {
		return err
	}

	objs = append(objs, querierObjs...)

	if query.Spec.QueryFrontend != nil {
		r.recorder.Event(&query, corev1.EventTypeNormal, "BuildingQueryFrontend", "Building Query Frontend resources")
		frontendObjs := r.buildQueryFrontend(query)
		objs = append(objs, frontendObjs...)
	}

	var errCount int
	if errCount := r.handler.CreateOrUpdate(ctx, query.GetNamespace(), &query, objs); errCount > 0 {
		r.metrics.ClientErrorsTotal.WithLabelValues(manifestquery.Name).Add(float64(errCount))
		return fmt.Errorf("failed to create or update %d resources for the querier and query frontend", errCount)
	}

	if query.Spec.ServiceMonitorConfig != nil && query.Spec.ServiceMonitorConfig.Enable != nil && !*query.Spec.ServiceMonitorConfig.Enable {
		if errCount = r.handler.DeleteResource(ctx, []client.Object{&monitoringv1.ServiceMonitor{
			ObjectMeta: metav1.ObjectMeta{
				Name:      manifestquery.Options{Options: manifests.Options{Owner: query.GetName()}}.GetGeneratedResourceName(),
				Namespace: query.GetNamespace(),
			},
		},
		}); errCount > 0 {
			r.metrics.ClientErrorsTotal.WithLabelValues(manifestquery.Name).Add(float64(errCount))
			return fmt.Errorf("failed to delete %d resources for the querier and query frontend", errCount)
		}
	}

	return nil
}

func (r *ThanosQueryReconciler) buildQuery(ctx context.Context, query monitoringthanosiov1alpha1.ThanosQuery) ([]client.Object, error) {
	endpoints, err := r.getStoreAPIServiceEndpoints(ctx, query)
	if err != nil {
		return nil, err
	}

	opts := queryV1Alpha1ToOptions(query)
	opts.Endpoints = endpoints

	return opts.Build(), nil
}

// getStoreAPIServiceEndpoints returns the list of endpoints for the StoreAPI services that match the ThanosQuery storeLabelSelector.
func (r *ThanosQueryReconciler) getStoreAPIServiceEndpoints(ctx context.Context, query monitoringthanosiov1alpha1.ThanosQuery) ([]manifestquery.Endpoint, error) {
	labelSelector, err := manifests.BuildLabelSelectorFrom(query.Spec.StoreLabelSelector, requiredStoreServiceLabels)
	if err != nil {
		return []manifestquery.Endpoint{}, err
	}
	services := &corev1.ServiceList{}
	listOpts := []client.ListOption{
		client.MatchingLabelsSelector{Selector: labelSelector},
		client.InNamespace(query.Namespace),
	}
	if err := r.List(ctx, services, listOpts...); err != nil {
		return []manifestquery.Endpoint{}, err
	}

	if len(services.Items) == 0 {
		r.recorder.Event(&query, corev1.EventTypeWarning, "NoEndpointsFound", "No StoreAPI services found")
		return []manifestquery.Endpoint{}, nil
	}

	endpoints := make([]manifestquery.Endpoint, len(services.Items))
	for i, svc := range services.Items {

		port, ok := manifests.IsGrpcServiceWithLabels(&svc, requiredStoreServiceLabels)
		if !ok {
			r.logger.Error(fmt.Errorf(
				"service %s/%s is missing required gRPC port", svc.GetNamespace(), svc.GetName()),
				"failed to get gRPC port for service",
			)
			continue
		}

		etype := r.getServiceTypeFromLabel(svc.ObjectMeta)

		endpoints[i] = manifestquery.Endpoint{
			ServiceName: svc.GetName(),
			Port:        port,
			Namespace:   svc.GetNamespace(),
			Type:        etype,
		}
		r.metrics.EndpointsConfigured.WithLabelValues(string(etype), query.GetName(), query.GetNamespace()).Inc()
	}

	return endpoints, nil
}

func (r *ThanosQueryReconciler) buildQueryFrontend(query monitoringthanosiov1alpha1.ThanosQuery) []client.Object {
	return queryV1Alpha1ToQueryFrontEndOptions(query).Build()
}

// SetupWithManager sets up the controller with the Manager.
func (r *ThanosQueryReconciler) SetupWithManager(mgr ctrl.Manager) error {
	servicePredicate, err := predicate.LabelSelectorPredicate(metav1.LabelSelector{
		MatchLabels: requiredStoreServiceLabels,
	})
	if err != nil {
		return err
	}

	withLabelChangedPredicate := predicate.And(servicePredicate, predicate.LabelChangedPredicate{})
	withGenerationChangePredicate := predicate.And(servicePredicate, predicate.GenerationChangedPredicate{}, servicePredicate)
	withPredicate := predicate.Or(withLabelChangedPredicate, withGenerationChangePredicate)

	err = ctrl.NewControllerManagedBy(mgr).
		For(&monitoringthanosiov1alpha1.ThanosQuery{}).
		Owns(&corev1.ConfigMap{}).
		Owns(&corev1.ServiceAccount{}).
		Owns(&corev1.Service{}).
		Owns(&appsv1.Deployment{}).
		Owns(&policyv1.PodDisruptionBudget{}).
		Owns(&monitoringv1.ServiceMonitor{}).
		Watches(
			&corev1.Service{},
			r.enqueueForService(),
			builder.WithPredicates(withPredicate),
		).
		Complete(r)

	// if servicemonitor CRD exists in the cluster, watch for changes to ServiceMonitor resources
	if err != nil {
		r.recorder.Event(&monitoringthanosiov1alpha1.ThanosQuery{}, corev1.EventTypeWarning, "SetupFailed", fmt.Sprintf("Failed to set up controller: %v", err))
		return err
	}

	return nil
}

// enqueueForService returns an EventHandler that will enqueue a request for the ThanosQuery instances
// that matches the Service.
func (r *ThanosQueryReconciler) enqueueForService() handler.EventHandler {
	return handler.EnqueueRequestsFromMapFunc(func(ctx context.Context, obj client.Object) []reconcile.Request {
		if !r.isQueueableStoreService(obj) {
			return []reconcile.Request{}
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
			selector, err := manifests.BuildLabelSelectorFrom(query.Spec.StoreLabelSelector, requiredStoreServiceLabels)
			if err != nil {
				r.logger.Error(err, "failed to build label selector from store label selector", "query", query.GetName())
				continue
			}

			if selector.Matches(labels.Set(obj.GetLabels())) {
				requests = append(requests, reconcile.Request{
					NamespacedName: types.NamespacedName{
						Name:      query.GetName(),
						Namespace: query.GetNamespace(),
					},
				})
			}
		}

		r.metrics.ServiceWatchesReconciliationsTotal.Add(float64(len(requests)))
		return requests
	})
}

// isQueueableStoreService returns true if the Service is a StoreAPI service that is part of a 'thanos' and has a gRPC port.
func (r *ThanosQueryReconciler) isQueueableStoreService(obj client.Object) bool {
	_, ok := manifests.IsGrpcServiceWithLabels(obj, requiredStoreServiceLabels)
	return ok
}

func (r *ThanosQueryReconciler) getServiceTypeFromLabel(objMeta metav1.ObjectMeta) manifestquery.EndpointType {
	etype := manifestquery.RegularLabel

	if metav1.HasLabel(objMeta, string(manifestquery.StrictLabel)) {
		etype = manifestquery.StrictLabel
	} else if metav1.HasLabel(objMeta, string(manifestquery.GroupStrictLabel)) {
		etype = manifestquery.GroupStrictLabel
	} else if metav1.HasLabel(objMeta, string(manifestquery.GroupLabel)) {
		etype = manifestquery.GroupLabel
	}
	return etype
}

var requiredStoreServiceLabels = manifestsstore.GetRequiredStoreServiceLabel()
