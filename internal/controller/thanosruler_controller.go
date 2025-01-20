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
	manifestruler "github.com/thanos-community/thanos-operator/internal/pkg/manifests/ruler"
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
	Scheme *runtime.Scheme

	logger   logr.Logger
	metrics  controllermetrics.ThanosRulerMetrics
	recorder record.EventRecorder

	handler *handlers.Handler
}

// NewThanosRulerReconciler returns a reconciler for ThanosRuler resources.
func NewThanosRulerReconciler(conf Config, client client.Client, scheme *runtime.Scheme) *ThanosRulerReconciler {
	handler := handlers.NewHandler(client, scheme, conf.InstrumentationConfig.Logger)
	featureGates := conf.FeatureGate.ToGVK()
	if len(featureGates) > 0 {
		handler.SetFeatureGates(featureGates)
	}

	return &ThanosRulerReconciler{
		Client:   client,
		Scheme:   scheme,
		logger:   conf.InstrumentationConfig.Logger,
		metrics:  controllermetrics.NewThanosRulerMetrics(conf.InstrumentationConfig.MetricsRegistry),
		recorder: conf.InstrumentationConfig.EventRecorder,
		handler:  handler,
	}
}

// +kubebuilder:rbac:groups=monitoring.thanos.io,resources=thanosrulers,verbs=get;list;watch;create;update;patch;delete
// +kubebuilder:rbac:groups=monitoring.thanos.io,resources=thanosrulers/status,verbs=get;update;patch
// +kubebuilder:rbac:groups=monitoring.thanos.io,resources=thanosrulers/finalizers,verbs=update
// +kubebuilder:rbac:groups=apps,resources=statefulsets,verbs=get;list;watch;create;update;patch;delete
// +kubebuilder:rbac:groups="",resources=services;configmaps;serviceaccounts,verbs=get;list;watch;create;update;patch;delete
// +kubebuilder:rbac:groups=policy,resources=poddisruptionbudgets,verbs=get;list;watch;create;update
// +kubebuilder:rbac:groups=monitoring.coreos.com,resources=prometheusrules,verbs=get;list;watch

// Reconcile is part of the main kubernetes reconciliation loop which aims to
// move the current state of the cluster closer to the desired state.
//
// For more details, check Reconcile and its Result here:
// - https://pkg.go.dev/sigs.k8s.io/controller-runtime@v0.17.0/pkg/reconcile
func (r *ThanosRulerReconciler) Reconcile(ctx context.Context, req ctrl.Request) (ctrl.Result, error) {
	ruler := &monitoringthanosiov1alpha1.ThanosRuler{}
	err := r.Get(ctx, req.NamespacedName, ruler)
	if err != nil {
		if apierrors.IsNotFound(err) {
			r.logger.Info("thanos ruler resource not found. ignoring since object may be deleted")
			return ctrl.Result{}, nil
		}
		r.logger.Error(err, "failed to get ThanosRuler")
		r.recorder.Event(ruler, corev1.EventTypeWarning, "GetFailed", "Failed to get ThanosRuler resource")
		return ctrl.Result{}, err
	}

	if ruler.Spec.Paused != nil && *ruler.Spec.Paused {
		r.logger.Info("reconciliation is paused for ThanosRuler resource")
		r.recorder.Event(ruler, corev1.EventTypeNormal, "Paused", "Reconciliation is paused for ThanosRuler resource")
		return ctrl.Result{}, nil
	}

	err = r.syncResources(ctx, *ruler)
	if err != nil {
		r.recorder.Event(ruler, corev1.EventTypeWarning, "SyncFailed", fmt.Sprintf("Failed to sync resources: %v", err))
		return ctrl.Result{}, err
	}

	return ctrl.Result{}, nil
}

func (r *ThanosRulerReconciler) syncResources(ctx context.Context, ruler monitoringthanosiov1alpha1.ThanosRuler) error {
	var objs []client.Object

	desiredObjs, err := r.buildRuler(ctx, ruler)
	if err != nil {
		return err
	}

	objs = append(objs, desiredObjs...)

	if errCount := r.handler.CreateOrUpdate(ctx, ruler.GetNamespace(), &ruler, objs); errCount > 0 {
		return fmt.Errorf("failed to create or update %d resources for the ruler", errCount)
	}
	if !manifests.HasServiceMonitorEnabled(ruler.Spec.FeatureGates) {
		if errCount := r.handler.DeleteResource(ctx, []client.Object{&monitoringv1.ServiceMonitor{
			ObjectMeta: metav1.ObjectMeta{
				Name:      RulerNameFromParent(ruler.GetName()),
				Namespace: ruler.GetNamespace(),
			},
		},
		}); errCount > 0 {
			return fmt.Errorf("failed to delete %d resources for the ruler", errCount)
		}
	}

	return nil
}

func (r *ThanosRulerReconciler) buildRuler(ctx context.Context, ruler monitoringthanosiov1alpha1.ThanosRuler) ([]client.Object, error) {
	endpoints, err := r.getQueryAPIServiceEndpoints(ctx, ruler)
	if err != nil {
		return []client.Object{}, err
	}

	if len(endpoints) == 0 {
		return []client.Object{}, fmt.Errorf("no query API services found")
	}

	ruleFiles, err := r.getRuleConfigMaps(ctx, ruler)
	if err != nil {
		return []client.Object{}, err
	}
	r.logger.Info("found rule configmaps", "count", len(ruleFiles), "ruler", ruler.Name)

	promRuleConfigMaps := []corev1.ConfigMapKeySelector{}
	if manifests.HasPrometheusRuleEnabled(ruler.Spec.FeatureGates) {
		promRuleConfigMaps, err = r.getPrometheusRuleConfigMaps(ctx, ruler)
		if err != nil {
			return []client.Object{}, err
		}
		r.logger.Info("found prometheus rule-based configmaps", "count", len(promRuleConfigMaps), "ruler", ruler.Name)
	}

	// PrometheusRule-based configmaps take precedence.
	uniqueRuleFiles := make(map[string]corev1.ConfigMapKeySelector)
	for _, rf := range ruleFiles {
		uniqueRuleFiles[rf.Name] = rf
	}
	for _, prf := range promRuleConfigMaps {
		uniqueRuleFiles[prf.Name] = prf
	}

	ruleFiles = make([]corev1.ConfigMapKeySelector, 0, len(uniqueRuleFiles))
	for _, rf := range uniqueRuleFiles {
		ruleFiles = append(ruleFiles, rf)
	}

	r.logger.Info("total rule files to configure", "count", len(ruleFiles), "ruler", ruler.Name)
	r.metrics.RuleFilesConfigured.WithLabelValues(ruler.GetName(), ruler.GetNamespace()).Set(float64(len(ruleFiles)))

	opts := rulerV1Alpha1ToOptions(ruler)
	opts.Endpoints = endpoints
	opts.RuleFiles = ruleFiles

	return opts.Build(), nil
}

// getStoreAPIServiceEndpoints returns the list of endpoints for the QueryAPI services that match the ThanosRuler queryLabelSelector.
func (r *ThanosRulerReconciler) getQueryAPIServiceEndpoints(ctx context.Context, ruler monitoringthanosiov1alpha1.ThanosRuler) ([]manifestruler.Endpoint, error) {
	labelSelector, err := manifests.BuildLabelSelectorFrom(ruler.Spec.QueryLabelSelector, requiredQueryServiceLabels)
	if err != nil {
		return []manifestruler.Endpoint{}, err
	}

	opts := []client.ListOption{client.MatchingLabelsSelector{Selector: labelSelector}, client.InNamespace(ruler.Namespace)}

	services := &corev1.ServiceList{}
	if err := r.List(ctx, services, opts...); err != nil {
		return nil, err
	}

	if len(services.Items) == 0 {
		r.recorder.Event(&ruler, corev1.EventTypeWarning, "NoEndpointsFound", "No QueryAPI services found")
		return []manifestruler.Endpoint{}, nil
	}

	endpoints := make([]manifestruler.Endpoint, len(services.Items))
	for i, svc := range services.Items {
		port, ok := manifests.IsGrpcServiceWithLabels(&svc, requiredQueryServiceLabels)
		if !ok {
			r.logger.Info("service is not a gRPC service", "service", svc.GetName())
			continue
		}

		endpoints[i] = manifestruler.Endpoint{
			Port:        port,
			ServiceName: svc.GetName(),
			Namespace:   svc.GetNamespace(),
		}
	}

	r.metrics.EndpointsConfigured.WithLabelValues(ruler.GetName(), ruler.GetNamespace()).Set(float64(len(endpoints)))

	return endpoints, nil
}

// getRuleConfigMaps returns the list of ruler configmaps of rule files to set on ThanosRuler.
func (r *ThanosRulerReconciler) getRuleConfigMaps(ctx context.Context, ruler monitoringthanosiov1alpha1.ThanosRuler) ([]corev1.ConfigMapKeySelector, error) {
	labelSelector, err := manifests.BuildLabelSelectorFrom(ruler.Spec.RuleConfigSelector, requiredRuleConfigMapLabels)
	if err != nil {
		return nil, err
	}

	opts := []client.ListOption{client.MatchingLabelsSelector{Selector: labelSelector}, client.InNamespace(ruler.Namespace)}
	cfgmaps := &corev1.ConfigMapList{}
	if err := r.List(ctx, cfgmaps, opts...); err != nil {
		return []corev1.ConfigMapKeySelector{}, err
	}

	if len(cfgmaps.Items) == 0 {
		r.recorder.Event(&ruler, corev1.EventTypeWarning, "NoRuleConfigsFound", "No rule ConfigMaps found")
		return []corev1.ConfigMapKeySelector{}, nil
	}

	r.logger.Info("processing rule config maps",
		"found", len(cfgmaps.Items),
		"ruler", ruler.Name,
		"namespace", ruler.Namespace)

	ruleFiles := make([]corev1.ConfigMapKeySelector, 0, len(cfgmaps.Items))
	for _, cfgmap := range cfgmaps.Items {
		if cfgmap.Data == nil || len(cfgmap.Data) != 1 {
			r.logger.Info("skipping invalid config map",
				"name", cfgmap.Name,
				"dataKeys", len(cfgmap.Data),
				"ruler", ruler.Name)
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

	return ruleFiles, nil
}

// getPrometheusRuleConfigMaps returns the list of ruler configmaps of rule files to set on ThanosRuler.
func (r *ThanosRulerReconciler) getPrometheusRuleConfigMaps(ctx context.Context, ruler monitoringthanosiov1alpha1.ThanosRuler) ([]corev1.ConfigMapKeySelector, error) {
	if ruler.Spec.PrometheusRuleSelector.MatchLabels == nil {
		r.logger.Info("no prometheus rule selector specified, skipping", "ruler", ruler.Name)
		return []corev1.ConfigMapKeySelector{}, nil
	}

	labelSelector, err := manifests.BuildLabelSelectorFrom(&ruler.Spec.PrometheusRuleSelector, nil)
	if err != nil {
		return nil, fmt.Errorf("failed to build PrometheusRule label selector: %w", err)
	}

	promRules := &monitoringv1.PrometheusRuleList{}
	if err := r.List(ctx, promRules,
		client.InNamespace(ruler.Namespace),
		client.MatchingLabelsSelector{Selector: labelSelector},
	); err != nil {
		return nil, err
	}

	if len(promRules.Items) == 0 {
		return []corev1.ConfigMapKeySelector{}, nil
	}

	r.logger.Info("processing prometheus rules",
		"found", len(promRules.Items),
		"ruler", ruler.Name,
		"namespace", ruler.Namespace)

	// Collect all rule files first
	allRuleFiles := make(map[string]string)
	for _, rule := range promRules.Items {
		ruleContent := manifestruler.GenerateRuleFileContent(rule.Spec.Groups)
		allRuleFiles[fmt.Sprintf("%s.yaml", rule.Name)] = ruleContent
	}

	// Now create ConfigMaps from all rules together
	configMaps, err := manifestruler.MakeRulesConfigMaps(allRuleFiles)
	if err != nil {
		return nil, fmt.Errorf("failed to create config maps for rules: %w", err)
	}

	var ruleFileCfgMaps []corev1.ConfigMapKeySelector
	objs := []client.Object{}

	// Create ConfigMaps with proper names and metadata
	for i, cm := range configMaps {
		cmName := manifests.SanitizeName(fmt.Sprintf("%s-promrule-%d", ruler.Name, i))

		configMap := &corev1.ConfigMap{
			ObjectMeta: metav1.ObjectMeta{
				Name:      cmName,
				Namespace: ruler.Namespace,
				Labels: map[string]string{
					manifests.DefaultRuleConfigLabel: manifests.DefaultRuleConfigValue,
				},
				OwnerReferences: []metav1.OwnerReference{
					{
						APIVersion: ruler.APIVersion,
						Kind:       ruler.Kind,
						Name:       ruler.Name,
						UID:        ruler.UID,
						Controller: ptr.To(true),
					},
				},
			},
			Data: cm.Data,
		}

		objs = append(objs, configMap)

		// Add each file in the ConfigMap to the rule files list
		for key := range cm.Data {
			ruleFileCfgMaps = append(ruleFileCfgMaps, corev1.ConfigMapKeySelector{
				LocalObjectReference: corev1.LocalObjectReference{
					Name: cmName,
				},
				Key:      key,
				Optional: ptr.To(true),
			})
		}
	}

	if errCount := r.handler.CreateOrUpdate(ctx, ruler.GetNamespace(), &ruler, objs); errCount > 0 {
		return nil, fmt.Errorf("failed to create or update %d ConfigMaps from PrometheusRule", errCount)
	}

	return ruleFileCfgMaps, nil
}

// SetupWithManager sets up the controller with the Manager.
func (r *ThanosRulerReconciler) SetupWithManager(mgr ctrl.Manager) error {
	serviceLabelPredicate, err := predicate.LabelSelectorPredicate(metav1.LabelSelector{
		MatchLabels: requiredQueryServiceLabels,
	})
	if err != nil {
		return err
	}

	svcOnLabelChangePredicate := predicate.And(serviceLabelPredicate, predicate.LabelChangedPredicate{})
	svcOnGenChangePredicate := predicate.And(serviceLabelPredicate, predicate.GenerationChangedPredicate{})
	svcPredicate := predicate.Or(svcOnLabelChangePredicate, svcOnGenChangePredicate)

	configMapPredicate, err := predicate.LabelSelectorPredicate(metav1.LabelSelector{
		MatchLabels: requiredRuleConfigMapLabels,
	})
	if err != nil {
		return err
	}

	bldr := ctrl.NewControllerManagedBy(mgr).
		For(&monitoringthanosiov1alpha1.ThanosRuler{}).
		Owns(&corev1.ConfigMap{}).
		Owns(&corev1.ServiceAccount{}).
		Owns(&corev1.Service{}).
		Owns(&appsv1.StatefulSet{}).
		Owns(&policyv1.PodDisruptionBudget{}).
		Owns(&monitoringv1.ServiceMonitor{}).
		Watches(
			&corev1.Service{},
			r.enqueueForService(),
			builder.WithPredicates(svcPredicate),
			builder.WithPredicates(predicate.GenerationChangedPredicate{}),
		).
		Watches(
			&corev1.ConfigMap{},
			r.enqueueForConfigMap(),
			builder.WithPredicates(predicate.GenerationChangedPredicate{}, configMapPredicate),
		)

	if !r.handler.IsFeatureGated(&monitoringv1.PrometheusRule{}) {
		bldr.Watches(
			&monitoringv1.PrometheusRule{},
			r.enqueueForPrometheusRule(),
			builder.WithPredicates(predicate.GenerationChangedPredicate{}),
		)
	}

	if err := bldr.Complete(r); err != nil {
		r.recorder.Event(&monitoringthanosiov1alpha1.ThanosRuler{}, corev1.EventTypeWarning, "SetupFailed", fmt.Sprintf("Failed to set up controller: %v", err))
		return err
	}

	return nil
}

// enqueueForService returns an EventHandler that will enqueue a request for the ThanosRuler instances
// that matches the Service.
func (r *ThanosRulerReconciler) enqueueForService() handler.EventHandler {
	return handler.EnqueueRequestsFromMapFunc(func(ctx context.Context, obj client.Object) []reconcile.Request {
		if !r.isQueueableQueryService(obj) {
			return []reconcile.Request{}
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
			selector, err := manifests.BuildLabelSelectorFrom(ruler.Spec.QueryLabelSelector, requiredQueryServiceLabels)
			if err != nil {
				r.logger.Error(err, "failed to build label selector from ruler query label selector", "ruler", ruler.GetName())
				continue
			}
			if selector.Matches(labels.Set(obj.GetLabels())) {
				requests = append(requests, reconcile.Request{
					NamespacedName: types.NamespacedName{
						Name:      ruler.GetName(),
						Namespace: ruler.GetNamespace(),
					},
				})
			}
		}

		r.metrics.ServiceWatchesReconciliationsTotal.Add(float64(len(requests)))
		return requests
	})
}

// enqueueForConfigMap returns an EventHandler that will enqueue a request for the ThanosRuler instances
// that matches the Service.
func (r *ThanosRulerReconciler) enqueueForConfigMap() handler.EventHandler {
	return handler.EnqueueRequestsFromMapFunc(func(ctx context.Context, obj client.Object) []reconcile.Request {
		if !manifests.HasRequiredLabels(obj, requiredRuleConfigMapLabels) {
			return []reconcile.Request{}
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
			selector, err := manifests.BuildLabelSelectorFrom(ruler.Spec.RuleConfigSelector, requiredRuleConfigMapLabels)
			if err != nil {
				r.logger.Error(err, "failed to build label selector from ruler rule config selector", "ruler", ruler.GetName())
				continue
			}
			if selector.Matches(labels.Set(obj.GetLabels())) {
				requests = append(requests, reconcile.Request{
					NamespacedName: types.NamespacedName{
						Name:      ruler.GetName(),
						Namespace: ruler.GetNamespace(),
					},
				})
			}
		}

		r.metrics.ConfigMapWatchesReconciliationsTotal.Add(float64(len(requests)))
		return requests
	})
}

// isQueueableQueryService returns true if the Service is a QueryAPI service that is part of a 'thanos' and has a gRPC port.
func (r *ThanosRulerReconciler) isQueueableQueryService(obj client.Object) bool {
	_, isGRPCSvc := manifests.IsGrpcServiceWithLabels(obj, requiredQueryServiceLabels)
	return isGRPCSvc
}

var requiredQueryServiceLabels = map[string]string{
	manifests.DefaultQueryAPILabel: manifests.DefaultQueryAPIValue,
	manifests.PartOfLabel:          manifests.DefaultPartOfLabel,
}

var requiredRuleConfigMapLabels = map[string]string{
	manifests.DefaultRuleConfigLabel: manifests.DefaultRuleConfigValue,
}

// Add this new function to handle PrometheusRule events
func (r *ThanosRulerReconciler) enqueueForPrometheusRule() handler.EventHandler {
	return handler.EnqueueRequestsFromMapFunc(func(ctx context.Context, obj client.Object) []reconcile.Request {
		rulers := &monitoringthanosiov1alpha1.ThanosRulerList{}
		err := r.List(ctx, rulers, client.InNamespace(obj.GetNamespace()))
		if err != nil {
			return []reconcile.Request{}
		}

		requests := []reconcile.Request{}
		for _, ruler := range rulers.Items {
			selector, err := manifests.BuildLabelSelectorFrom(&ruler.Spec.PrometheusRuleSelector, nil)
			if err != nil {
				r.logger.Error(err, "failed to build label selector from ruler PrometheusRule selector",
					"ruler", ruler.GetName())
				continue
			}

			if selector.Matches(labels.Set(obj.GetLabels())) {
				r.logger.Info("found prometheus rule enqueueing", "ruler", ruler.GetName())
				requests = append(requests, reconcile.Request{
					NamespacedName: types.NamespacedName{
						Name:      ruler.GetName(),
						Namespace: ruler.GetNamespace(),
					},
				})
			}
		}

		return requests
	})
}
