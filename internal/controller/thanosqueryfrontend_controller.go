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
	"github.com/prometheus/client_golang/prometheus"
	monitoringthanosiov1alpha1 "github.com/thanos-community/thanos-operator/api/v1alpha1"
	"github.com/thanos-community/thanos-operator/internal/pkg/manifests"
	manifestqueryfrontend "github.com/thanos-community/thanos-operator/internal/pkg/manifests/queryfrontend"
	controllermetrics "github.com/thanos-community/thanos-operator/internal/pkg/metrics"

	appsv1 "k8s.io/api/apps/v1"
	corev1 "k8s.io/api/core/v1"
	apierrors "k8s.io/apimachinery/pkg/api/errors"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
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

// ThanosQueryFrontendReconciler reconciles a ThanosQueryFrontend object
type ThanosQueryFrontendReconciler struct {
	client.Client
	Scheme   *runtime.Scheme
	Recorder record.EventRecorder

	logger logr.Logger

	reg                        prometheus.Registerer
	ControllerBaseMetrics      *controllermetrics.BaseMetrics
	ThanosQueryFrontendMetrics controllermetrics.ThanosQueryFrontendMetrics
}

// NewThanosQueryFrontendReconciler returns a reconciler for ThanosQueryFrontend resources.
func NewThanosQueryFrontendReconciler(logger logr.Logger, client client.Client, scheme *runtime.Scheme, recorder record.EventRecorder, reg prometheus.Registerer, controllerBaseMetrics *controllermetrics.BaseMetrics) *ThanosQueryFrontendReconciler {
	return &ThanosQueryFrontendReconciler{
		Client:   client,
		Scheme:   scheme,
		Recorder: recorder,

		logger:                     logger,
		reg:                        reg,
		ControllerBaseMetrics:      controllerBaseMetrics,
		ThanosQueryFrontendMetrics: controllermetrics.NewThanosQueryFrontendMetrics(reg),
	}
}

//+kubebuilder:rbac:groups=monitoring.thanos.io,resources=thanosqueryfrontends,verbs=get;list;watch;create;update;patch;delete
//+kubebuilder:rbac:groups=monitoring.thanos.io,resources=thanosqueryfrontends/status,verbs=get;update;patch
//+kubebuilder:rbac:groups=monitoring.thanos.io,resources=thanosqueryfrontends/finalizers,verbs=update
//+kubebuilder:rbac:groups=apps,resources=deployments,verbs=get;list;watch;create;update;patch;delete
//+kubebuilder:rbac:groups="",resources=services;configmaps;serviceaccounts,verbs=get;list;watch;create;update;patch;delete

// Reconcile is part of the main kubernetes reconciliation loop which aims to
// move the current state of the cluster closer to the desired state.
func (r *ThanosQueryFrontendReconciler) Reconcile(ctx context.Context, req ctrl.Request) (ctrl.Result, error) {
	r.ControllerBaseMetrics.ReconciliationsTotal.WithLabelValues(manifestqueryfrontend.Name).Inc()

	frontend := &monitoringthanosiov1alpha1.ThanosQueryFrontend{}
	err := r.Get(ctx, req.NamespacedName, frontend)
	if err != nil {
		r.ControllerBaseMetrics.ClientErrorsTotal.WithLabelValues(manifestqueryfrontend.Name).Inc()
		if apierrors.IsNotFound(err) {
			r.logger.Info("thanos query frontend resource not found. ignoring since object may be deleted")
			return ctrl.Result{}, nil
		}
		r.logger.Error(err, "failed to get ThanosQueryFrontend")
		r.ControllerBaseMetrics.ReconciliationsFailedTotal.WithLabelValues(manifestqueryfrontend.Name).Inc()
		return ctrl.Result{}, err
	}

	if frontend.Spec.Paused != nil {
		if *frontend.Spec.Paused {
			r.logger.Info("reconciliation is paused for ThanosQueryFrontend resource")
			return ctrl.Result{}, nil
		}
	}

	err = r.syncResources(ctx, *frontend)
	if err != nil {
		r.ControllerBaseMetrics.ReconciliationsFailedTotal.WithLabelValues(manifestqueryfrontend.Name).Inc()
		return ctrl.Result{}, err
	}

	return ctrl.Result{}, nil
}

func (r *ThanosQueryFrontendReconciler) syncResources(ctx context.Context, frontend monitoringthanosiov1alpha1.ThanosQueryFrontend) error {
	var objs []client.Object

	desiredObjs, err := r.buildQueryFrontend(ctx, frontend)
	if err != nil {
		return err
	}
	objs = append(objs, desiredObjs...)

	var errCount int32
	for _, obj := range objs {
		if manifests.IsNamespacedResource(obj) {
			obj.SetNamespace(frontend.Namespace)
			if err := ctrl.SetControllerReference(&frontend, obj, r.Scheme); err != nil {
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
		r.ControllerBaseMetrics.ClientErrorsTotal.WithLabelValues(manifestqueryfrontend.Name).Add(float64(errCount))
		return fmt.Errorf("failed to create or update %d resources for the query frontend", errCount)
	}

	return nil
}

func (r *ThanosQueryFrontendReconciler) buildQueryFrontend(ctx context.Context, frontend monitoringthanosiov1alpha1.ThanosQueryFrontend) ([]client.Object, error) {
	metaOpts := manifests.Options{
		Name:      frontend.GetName(),
		Namespace: frontend.GetNamespace(),
		Replicas:  frontend.Spec.Replicas,
		Labels:    frontend.GetLabels(),
		Image:     frontend.Spec.Image,
		LogLevel:  frontend.Spec.LogLevel,
		LogFormat: frontend.Spec.LogFormat,
	}.ApplyDefaults()

	queryService, queryPort, err := r.getQueryService(ctx, frontend)
	if err != nil {
		return nil, err
	}

	additional := manifests.Additional{
		Args:         frontend.Spec.Additional.Args,
		Containers:   frontend.Spec.Additional.Containers,
		Volumes:      frontend.Spec.Additional.Volumes,
		VolumeMounts: frontend.Spec.Additional.VolumeMounts,
		Ports:        frontend.Spec.Additional.Ports,
		Env:          frontend.Spec.Additional.Env,
		ServicePorts: frontend.Spec.Additional.ServicePorts,
	}

	return manifestqueryfrontend.BuildQueryFrontend(manifestqueryfrontend.QueryFrontendOptions{
		Options:                metaOpts,
		QueryService:           queryService,
		QueryPort:              queryPort,
		Additional:             additional,
		LogQueriesLongerThan:   manifests.Duration(manifests.OptionalToString(frontend.Spec.LogQueriesLongerThan)),
		CompressResponses:      frontend.Spec.CompressResponses,
		ResponseCacheConfig:    frontend.Spec.QueryRangeResponseCacheConfig,
		RangeSplitInterval:     manifests.Duration(manifests.OptionalToString(frontend.Spec.QueryRangeSplitInterval)),
		LabelsSplitInterval:    manifests.Duration(manifests.OptionalToString(frontend.Spec.LabelsSplitInterval)),
		RangeMaxRetries:        frontend.Spec.QueryRangeMaxRetries,
		LabelsMaxRetries:       frontend.Spec.LabelsMaxRetries,
		LabelsDefaultTimeRange: manifests.Duration(manifests.OptionalToString(frontend.Spec.LabelsDefaultTimeRange)),
	}), nil
}

// getQueryService returns the first ThanosQuery service found in the same namespace as the ThanosQueryFrontend.
func (r *ThanosQueryFrontendReconciler) getQueryService(ctx context.Context, frontend monitoringthanosiov1alpha1.ThanosQueryFrontend) (string, int32, error) {
	services := &corev1.ServiceList{}
	listOpts := []client.ListOption{
		client.InNamespace(frontend.Namespace),
		client.MatchingLabels(map[string]string{
			manifests.PartOfLabel:          manifests.DefaultPartOfLabel,
			manifests.DefaultQueryAPILabel: manifests.DefaultQueryAPIValue,
		}),
	}
	if err := r.List(ctx, services, listOpts...); err != nil {
		return "", 0, err
	}

	if len(services.Items) == 0 {
		return "", 0, fmt.Errorf("no ThanosQuery service found in namespace %s", frontend.Namespace)
	}

	service := services.Items[0]
	var port int32
	for _, p := range service.Spec.Ports {
		if p.Name == manifestqueryfrontend.HTTPPortName {
			port = p.Port
			break
		}
	}
	if port == 0 {
		return "", 0, fmt.Errorf("no HTTP port found for ThanosQuery service %s in namespace %s", service.Name, frontend.Namespace)
	}

	return service.Name, port, nil
}

// SetupWithManager sets up the controller with the Manager.
func (r *ThanosQueryFrontendReconciler) SetupWithManager(mgr ctrl.Manager) error {
	queryServicePredicate, err := predicate.LabelSelectorPredicate(metav1.LabelSelector{
		MatchLabels: map[string]string{
			manifests.PartOfLabel:          manifests.DefaultPartOfLabel,
			manifests.DefaultQueryAPILabel: manifests.DefaultQueryAPIValue,
		},
	})
	if err != nil {
		return err
	}

	return ctrl.NewControllerManagedBy(mgr).
		For(&monitoringthanosiov1alpha1.ThanosQueryFrontend{}).
		Owns(&corev1.ConfigMap{}).
		Owns(&corev1.ServiceAccount{}).
		Owns(&corev1.Service{}).
		Owns(&appsv1.Deployment{}).
		Watches(
			&corev1.Service{},
			r.enqueueForQueryService(),
			builder.WithPredicates(predicate.GenerationChangedPredicate{}, queryServicePredicate),
		).
		Complete(r)
}

// enqueueForQueryService returns an EventHandler that will enqueue a request for the ThanosQueryFrontend instances
// that are in the same namespace as the ThanosQuery service.
func (r *ThanosQueryFrontendReconciler) enqueueForQueryService() handler.EventHandler {
	return handler.EnqueueRequestsFromMapFunc(func(ctx context.Context, obj client.Object) []reconcile.Request {
		if obj.GetLabels()[manifests.DefaultQueryAPILabel] != manifests.DefaultQueryAPIValue {
			return nil
		}

		frontends := &monitoringthanosiov1alpha1.ThanosQueryFrontendList{}
		err := r.List(
			ctx,
			frontends,
			client.InNamespace(obj.GetNamespace()),
		)
		if err != nil {
			return []reconcile.Request{}
		}

		requests := make([]reconcile.Request, len(frontends.Items))
		for i, frontend := range frontends.Items {
			requests[i] = reconcile.Request{
				NamespacedName: types.NamespacedName{
					Name:      frontend.GetName(),
					Namespace: frontend.GetNamespace(),
				},
			}
		}

		return requests
	})
}
