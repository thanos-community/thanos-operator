package handlers

import (
	"context"
	"fmt"
	slices0 "slices"

	"github.com/go-logr/logr"
	monitoringv1 "github.com/prometheus-operator/prometheus-operator/pkg/apis/monitoring/v1"

	"github.com/thanos-community/thanos-operator/internal/pkg/manifests"

	appsv1 "k8s.io/api/apps/v1"
	corev1 "k8s.io/api/core/v1"
	discoveryv1 "k8s.io/api/discovery/v1"
	policyv1 "k8s.io/api/policy/v1"
	"k8s.io/apimachinery/pkg/api/errors"
	"k8s.io/apimachinery/pkg/api/meta"
	"k8s.io/apimachinery/pkg/runtime"
	"k8s.io/apimachinery/pkg/runtime/schema"
	"k8s.io/utils/strings/slices"

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

	gatedGVK []schema.GroupVersionKind
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

// SetFeatureGates sets the feature gates for the handler.
// Handler will ignore actions on resources with the given GroupVersionKind.
func (h *Handler) SetFeatureGates(gvk []schema.GroupVersionKind) {
	h.gatedGVK = gvk
}

// CreateOrUpdate creates or updates the given objects in the Kubernetes cluster.
// It sets the owner reference of each object to the given owner.
// It logs the operation and any errors encountered.
// It returns the number of errors encountered.
func (h *Handler) CreateOrUpdate(ctx context.Context, namespace string, owner client.Object, objs []client.Object) int {
	var errCount int
	for _, obj := range objs {
		logger := loggerForObj(h.logger, obj)
		if h.IsFeatureGated(obj) {
			logger.V(1).Info("resource is feature gated, skipping")
			continue
		}

		if manifests.IsNamespacedResource(obj) {
			obj.SetNamespace(namespace)
			if err := ctrl.SetControllerReference(owner, obj, h.scheme); err != nil {
				logger.Error(err, "failed to set controller owner reference to resource")
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
	}
	return errCount
}

// IsFeatureGated returns true if the given object is feature gated.
func (h *handler) IsFeatureGated(obj client.Object) bool {
	gvk := obj.GetObjectKind().GroupVersionKind()
	return slices0.Contains(h.gatedGVK, gvk)
}

// DeleteResource if they exist in the Kubernetes cluster.
// It reads the item from the cache initially to see if it is present.
// It issues a DELETE request to the Kubernetes API server if it exists.
func (h *Handler) DeleteResource(ctx context.Context, objs []client.Object) int {
	var errCount int
	for _, obj := range objs {
		logger := loggerForObj(h.logger, obj)

		if h.IsFeatureGated(obj) {
			logger.V(1).Info("resource is feature gated, skipping")
			continue
		}

		err := h.client.Get(ctx, client.ObjectKeyFromObject(obj), obj)
		if err != nil {
			if errors.IsNotFound(err) {
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
	if err := h.client.Delete(ctx, obj); err != nil && !errors.IsNotFound(err) {
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
	var errCount int
	deleteOrphanedResources := func(obj client.Object) error {
		if !slices.Contains(keepResourceNames, obj.GetName()) {
			return r.deleteResource(ctx, obj)
		}
		return nil
	}

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
				if err := deleteOrphanedResources(item.(client.Object)); err != nil {
					errCount++
					continue
				}
			}
		}
	}
	return errCount
}

// ExpandPVCsForStatefulSet expands PVCs for a StatefulSet if the desired storage size is larger than the current size.
// This function should be called AFTER the StatefulSet's VolumeClaimTemplates have been updated to the new size.
// It lists all existing PVCs that belong to the StatefulSet and updates their storage request if needed.
//
// Returns the number of errors encountered.
// Note: Volume expansion must be supported by the storage class (allowVolumeExpansion: true).
func (h *Handler) ExpandPVCsForStatefulSet(ctx context.Context, sts *appsv1.StatefulSet, desiredSize corev1.ResourceList) int {
	var errCount int
	logger := h.logger.WithValues("statefulset", sts.GetName(), "namespace", sts.GetNamespace())

	desiredStorageSize := desiredSize[corev1.ResourceStorage]

	// Don't expand PVCs if the StatefulSet update failed
	if len(sts.Spec.VolumeClaimTemplates) > 0 {
		templateSize := sts.Spec.VolumeClaimTemplates[0].Spec.Resources.Requests[corev1.ResourceStorage]
		if templateSize.Cmp(desiredStorageSize) != 0 {
			logger.Info("skipping PVC expansion: StatefulSet VolumeClaimTemplate size doesn't match desired size", "templateSize", templateSize.String(), "desiredSize", desiredStorageSize.String())
			// Don't return error here as the StatefulSet might not have been updated yet
			return 0
		}
	}

	pvcList := &corev1.PersistentVolumeClaimList{}
	listOpts := []client.ListOption{
		client.InNamespace(sts.GetNamespace()),
		client.MatchingLabels(sts.Spec.Selector.MatchLabels),
	}
	if err := h.client.List(ctx, pvcList, listOpts...); err != nil {
		logger.Error(err, "failed to list PVCs for StatefulSet")
		return 1
	}

	for i := range pvcList.Items {
		pvc := &pvcList.Items[i]
		pvcLogger := logger.WithValues("pvc", pvc.GetName())

		currentSize := pvc.Spec.Resources.Requests[corev1.ResourceStorage]

		// Only expand if desired size is larger than current size.
		// Volume shrinking is not supported by Kubernetes.
		if desiredStorageSize.Cmp(currentSize) > 0 {
			pvcLogger.Info("expanding PVC storage", "currentSize", currentSize.String(), "desiredSize", desiredStorageSize.String())

			pvcCopy := pvc.DeepCopy()
			pvcCopy.Spec.Resources.Requests[corev1.ResourceStorage] = desiredStorageSize
			if err := h.client.Update(ctx, pvcCopy); err != nil {
				// Check if the error is due to immutable field (expansion not supported)
				if errors.IsForbidden(err) || errors.IsInvalid(err) {
					pvcLogger.Error(err, "PVC expansion not supported, ensure StorageClass has allowVolumeExpansion=true", "storageClass", pvc.Spec.StorageClassName, "currentSize", currentSize.String(), "desiredSize", desiredStorageSize.String())
				} else {
					pvcLogger.Error(err, "failed to update PVC storage request")
				}
				errCount++
				continue
			}

			pvcLogger.Info("successfully updated PVC storage request, volume expansion will be performed by Kubernetes")
		}
	}

	return errCount
}

func loggerForObj(logger logr.Logger, obj client.Object) logr.Logger {
	return logger.WithValues("name", obj.GetName(), "namespace", obj.GetNamespace(), "kind", obj.GetObjectKind().GroupVersionKind().Kind)
}
