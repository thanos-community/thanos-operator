package handlers

import (
	"context"

	"github.com/go-logr/logr"
	"k8s.io/apimachinery/pkg/api/errors"

	"github.com/thanos-community/thanos-operator/internal/pkg/manifests"

	"k8s.io/apimachinery/pkg/runtime"

	ctrl "sigs.k8s.io/controller-runtime"
	"sigs.k8s.io/controller-runtime/pkg/client"
)

type Handler struct {
	client client.Client
	scheme *runtime.Scheme
	logger logr.Logger
}

// NewHandler creates a new Handler.
func NewHandler(client client.Client, scheme *runtime.Scheme, logger logr.Logger) *Handler {
	return &Handler{
		client: client,
		scheme: scheme,
		logger: logger,
	}
}

// CreateOrUpdate creates or updates the given objects in the Kubernetes cluster.
// It sets the owner reference of each object to the given owner.
// It logs the operation and any errors encountered.
// It returns the number of errors encountered.
func (h *Handler) CreateOrUpdate(ctx context.Context, namespace string, owner client.Object, objs []client.Object) int {
	var errCount int
	for _, obj := range objs {
		if manifests.IsNamespacedResource(obj) {
			obj.SetNamespace(namespace)
			if err := ctrl.SetControllerReference(owner, obj, h.scheme); err != nil {
				h.logger.Error(err, "failed to set controller owner reference to resource")
				errCount++
				continue
			}
		}

		desired := obj.DeepCopyObject().(client.Object)
		mutateFn := manifests.MutateFuncFor(obj, desired)

		op, err := ctrl.CreateOrUpdate(ctx, h.client, obj, mutateFn)
		if err != nil {
			h.logger.Error(
				err, "failed to create or update resource",
				"gvk", obj.GetObjectKind().GroupVersionKind().String(),
				"resource", obj.GetName(),
				"namespace", obj.GetNamespace(),
			)
			errCount++
			continue
		}

		h.logger.V(1).Info(
			"resource configured",
			"operation", op, "gvk", obj.GetObjectKind().GroupVersionKind().String(),
			"resource", obj.GetName(), "namespace", obj.GetNamespace(),
		)
	}
	return errCount
}

// Delete resources if they exist.
func (h *Handler) DeleteResource(ctx context.Context, objs []client.Object) int {
	var errCount int
	for _, obj := range objs {
		if err := h.client.Delete(ctx, obj); err != nil && !errors.IsNotFound(err) {
			h.logger.Error(
				err, "failed to delete resource",
				"gvk", obj.GetObjectKind().GroupVersionKind().String(),
				"resource", obj.GetName(),
				"namespace", obj.GetNamespace(),
			)
			errCount++
			continue
		}

		h.logger.V(1).Info(
			"resource deleted",
			"gvk", obj.GetObjectKind().GroupVersionKind().String(),
			"resource", obj.GetName(),
			"namespace", obj.GetNamespace(),
		)
	}
	return errCount
}
