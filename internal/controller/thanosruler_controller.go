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
	"sort"
	"strings"

	"github.com/go-logr/logr"
	"github.com/prometheus-community/prom-label-proxy/injectproxy"
	monitoringv1 "github.com/prometheus-operator/prometheus-operator/pkg/apis/monitoring/v1"
	promlabels "github.com/prometheus/prometheus/model/labels"

	monitoringthanosiov1alpha1 "github.com/thanos-community/thanos-operator/api/v1alpha1"
	"github.com/thanos-community/thanos-operator/internal/pkg/featuregate"
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
	"k8s.io/apimachinery/pkg/selection"
	"k8s.io/apimachinery/pkg/types"
	"k8s.io/client-go/tools/record"
	"k8s.io/utils/ptr"

	"k8s.io/apimachinery/pkg/api/meta"
	"k8s.io/apimachinery/pkg/util/intstr"
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

	handler                *handlers.Handler
	disableConditionUpdate bool

	featureGate featuregate.Config
}

// NewThanosRulerReconciler returns a reconciler for ThanosRuler resources.
func NewThanosRulerReconciler(conf Config, client client.Client, scheme *runtime.Scheme) *ThanosRulerReconciler {
	reconciler := &ThanosRulerReconciler{
		Client:      client,
		Scheme:      scheme,
		logger:      conf.InstrumentationConfig.Logger,
		metrics:     controllermetrics.NewThanosRulerMetrics(conf.InstrumentationConfig.MetricsRegistry, conf.InstrumentationConfig.CommonMetrics),
		recorder:    conf.InstrumentationConfig.EventRecorder,
		featureGate: conf.FeatureGate,
	}

	h := handlers.NewHandler(client, scheme, conf.InstrumentationConfig.Logger)
	featureGates := conf.FeatureGate.ToGVK()
	if len(featureGates) > 0 {
		reconciler.metrics.FeatureGatesEnabled.WithLabelValues("ruler").Set(float64(len(featureGates)))
		h.SetFeatureGates(featureGates)
	}
	reconciler.handler = h

	return reconciler
}

// +kubebuilder:rbac:groups=monitoring.thanos.io,resources=thanosrulers,verbs=get;list;watch;create;update;patch;delete
// +kubebuilder:rbac:groups=monitoring.thanos.io,resources=thanosrulers/status,verbs=get;update;patch
// +kubebuilder:rbac:groups=monitoring.thanos.io,resources=thanosrulers/finalizers,verbs=update
// +kubebuilder:rbac:groups=apps,resources=statefulsets,verbs=get;list;watch;create;update;patch;delete
// +kubebuilder:rbac:groups="",resources=services;configmaps;serviceaccounts,verbs=get;list;watch;create;update;patch;delete
// +kubebuilder:rbac:groups=policy,resources=poddisruptionbudgets,verbs=get;list;watch;create;update;patch;delete
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
		r.metrics.Paused.WithLabelValues("ruler", ruler.GetName(), ruler.GetNamespace()).Set(1)
		r.recorder.Event(ruler, corev1.EventTypeNormal, "Paused", "Reconciliation is paused for ThanosRuler resource")
		r.updateCondition(ctx, ruler, metav1.Condition{
			Type:    ConditionPaused,
			Status:  metav1.ConditionTrue,
			Reason:  ReasonPaused,
			Message: "Reconciliation is paused",
		})
		return ctrl.Result{}, nil
	}

	r.metrics.Paused.WithLabelValues("ruler", ruler.GetName(), ruler.GetNamespace()).Set(0)

	err = r.syncResources(ctx, *ruler)
	if err != nil {
		r.logger.Error(err, "failed to sync resources", "resource", ruler.GetName(), "namespace", ruler.GetNamespace())
		r.recorder.Event(ruler, corev1.EventTypeWarning, "SyncFailed", fmt.Sprintf("Failed to sync resources: %v", err))
		r.updateCondition(ctx, ruler, metav1.Condition{
			Type:    ConditionReconcileFailed,
			Status:  metav1.ConditionTrue,
			Reason:  ReasonReconcileError,
			Message: err.Error(),
		})
		return ctrl.Result{}, err
	}

	r.updateCondition(ctx, ruler, metav1.Condition{
		Type:    ConditionReconcileSuccess,
		Status:  metav1.ConditionTrue,
		Reason:  ReasonReconcileComplete,
		Message: "Reconciliation completed successfully",
	})

	return ctrl.Result{}, nil
}

func (r *ThanosRulerReconciler) syncResources(ctx context.Context, ruler monitoringthanosiov1alpha1.ThanosRuler) error {
	var objs []client.Object

	opts, expectedPromRuleConfigMaps, err := r.buildRuler(ctx, ruler)
	if err != nil {
		return err
	}
	expectedResources := []string{opts.GetGeneratedResourceName()}

	objs = append(objs, opts.Build()...)

	if errCount := r.handler.CreateOrUpdate(ctx, ruler.GetNamespace(), &ruler, objs); errCount > 0 {
		return fmt.Errorf("failed to create or update %d resources for the ruler", errCount)
	}

	cleanErrCount := r.cleanup(ctx, ruler, expectedResources, expectedPromRuleConfigMaps)
	if cleanErrCount > 0 {
		return fmt.Errorf("failed to clean up %d orphaned resources for the ruler", cleanErrCount)
	}

	return nil
}

func (r *ThanosRulerReconciler) buildRuler(ctx context.Context, ruler monitoringthanosiov1alpha1.ThanosRuler) (manifests.Buildable, []string, error) {
	endpoints, err := r.getQueryAPIServiceEndpoints(ctx, ruler)
	if err != nil {
		return nil, nil, err
	}

	if len(endpoints) == 0 {
		return nil, nil, fmt.Errorf("no query API services found")
	}

	// Get user-provided rule ConfigMaps.
	ruleFiles, expectedUserConfigMapNames, err := r.getRuleConfigMaps(ctx, ruler)
	if err != nil {
		return nil, nil, err
	}
	r.logger.Info("found rule configmaps", "count", len(ruleFiles), "ruler", ruler.Name)

	// Get PrometheusRule-based ConfigMaps
	promRuleConfigMaps := []corev1.ConfigMapKeySelector{}
	expectedPromRuleConfigMapNames := []string{}
	if r.featureGate.PrometheusRuleEnabled() {
		promRuleConfigMaps, expectedPromRuleConfigMapNames, err = r.getPrometheusRuleConfigMaps(ctx, ruler)
		if err != nil {
			return nil, nil, err
		}
		r.logger.Info("found prometheus rule-based configmaps", "count", len(promRuleConfigMaps), "ruler", ruler.Name)
	}

	// Merge expected ConfigMap names for cleanup
	expectedDerivedConfigMapNames := append(expectedUserConfigMapNames, expectedPromRuleConfigMapNames...)

	// PrometheusRule-based configmaps take precedence.
	uniqueRuleFiles := make(map[string]corev1.ConfigMapKeySelector)
	for _, rf := range ruleFiles {
		uniqueRuleFiles[fmt.Sprintf("%s/%s", rf.Name, rf.Key)] = rf
	}
	for _, prf := range promRuleConfigMaps {
		uniqueRuleFiles[fmt.Sprintf("%s/%s", prf.Name, prf.Key)] = prf
	}

	ruleFiles = make([]corev1.ConfigMapKeySelector, 0, len(uniqueRuleFiles))
	for _, rf := range uniqueRuleFiles {
		ruleFiles = append(ruleFiles, rf)
	}

	sort.Slice(ruleFiles, func(i, j int) bool {
		return fmt.Sprintf("%s/%s", ruleFiles[i].Name, ruleFiles[i].Key) < fmt.Sprintf("%s/%s", ruleFiles[j].Name, ruleFiles[j].Key)
	})

	r.logger.Info("total rule files to configure", "count", len(ruleFiles), "ruler", ruler.Name)
	r.metrics.RuleFilesConfigured.WithLabelValues(ruler.GetName(), ruler.GetNamespace()).Set(float64(len(ruleFiles)))

	opts := rulerV1Alpha1ToOptions(ruler, r.featureGate)
	opts.Endpoints = endpoints
	opts.RuleFiles = ruleFiles

	return opts, expectedDerivedConfigMapNames, nil
}

func (r *ThanosRulerReconciler) pruneOrphanedResources(ctx context.Context, ns, owner string, expectedResources []string) int {
	listOpt := manifests.GetLabelSelectorForOwner(manifestruler.Options{Options: manifests.Options{Owner: owner}})
	listOpts := []client.ListOption{listOpt, client.InNamespace(ns)}

	pruner := r.handler.NewResourcePruner().WithServiceAccount().WithService().WithStatefulSet().WithPodDisruptionBudget().WithServiceMonitor()
	return pruner.Prune(ctx, expectedResources, listOpts...)
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

// getRuleConfigMaps returns the list of ruler configmaps of rule files to set on ThanosRuler,
// along with the names of the ConfigMaps created for cleanup purposes.
func (r *ThanosRulerReconciler) getRuleConfigMaps(ctx context.Context, ruler monitoringthanosiov1alpha1.ThanosRuler) ([]corev1.ConfigMapKeySelector, []string, error) {
	if ruler.Spec.RuleConfigSelector.MatchLabels == nil {
		r.logger.Error(fmt.Errorf("no prometheus rule selector specified"), "no prometheus rule selector specified", "ruler", ruler.Name)
		return []corev1.ConfigMapKeySelector{}, []string{}, fmt.Errorf("no prometheus rule selector specified")
	}

	labelSelector, err := manifests.BuildLabelSelectorFrom(&ruler.Spec.RuleConfigSelector, defaultRuleLabels)
	if err != nil {
		return nil, nil, err
	}

	// Add a requirement to exclude derived ConfigMaps (those created from PrometheusRules)
	excludeDerivedReq, err := labels.NewRequirement(
		manifests.PromRuleDerivedConfigMapLabel,
		selection.NotIn,
		[]string{manifests.PromRuleDerivedConfigMapValue},
	)
	if err != nil {
		return nil, nil, err
	}

	// Add a requirement to exclude user-provided ConfigMaps (those created from user-provided ConfigMaps)
	excludeDerivedConfigMapReq, err := labels.NewRequirement(
		manifests.UserConfigMapSourceLabel,
		selection.NotIn,
		[]string{manifests.UserConfigMapSourceValue},
	)
	if err != nil {
		return nil, nil, err
	}

	combinedSelector := labelSelector.Add(*excludeDerivedReq, *excludeDerivedConfigMapReq)
	opts := []client.ListOption{client.MatchingLabelsSelector{Selector: combinedSelector}, client.InNamespace(ruler.Namespace)}
	cfgmaps := &corev1.ConfigMapList{}
	if err := r.List(ctx, cfgmaps, opts...); err != nil {
		return []corev1.ConfigMapKeySelector{}, []string{}, err
	}

	if len(cfgmaps.Items) == 0 {
		r.recorder.Event(&ruler, corev1.EventTypeWarning, "NoRuleConfigsFound", "No rule ConfigMaps found")
		return []corev1.ConfigMapKeySelector{}, []string{}, nil
	}

	r.logger.Info("processing rule config maps",
		"found", len(cfgmaps.Items),
		"ruler", ruler.Name,
		"namespace", ruler.Namespace)

	tenantRuleGroupCount := make(map[string]int)
	tenantRuleGroupCount[""] = 0

	// Collect all rule files from ConfigMaps
	allRuleFiles := make(map[string]string)
	for _, cfgmap := range cfgmaps.Items {
		if cfgmap.Data == nil || len(cfgmap.Data) != 1 {
			r.logger.Info("skipping invalid config map",
				"name", cfgmap.Name,
				"dataKeys", len(cfgmap.Data),
				"ruler", ruler.Name)
			continue
		}

		for key, content := range cfgmap.Data {
			// Parse YAML content to extract rule groups
			groups, err := parseRuleFileContent(content)
			if err != nil {
				r.logger.Error(err, "failed to parse rule file content",
					"configMap", cfgmap.Name,
					"key", key)
				continue
			}

			// Apply tenant labels if configured
			var tenantValue string
			if ruler.Spec.RuleTenancyConfig != nil {
				tenantValueLabel := ruler.Spec.RuleTenancyConfig.TenantValueLabel
				value, exists := cfgmap.Labels[tenantValueLabel]
				if !exists {
					r.logger.Info("tenant value label key not found in ConfigMap labels",
						"tenantValueLabel", tenantValueLabel,
						"configMap", cfgmap.Name)
					tenantRuleGroupCount[""] += len(groups)
				} else {
					if value == "" {
						err := fmt.Errorf("tenant in labels of ConfigMap %s is empty", cfgmap.Name)
						return nil, nil, err
					}
					tenantValue = value

					if _, exists := tenantRuleGroupCount[value]; !exists {
						tenantRuleGroupCount[value] = 0
					}
					tenantRuleGroupCount[value] += len(groups)
				}
			}

			// Process rule groups with tenancy
			processedGroups := r.processRuleGroupsWithTenancy(
				groups,
				ruler.Spec.RuleTenancyConfig,
				tenantValue,
				cfgmap.Name,
			)

			ruleContent := manifestruler.GenerateRuleFileContent(processedGroups)
			// Strip .yaml or .yml extension from key to avoid double extensions
			keyWithoutExt := strings.TrimSuffix(strings.TrimSuffix(key, ".yaml"), ".yml")
			allRuleFiles[fmt.Sprintf("%s-%s.yaml", cfgmap.Name, keyWithoutExt)] = ruleContent
		}
	}

	if ruler.Spec.RuleTenancyConfig != nil {
		for tenant, count := range tenantRuleGroupCount {
			r.metrics.PrometheusRuleGroupsTenantCount.WithLabelValues(ruler.GetName(), ruler.GetNamespace(), tenant).Set(float64(count))
		}
	}

	if len(allRuleFiles) == 0 {
		return []corev1.ConfigMapKeySelector{}, []string{}, nil
	}

	additionalLabels := map[string]string{
		manifests.UserConfigMapSourceLabel: manifests.UserConfigMapSourceValue,
	}
	return r.createBucketedRuleConfigMaps(ctx, ruler, allRuleFiles, "usercfgmap", additionalLabels)
}

// getPrometheusRuleConfigMaps returns the list of ruler configmaps of rule files to set on ThanosRuler,
// along with the names of the ConfigMaps created for cleanup purposes.
func (r *ThanosRulerReconciler) getPrometheusRuleConfigMaps(ctx context.Context, ruler monitoringthanosiov1alpha1.ThanosRuler) ([]corev1.ConfigMapKeySelector, []string, error) {
	if ruler.Spec.RuleConfigSelector.MatchLabels == nil {
		r.logger.Error(fmt.Errorf("no prometheus rule selector specified"), "no prometheus rule selector specified", "ruler", ruler.Name)
		return []corev1.ConfigMapKeySelector{}, []string{}, fmt.Errorf("no prometheus rule selector specified")
	}

	labelSelector, err := manifests.BuildLabelSelectorFrom(&ruler.Spec.RuleConfigSelector, defaultRuleLabels)
	if err != nil {
		return nil, nil, fmt.Errorf("failed to build PrometheusRule label selector: %w", err)
	}

	promRules := &monitoringv1.PrometheusRuleList{}
	if err := r.List(ctx, promRules,
		client.InNamespace(ruler.Namespace),
		client.MatchingLabelsSelector{Selector: labelSelector},
	); err != nil {
		return nil, nil, err
	}

	if len(promRules.Items) == 0 {
		return []corev1.ConfigMapKeySelector{}, []string{}, nil
	}

	r.logger.Info("processing prometheus rules",
		"found", len(promRules.Items),
		"ruler", ruler.Name,
		"namespace", ruler.Namespace)

	r.metrics.PrometheusRulesFound.WithLabelValues(ruler.GetName(), ruler.GetNamespace()).Set(float64(len(promRules.Items)))

	// Process tenant labels if configured
	tenantRuleGroupCount := make(map[string]int)
	tenantRuleGroupCount[""] = 0

	// Collect all rule files from PrometheusRules
	allRuleFiles := make(map[string]string)

	for _, rule := range promRules.Items {
		r.metrics.PrometheusRuleGroupsFound.WithLabelValues(ruler.GetName(), ruler.GetNamespace(), rule.Name).Set(float64(len(rule.Spec.Groups)))

		groups := rule.Spec.Groups

		// Apply tenant labels if configured
		var tenantValue string
		if ruler.Spec.RuleTenancyConfig != nil {
			tenantValueLabel := ruler.Spec.RuleTenancyConfig.TenantValueLabel
			value, exists := rule.Labels[tenantValueLabel]
			if !exists {
				r.logger.Info("tenant value label key not found in PrometheusRule labels",
					"tenantValueLabel", tenantValueLabel,
					"prometheusRule", rule.Name)
				tenantRuleGroupCount[""] += len(groups)
			} else {
				if value == "" {
					err := fmt.Errorf("tenant in labels of PrometheusRule %s is empty", rule.Name)
					return nil, nil, err
				}
				tenantValue = value

				if _, exists := tenantRuleGroupCount[value]; !exists {
					tenantRuleGroupCount[value] = 0
				}
				tenantRuleGroupCount[value] += len(groups)
			}
		}

		// Process rule groups with tenancy
		processedGroups := r.processRuleGroupsWithTenancy(
			groups,
			ruler.Spec.RuleTenancyConfig,
			tenantValue,
			rule.Name,
		)

		// Generate rule file content
		ruleContent := manifestruler.GenerateRuleFileContent(processedGroups)
		allRuleFiles[fmt.Sprintf("%s.yaml", rule.Name)] = ruleContent
	}

	// Record tenant metrics
	if ruler.Spec.RuleTenancyConfig != nil {
		for tenant, count := range tenantRuleGroupCount {
			r.metrics.PrometheusRuleGroupsTenantCount.WithLabelValues(ruler.GetName(), ruler.GetNamespace(), tenant).Set(float64(count))
		}
	}

	if len(allRuleFiles) == 0 {
		return []corev1.ConfigMapKeySelector{}, []string{}, nil
	}

	additionalLabels := map[string]string{
		manifests.PromRuleDerivedConfigMapLabel: manifests.PromRuleDerivedConfigMapValue,
	}
	return r.createBucketedRuleConfigMaps(ctx, ruler, allRuleFiles, "promrule", additionalLabels)
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
		MatchLabels: defaultRuleLabels,
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
		).
		Watches(
			&corev1.ConfigMap{},
			r.enqueueForConfigMap(),
			builder.WithPredicates(configMapPredicate),
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
				r.metrics.ServiceWatchesReconciliationsTotal.WithLabelValues(ruler.GetName(), ruler.GetNamespace()).Inc()
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

// enqueueForConfigMap returns an EventHandler that will enqueue a request for the ThanosRuler instances
// that matches the Service.
func (r *ThanosRulerReconciler) enqueueForConfigMap() handler.EventHandler {
	return handler.EnqueueRequestsFromMapFunc(func(ctx context.Context, obj client.Object) []reconcile.Request {
		if !manifests.HasRequiredLabels(obj, defaultRuleLabels) {
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
			selector, err := manifests.BuildLabelSelectorFrom(&ruler.Spec.RuleConfigSelector, defaultRuleLabels)
			if err != nil {
				r.logger.Error(err, "failed to build label selector from ruler rule config selector", "ruler", ruler.GetName())
				continue
			}
			if selector.Matches(labels.Set(obj.GetLabels())) {
				r.metrics.ConfigMapWatchesReconciliationsTotal.WithLabelValues(ruler.GetName(), ruler.GetNamespace()).Inc()
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

// isQueueableQueryService returns true if the Service is a QueryAPI service that is part of a 'thanos' and has a gRPC port.
func (r *ThanosRulerReconciler) isQueueableQueryService(obj client.Object) bool {
	_, isGRPCSvc := manifests.IsGrpcServiceWithLabels(obj, requiredQueryServiceLabels)
	return isGRPCSvc
}

var requiredQueryServiceLabels = map[string]string{
	manifests.DefaultQueryAPILabel: manifests.DefaultQueryAPIValue,
	manifests.PartOfLabel:          manifests.DefaultPartOfLabel,
}

var defaultRuleLabels = map[string]string{
	manifests.DefaultPrometheusRuleLabel: manifests.DefaultPrometheusRuleValue,
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
			selector, err := manifests.BuildLabelSelectorFrom(&ruler.Spec.RuleConfigSelector, nil)
			if err != nil {
				r.logger.Error(err, "failed to build label selector from ruler PrometheusRule selector",
					"ruler", ruler.GetName())
				continue
			}

			if selector.Matches(labels.Set(obj.GetLabels())) {
				r.metrics.PrometheusRuleWatchesReconciliationsTotal.WithLabelValues(ruler.GetName(), ruler.GetNamespace()).Inc()
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

// parseRuleFileContent parses YAML rule file content and returns rule groups
func parseRuleFileContent(content string) ([]monitoringv1.RuleGroup, error) {
	type ruleFile struct {
		Groups []monitoringv1.RuleGroup `yaml:"groups"`
	}

	var rf ruleFile
	if err := manifestruler.UnmarshalYAML([]byte(content), &rf); err != nil {
		return nil, fmt.Errorf("failed to parse rule file content: %w", err)
	}

	return rf.Groups, nil
}

// enforceTenantLabelInPromQL enforces a tenant label in a PromQL expression.
func enforceTenantLabelInPromQL(expr string, tenantLabel string, tenantValue string) (string, error) {
	matcher := promlabels.Matcher{
		Type:  promlabels.MatchEqual,
		Name:  tenantLabel,
		Value: tenantValue,
	}
	enforcer := injectproxy.NewPromQLEnforcer(true, &matcher)
	expr, err := enforcer.Enforce(expr)
	if err != nil {
		return "", err
	}
	return expr, nil
}

// processRuleGroupsWithTenancy applies tenant labels to rule groups and enforces them in PromQL expressions.
func (r *ThanosRulerReconciler) processRuleGroupsWithTenancy(
	groups []monitoringv1.RuleGroup,
	tenancyConfig *monitoringthanosiov1alpha1.RuleTenancyConfig,
	tenantValue string,
	ruleName string,
) []monitoringv1.RuleGroup {
	if tenancyConfig == nil {
		return groups
	}

	if tenantValue == "" {
		r.logger.Info("tenant value is empty, skipping tenancy processing", "ruleName", ruleName)
		return groups
	}

	tenantLabel := tenancyConfig.TenantLabel

	for i, group := range groups {
		// Set the tenant label on each rule group
		if group.Labels == nil {
			group.Labels = make(map[string]string)
		}
		group.Labels[tenantLabel] = tenantValue

		// Enforce tenant label in PromQL expressions
		for j, ru := range group.Rules {
			exprStr := ru.Expr.String()
			expr, err := enforceTenantLabelInPromQL(exprStr, tenantLabel, tenantValue)
			if err != nil {
				r.logger.Error(err, "failed to enforce tenant label in PromQL",
					"expr", exprStr,
					"tenantLabel", tenantLabel,
					"tenantValue", tenantValue,
					"ruleName", ruleName)
				continue
			}
			ru.Expr = intstr.FromString(expr)
			group.Rules[j] = ru
		}
		groups[i] = group
	}

	return groups
}

// createBucketedRuleConfigMaps creates bucketed ConfigMaps from rule files.
func (r *ThanosRulerReconciler) createBucketedRuleConfigMaps(
	ctx context.Context,
	ruler monitoringthanosiov1alpha1.ThanosRuler,
	allRuleFiles map[string]string,
	namePrefix string,
	additionalLabels map[string]string,
) ([]corev1.ConfigMapKeySelector, []string, error) {
	configMaps, err := manifestruler.MakeRulesConfigMaps(allRuleFiles)
	if err != nil {
		return nil, nil, fmt.Errorf("failed to create config maps for rules: %w", err)
	}

	var ruleFileCfgMaps []corev1.ConfigMapKeySelector
	objs := []client.Object{}
	expectedConfigMapNames := make([]string, 0, len(configMaps))

	// Create ConfigMaps with proper names and metadata
	for i, cm := range configMaps {
		cmName := manifests.SanitizeName(fmt.Sprintf("%s-%s-%d", ruler.Name, namePrefix, i))
		expectedConfigMapNames = append(expectedConfigMapNames, cmName)

		configMap := &corev1.ConfigMap{
			ObjectMeta: metav1.ObjectMeta{
				Name:      cmName,
				Namespace: ruler.Namespace,
				Labels:    manifests.MergeLabels(manifests.MergeLabels(ruler.Spec.RuleConfigSelector.MatchLabels, defaultRuleLabels), additionalLabels),
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

	r.metrics.ConfigMapsCreated.WithLabelValues(ruler.GetName(), ruler.GetNamespace()).Add(float64(len(configMaps)))

	if errCount := r.handler.CreateOrUpdate(ctx, ruler.GetNamespace(), &ruler, objs); errCount > 0 {
		r.metrics.ConfigMapCreationFailures.WithLabelValues(ruler.GetName(), ruler.GetNamespace()).Add(float64(errCount))
		return nil, nil, fmt.Errorf("failed to create or update %d ConfigMaps", errCount)
	}

	return ruleFileCfgMaps, expectedConfigMapNames, nil
}

func (r *ThanosRulerReconciler) cleanup(ctx context.Context, resource monitoringthanosiov1alpha1.ThanosRuler, expectedResources []string, expectedDerivedConfigMaps []string) int {
	var cleanErrCount int
	ns := resource.GetNamespace()
	owner := resource.GetName()

	cleanErrCount = r.pruneOrphanedResources(ctx, ns, owner, expectedResources)

	cleanErrCount += r.handler.DeleteResource(ctx, getDisabledFeatureGatedResources(r.featureGate, []string{RulerNameFromParent(owner)}, ns))

	if resource.Spec.Replicas < 2 {
		listOpt := manifests.GetLabelSelectorForOwner(manifestruler.Options{Options: manifests.Options{Owner: owner}})
		listOpts := []client.ListOption{listOpt, client.InNamespace(ns)}
		cleanErrCount += r.handler.NewResourcePruner().WithPodDisruptionBudget().Prune(ctx, []string{}, listOpts...)
	}

	cleanErrCount += r.pruneOrphanedDerivedConfigMaps(ctx, ns, expectedDerivedConfigMaps)

	return cleanErrCount
}

// pruneOrphanedDerivedConfigMaps prunes orphaned derived ConfigMaps (both PrometheusRule and user ConfigMap sources).
func (r *ThanosRulerReconciler) pruneOrphanedDerivedConfigMaps(ctx context.Context, ns string, expectedDerivedConfigMaps []string) int {
	pruner := r.handler.NewResourcePruner().WithConfigMap()

	listOptsPromRuleDerivedConfigMaps := []client.ListOption{
		client.InNamespace(ns),
		client.MatchingLabels{
			manifests.PromRuleDerivedConfigMapLabel: manifests.PromRuleDerivedConfigMapValue,
		},
	}
	prunedPrd := pruner.Prune(ctx, expectedDerivedConfigMaps, listOptsPromRuleDerivedConfigMaps...)

	listOptsUserConfigMapSourceConfigMaps := []client.ListOption{
		client.InNamespace(ns),
		client.MatchingLabels{
			manifests.UserConfigMapSourceLabel: manifests.UserConfigMapSourceValue,
		},
	}
	prunedUCMD := pruner.Prune(ctx, expectedDerivedConfigMaps, listOptsUserConfigMapSourceConfigMaps...)

	return prunedPrd + prunedUCMD
}

func (r *ThanosRulerReconciler) DisableConditionUpdate() *ThanosRulerReconciler {
	r.disableConditionUpdate = true
	return r
}

// updateCondition updates the status conditions of the ThanosRuler resource
func (r *ThanosRulerReconciler) updateCondition(ctx context.Context, ruler *monitoringthanosiov1alpha1.ThanosRuler, condition metav1.Condition) {
	if r.disableConditionUpdate {
		return
	}
	conditions := ruler.Status.Conditions
	meta.SetStatusCondition(&conditions, condition)
	ruler.Status.Conditions = conditions
	if condition.Type == ConditionPaused {
		ruler.Status.Paused = ptr.To(true)
	}
	if err := r.Status().Update(ctx, ruler); err != nil {
		r.logger.Error(err, "failed to update status for ThanosRuler", "name", ruler.Name)
	}
}
