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
	manifestruler "github.com/thanos-community/thanos-operator/internal/pkg/manifests/ruler"
	controllermetrics "github.com/thanos-community/thanos-operator/internal/pkg/metrics"

	appsv1 "k8s.io/api/apps/v1"
	corev1 "k8s.io/api/core/v1"
	apierrors "k8s.io/apimachinery/pkg/api/errors"
	"k8s.io/apimachinery/pkg/api/resource"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/labels"
	"k8s.io/apimachinery/pkg/runtime"
	"k8s.io/apimachinery/pkg/types"
	"k8s.io/client-go/tools/record"
	"k8s.io/utils/ptr"

	ctrl "sigs.k8s.io/controller-runtime"
	"sigs.k8s.io/controller-runtime/pkg/builder"
	"sigs.k8s.io/controller-runtime/pkg/client"
	"sigs.k8s.io/controller-runtime/pkg/handler"
	"sigs.k8s.io/controller-runtime/pkg/predicate"
	"sigs.k8s.io/controller-runtime/pkg/reconcile"
)

// ThanosRulerReconciler reconciles a ThanosRuler object
type ThanosRulerReconciler struct {
	client.Client
	Scheme   *runtime.Scheme
	Recorder record.EventRecorder

	logger logr.Logger

	reg                   prometheus.Registerer
	ControllerBaseMetrics *controllermetrics.BaseMetrics
	thanosRulerMetrics    controllermetrics.ThanosRulerMetrics
}

// NewThanosRulerReconciler returns a reconciler for ThanosRuler resources.
func NewThanosRulerReconciler(logger logr.Logger, client client.Client, scheme *runtime.Scheme, recorder record.EventRecorder, reg prometheus.Registerer, controllerBaseMetrics *controllermetrics.BaseMetrics) *ThanosRulerReconciler {
	return &ThanosRulerReconciler{
		Client:   client,
		Scheme:   scheme,
		Recorder: recorder,

		logger:                logger,
		reg:                   reg,
		ControllerBaseMetrics: controllerBaseMetrics,
		thanosRulerMetrics:    controllermetrics.NewThanosRulerMetrics(reg),
	}
}

//+kubebuilder:rbac:groups=monitoring.thanos.io,resources=thanosrulers,verbs=get;list;watch;create;update;patch;delete
//+kubebuilder:rbac:groups=monitoring.thanos.io,resources=thanosrulers/status,verbs=get;update;patch
//+kubebuilder:rbac:groups=monitoring.thanos.io,resources=thanosrulers/finalizers,verbs=update
//+kubebuilder:rbac:groups=apps,resources=statefulsets,verbs=get;list;watch;create;update;patch;delete
//+kubebuilder:rbac:groups="",resources=services;configmaps;serviceaccounts,verbs=get;list;watch;create;update;patch;delete

// Reconcile is part of the main kubernetes reconciliation loop which aims to
// move the current state of the cluster closer to the desired state.
//
// For more details, check Reconcile and its Result here:
// - https://pkg.go.dev/sigs.k8s.io/controller-runtime@v0.17.0/pkg/reconcile
func (r *ThanosRulerReconciler) Reconcile(ctx context.Context, req ctrl.Request) (ctrl.Result, error) {
	r.ControllerBaseMetrics.ReconciliationsTotal.WithLabelValues(manifestruler.Name).Inc()

	ruler := &monitoringthanosiov1alpha1.ThanosRuler{}
	err := r.Get(ctx, req.NamespacedName, ruler)
	if err != nil {
		r.ControllerBaseMetrics.ClientErrorsTotal.WithLabelValues(manifestruler.Name).Inc()
		if apierrors.IsNotFound(err) {
			r.logger.Info("thanos ruler resource not found. ignoring since object may be deleted")
			return ctrl.Result{}, nil
		}
		r.logger.Error(err, "failed to get ThanosRuler")
		r.ControllerBaseMetrics.ReconciliationsFailedTotal.WithLabelValues(manifestruler.Name).Inc()
		return ctrl.Result{}, err
	}

	err = r.syncResources(ctx, *ruler)
	if err != nil {
		r.ControllerBaseMetrics.ReconciliationsFailedTotal.WithLabelValues(manifestruler.Name).Inc()
		return ctrl.Result{}, err
	}

	return ctrl.Result{}, nil
}

func (r *ThanosRulerReconciler) syncResources(ctx context.Context, ruler monitoringthanosiov1alpha1.ThanosRuler) error {
	var objs []client.Object

	desiredObjs := r.buildRuler(ctx, ruler)
	objs = append(objs, desiredObjs...)

	var errCount int32
	for _, obj := range objs {
		if manifests.IsNamespacedResource(obj) {
			obj.SetNamespace(ruler.Namespace)
			if err := ctrl.SetControllerReference(&ruler, obj, r.Scheme); err != nil {
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
		r.ControllerBaseMetrics.ClientErrorsTotal.WithLabelValues(manifestruler.Name).Add(float64(errCount))
		return fmt.Errorf("failed to create or update %d resources for the ruler", errCount)
	}

	return nil
}

func (r *ThanosRulerReconciler) buildRuler(ctx context.Context, ruler monitoringthanosiov1alpha1.ThanosRuler) []client.Object {
	metaOpts := manifests.Options{
		Name:      ruler.GetName(),
		Namespace: ruler.GetNamespace(),
		Replicas:  ruler.Spec.Replicas,
		Labels:    ruler.GetLabels(),
		Image:     ruler.Spec.Image,
		LogLevel:  ruler.Spec.LogLevel,
		LogFormat: ruler.Spec.LogFormat,
	}.ApplyDefaults()

	endpoints := r.getQueryAPIServiceEndpoints(ctx, ruler)
	ruleFiles := r.getRuleConfigMaps(ctx, ruler)

	additional := manifests.Additional{
		Args:         ruler.Spec.Additional.Args,
		Containers:   ruler.Spec.Additional.Containers,
		Volumes:      ruler.Spec.Additional.Volumes,
		VolumeMounts: ruler.Spec.Additional.VolumeMounts,
		Ports:        ruler.Spec.Additional.Ports,
		Env:          ruler.Spec.Additional.Env,
		ServicePorts: ruler.Spec.Additional.ServicePorts,
	}
	return manifestruler.BuildRuler(manifestruler.RulerOptions{
		Options:            metaOpts,
		Endpoints:          endpoints,
		RuleFiles:          ruleFiles,
		ObjStoreSecret:     ruler.Spec.ObjectStorageConfig.ToSecretKeySelector(),
		Retention:          manifests.Duration(ruler.Spec.Retention),
		AlertmanagerURL:    ruler.Spec.AlertmanagerURL,
		ExternalLabels:     ruler.Spec.ExternalLabels,
		AlertLabelDrop:     ruler.Spec.AlertLabelDrop,
		StorageSize:        resource.MustParse(ruler.Spec.StorageSize),
		EvaluationInterval: manifests.Duration(ruler.Spec.EvaluationInterval),
		Additional:         additional,
	})
}

// getStoreAPIServiceEndpoints returns the list of endpoints for the QueryAPI services that match the ThanosRuler queryLabelSelector.
func (r *ThanosRulerReconciler) getQueryAPIServiceEndpoints(ctx context.Context, ruler monitoringthanosiov1alpha1.ThanosRuler) []manifestruler.Endpoint {
	services := &corev1.ServiceList{}
	if err := r.List(
		ctx,
		services,
		[]client.ListOption{
			client.MatchingLabels(ruler.Spec.QueryLabelSelector.MatchLabels),
			client.InNamespace(ruler.Namespace),
		}...); err != nil {
		return []manifestruler.Endpoint{}
	}

	if len(services.Items) == 0 {
		return []manifestruler.Endpoint{}
	}

	endpoints := make([]manifestruler.Endpoint, len(services.Items))
	for i, svc := range services.Items {
		for _, port := range svc.Spec.Ports {
			if port.Name == manifestruler.GRPCPortName {
				endpoints[i].Port = port.Port
				break
			}
		}

		endpoints[i] = manifestruler.Endpoint{
			ServiceName: svc.GetName(),
			Namespace:   svc.GetNamespace(),
		}
	}

	r.thanosRulerMetrics.EndpointsConfigured.WithLabelValues(ruler.GetName(), ruler.GetNamespace()).Set(float64(len(endpoints)))

	return endpoints
}

// getRuleConfigMaps returns the list of ruler configmaps of rule files to set on ThanosRuler.
func (r *ThanosRulerReconciler) getRuleConfigMaps(ctx context.Context, ruler monitoringthanosiov1alpha1.ThanosRuler) []corev1.ConfigMapKeySelector {
	cfgmaps := &corev1.ConfigMapList{}
	if err := r.List(
		ctx,
		cfgmaps,
		[]client.ListOption{
			client.MatchingLabels(ruler.Spec.RuleConfigSelector.MatchLabels),
			client.InNamespace(ruler.Namespace),
		}...); err != nil {
		return []corev1.ConfigMapKeySelector{}
	}

	if len(cfgmaps.Items) == 0 {
		return []corev1.ConfigMapKeySelector{}
	}

	ruleFiles := make([]corev1.ConfigMapKeySelector, 0, len(cfgmaps.Items))
	for _, cfgmap := range cfgmaps.Items {
		if cfgmap.Data == nil || len(cfgmap.Data) != 1 {
			continue
		}

		for key := range cfgmap.Data {
			ruleFiles = append(ruleFiles, corev1.ConfigMapKeySelector{
				LocalObjectReference: corev1.LocalObjectReference{
					Name: cfgmap.GetName(),
				},
				Key:      key,
				Optional: ptr.To(true),
			})
		}
	}

	r.thanosRulerMetrics.RuleFilesConfigured.WithLabelValues(ruler.GetName(), ruler.GetNamespace()).Set(float64(len(ruleFiles)))

	return ruleFiles
}

// SetupWithManager sets up the controller with the Manager.
func (r *ThanosRulerReconciler) SetupWithManager(mgr ctrl.Manager) error {
	servicePredicate, err := predicate.LabelSelectorPredicate(metav1.LabelSelector{
		MatchLabels: map[string]string{
			manifests.PartOfLabel:          manifests.DefaultPartOfLabel,
			manifests.DefaultQueryAPILabel: manifests.DefaultQueryAPIValue,
		},
	})
	if err != nil {
		return err
	}

	configMapPredicate, err := predicate.LabelSelectorPredicate(metav1.LabelSelector{
		MatchLabels: map[string]string{
			manifests.DefaultRuleConfigLabel: manifests.DefaultRuleConfigValue,
		},
	})
	if err != nil {
		return err
	}

	return ctrl.NewControllerManagedBy(mgr).
		For(&monitoringthanosiov1alpha1.ThanosRuler{}).
		Owns(&corev1.ConfigMap{}).
		Owns(&corev1.ServiceAccount{}).
		Owns(&corev1.Service{}).
		Owns(&appsv1.StatefulSet{}).
		Watches(
			&corev1.Service{},
			r.enqueueForService(),
			builder.WithPredicates(predicate.GenerationChangedPredicate{}, servicePredicate),
		).
		Watches(
			&corev1.ConfigMap{},
			r.enqueueForConfigMap(),
			builder.WithPredicates(predicate.GenerationChangedPredicate{}, configMapPredicate),
		).
		Complete(r)
}

// enqueueForService returns an EventHandler that will enqueue a request for the ThanosRuler instances
// that matches the Service.
func (r *ThanosRulerReconciler) enqueueForService() handler.EventHandler {
	return handler.EnqueueRequestsFromMapFunc(func(ctx context.Context, obj client.Object) []reconcile.Request {
		if obj.GetLabels()[manifests.DefaultQueryAPILabel] != manifests.DefaultQueryAPIValue {
			return nil
		}

		rulers := &monitoringthanosiov1alpha1.ThanosRulerList{}
		err := r.List(
			ctx,
			rulers,
			[]client.ListOption{
				client.InNamespace(obj.GetNamespace()),
			}...)
		if err != nil {
			return []reconcile.Request{}
		}

		requests := []reconcile.Request{}
		for _, ruler := range rulers.Items {
			if labels.SelectorFromSet(ruler.Spec.QueryLabelSelector.MatchLabels).Matches(labels.Set(obj.GetLabels())) {
				requests = append(requests, reconcile.Request{
					NamespacedName: types.NamespacedName{
						Name:      ruler.GetName(),
						Namespace: ruler.GetNamespace(),
					},
				})
			}
		}

		r.thanosRulerMetrics.ServiceWatchesReconciliationsTotal.Add(float64(len(requests)))
		return requests
	})
}

// enqueueForConfigMap returns an EventHandler that will enqueue a request for the ThanosRuler instances
// that matches the Service.
func (r *ThanosRulerReconciler) enqueueForConfigMap() handler.EventHandler {
	return handler.EnqueueRequestsFromMapFunc(func(ctx context.Context, obj client.Object) []reconcile.Request {
		if obj.GetLabels()[manifests.DefaultRuleConfigLabel] != manifests.DefaultRuleConfigValue {
			return nil
		}

		rulers := &monitoringthanosiov1alpha1.ThanosRulerList{}
		err := r.List(
			ctx,
			rulers,
			[]client.ListOption{
				client.InNamespace(obj.GetNamespace()),
			}...)
		if err != nil {
			return []reconcile.Request{}
		}

		requests := []reconcile.Request{}
		for _, ruler := range rulers.Items {
			if labels.SelectorFromSet(ruler.Spec.RuleConfigSelector.MatchLabels).Matches(labels.Set(obj.GetLabels())) {
				requests = append(requests, reconcile.Request{
					NamespacedName: types.NamespacedName{
						Name:      ruler.GetName(),
						Namespace: ruler.GetNamespace(),
					},
				})
			}
		}

		r.thanosRulerMetrics.ConfigMapWatchesReconcilationsTotal.Add(float64(len(requests)))
		return requests
	})
}