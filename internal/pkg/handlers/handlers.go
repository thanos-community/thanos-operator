package handlers

import (
	"context"
	"fmt"
	"slices"

	"github.com/go-logr/logr"
	monitoringv1 "github.com/prometheus-operator/prometheus-operator/pkg/apis/monitoring/v1"

	"github.com/thanos-community/thanos-operator/internal/pkg/certificates"
	"github.com/thanos-community/thanos-operator/internal/pkg/featuregate"
	"github.com/thanos-community/thanos-operator/internal/pkg/manifests"

	appsv1 "k8s.io/api/apps/v1"
	corev1 "k8s.io/api/core/v1"
	discoveryv1 "k8s.io/api/discovery/v1"
	policyv1 "k8s.io/api/policy/v1"
	"k8s.io/apimachinery/pkg/api/errors"
	"k8s.io/apimachinery/pkg/api/meta"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/runtime"

	ctrl "sigs.k8s.io/controller-runtime"
	"sigs.k8s.io/controller-runtime/pkg/client"
)

type Handler struct {
	*handler
}

type handler struct {
	client client.Client
	scheme *runtime.Scheme
	logger logr.Logger

	features featuregate.Config
}

// resourcePruner creates an object that prunes resources in the Kubernetes cluster.
type resourcePruner struct {
	*handler
	sa, svc, sts, dep, cm, secret, pdb, svcMon bool
}

// NewHandler creates a new Handler.
func NewHandler(client client.Client, scheme *runtime.Scheme, logger logr.Logger) *Handler {
	return &Handler{
		handler: &handler{
			client: client,
			scheme: scheme,
			logger: logger,
		},
	}
}

func (h *Handler) WithTLS(features featuregate.Config) *Handler {
	h.features = features
	return h
}

// CreateOrUpdate creates or updates the given objects in the Kubernetes cluster.
// It sets the owner reference of each object to the given owner.
// It logs the operation and any errors encountered.
// It returns the number of errors encountered.
func (h *Handler) CreateOrUpdate(ctx context.Context, namespace string, owner client.Object, objs []client.Object) int {
	var errCount int
	tlsManager := certificates.Manager{Client: h.client, Scheme: h.scheme}
	if h.features.TLSEnabled() {
		tlsManager.Config = *h.features.TLS
		for _, obj := range objs {
			if template := manifests.PodTemplate(obj); template != nil {
				if err := manifests.ValidateTLSWorkload(template); err != nil {
					h.logger.Error(err, "invalid TLS workload", "name", obj.GetName())
					return 1
				}
			}
		}
		if err := tlsManager.EnsureNamespace(ctx, namespace); err != nil {
			h.logger.Error(err, "failed to reconcile TLS trust", "namespace", namespace)
			return 1
		}
	}
	for _, obj := range objs {
		logger := loggerForObj(h.logger, obj)
		if manifests.IsNamespacedResource(obj) {
			obj.SetNamespace(namespace)
			if err := ctrl.SetControllerReference(owner, obj, h.scheme); err != nil {
				logger.Error(err, "failed to set controller owner reference to resource")
				errCount++
				continue
			}
		}

		if h.features.TLSEnabled() && manifests.PodTemplate(obj) != nil {
			if err := tlsManager.SetChecksum(ctx, obj); err != nil {
				logger.Error(err, "failed to read TLS material")
				errCount++
				continue
			}
		}
		desired := obj.DeepCopyObject().(client.Object)
		mutateFn := manifests.MutateFuncFor(obj, desired)

		op, err := ctrl.CreateOrUpdate(ctx, h.client, obj, mutateFn)

		if err != nil {
			logger.Error(err, "failed to create or update resource")
			errCount++
			continue
		}
		logger.V(1).Info("resource configured", "operation", op)
		if manifests.PodTemplate(obj) != nil {
			var err error
			if h.features.TLSEnabled() {
				err = tlsManager.EnsureWorkload(ctx, obj)
			} else {
				err = tlsManager.CleanupWorkload(ctx, obj)
			}
			if err != nil {
				logger.Error(err, "failed to reconcile workload certificates")
				errCount++
			}
		}
	}
	return errCount
}

// DeleteResource if they exist in the Kubernetes cluster.
// It reads the item from the cache initially to see if it is present.
// It issues a DELETE request to the Kubernetes API server if it exists.
func (h *Handler) DeleteResource(ctx context.Context, objs []client.Object) int {
	var errCount int
	for _, obj := range objs {
		logger := loggerForObj(h.logger, obj)

		err := h.client.Get(ctx, client.ObjectKeyFromObject(obj), obj)
		if err != nil {
			if errors.IsNotFound(err) || meta.IsNoMatchError(err) {
				continue
			}

			logger.Error(err, "failed to get resource in DeleteResource handler")
			errCount++
			continue
		}

		if err := h.deleteResource(ctx, obj); err != nil {
			errCount++
			continue
		}
	}
	return errCount
}

// deleteResource deletes the given objects in the Kubernetes cluster.
// It logs the operation and any errors encountered.
// If the item does not exist, it does not return an error.
func (h *handler) deleteResource(ctx context.Context, obj client.Object) error {
	logger := loggerForObj(h.logger, obj)
	if err := h.client.Delete(ctx, obj); err != nil && !errors.IsNotFound(err) && !meta.IsNoMatchError(err) {
		logger.Error(err, fmt.Sprintf("failed to delete resource %s, name: %s",
			obj.GetObjectKind().GroupVersionKind().GroupKind().String(),
			obj.GetName(),
		))

		return err
	}

	logger.V(1).Info("resource deleted")
	return nil
}

// GetEndpointSlices returns the EndpointSlices for the given service in the given namespace.
func (h *Handler) GetEndpointSlices(ctx context.Context, serviceName string, namespace string) (*discoveryv1.EndpointSliceList, error) {
	selectorListOpt := client.MatchingLabels{discoveryv1.LabelServiceName: serviceName}
	eps := discoveryv1.EndpointSliceList{}
	if err := h.client.List(ctx, &eps, selectorListOpt, client.InNamespace(namespace)); err != nil {
		return nil, fmt.Errorf("failed to list endpoint slices for service %s in namespace %s: %w",
			serviceName, namespace, err)
	}
	return &eps, nil
}

// NewResourcePruner creates a new resourcePruner.
func (h *Handler) NewResourcePruner() *resourcePruner {
	return &resourcePruner{
		handler: h.handler,
	}
}

// WithConfigMap returns a resourcePruner with ConfigMap enabled.
func (r *resourcePruner) WithConfigMap() *resourcePruner {
	r.cm = true
	return r
}

// WithSecret returns a resourcePruner with Secret enabled.
func (r *resourcePruner) WithSecret() *resourcePruner {
	r.secret = true
	return r
}

// WithService returns a resourcePruner with Service enabled.
func (r *resourcePruner) WithService() *resourcePruner {
	r.svc = true
	return r
}

// WithStatefulSet returns a resourcePruner with StatefulSet enabled.
func (r *resourcePruner) WithStatefulSet() *resourcePruner {
	r.sts = true
	return r
}

// WithDeployment returns a resourcePruner with Deployment enabled.
func (r *resourcePruner) WithDeployment() *resourcePruner {
	r.dep = true
	return r
}

// WithServiceMonitor returns a resourcePruner with ServiceMonitor enabled.
func (r *resourcePruner) WithServiceMonitor() *resourcePruner {
	r.svcMon = true
	return r
}

// WithServiceAccount returns a resourcePruner with ServiceAccount enabled.
func (r *resourcePruner) WithServiceAccount() *resourcePruner {
	r.sa = true
	return r
}

// WithPodDisruptionBudget returns a resourcePruner with PodDisruptionBudget enabled.
func (r *resourcePruner) WithPodDisruptionBudget() *resourcePruner {
	r.pdb = true
	return r
}

// Prune deletes resources that are not in the keepResourceNames list.
// It acts on the resources enabled in the resourcePruner.
// It logs the operation and any errors encountered.
// It returns the number of errors encountered.
func (r *resourcePruner) Prune(ctx context.Context, keepResourceNames []string, listOpts ...client.ListOption) int {
	return r.prune(ctx, func(obj client.Object) bool {
		return !slices.Contains(keepResourceNames, obj.GetName())
	}, listOpts...)
}

// PruneByOwner deletes enabled resources controlled by owner in its namespace.
// It returns the number of errors encountered.
func (r *resourcePruner) PruneByOwner(ctx context.Context, owner client.Object, listOpts ...client.ListOption) int {
	if owner.GetUID() == "" {
		return 0
	}

	listOpts = append(listOpts, client.InNamespace(owner.GetNamespace()))
	return r.prune(ctx, func(obj client.Object) bool {
		return metav1.IsControlledBy(obj, owner)
	}, listOpts...)
}

func (r *resourcePruner) prune(ctx context.Context, shouldDelete func(client.Object) bool, listOpts ...client.ListOption) int {
	var errCount int
	resourceTypes := []struct {
		enabled bool
		list    client.ObjectList
	}{
		{r.sa, &corev1.ServiceAccountList{}},
		{r.svc, &corev1.ServiceList{}},
		{r.sts, &appsv1.StatefulSetList{}},
		{r.dep, &appsv1.DeploymentList{}},
		{r.cm, &corev1.ConfigMapList{}},
		{r.secret, &corev1.SecretList{}},
		{r.pdb, &policyv1.PodDisruptionBudgetList{}},
		{r.svcMon, &monitoringv1.ServiceMonitorList{}},
	}

	for _, rt := range resourceTypes {
		if rt.enabled {
			if err := r.client.List(ctx, rt.list, listOpts...); err != nil {
				if errors.IsNotFound(err) || meta.IsNoMatchError(err) {
					continue
				}
				r.logger.Error(err, "failed to list resources for pruning")
				errCount++
				continue
			}
			items, err := meta.ExtractList(rt.list)
			if err != nil {
				r.logger.Error(err, "failed to extract list items")
				errCount++
				continue
			}
			for _, item := range items {
				obj := item.(client.Object)
				if !shouldDelete(obj) {
					continue
				}
				if err := r.deleteResource(ctx, obj); err != nil {
					errCount++
					continue
				}
			}
		}
	}
	return errCount
}

func loggerForObj(logger logr.Logger, obj client.Object) logr.Logger {
	return logger.WithValues("name", obj.GetName(), "namespace", obj.GetNamespace(), "kind", obj.GetObjectKind().GroupVersionKind().Kind)
}
