package controller

import (
	"context"

	"k8s.io/apimachinery/pkg/types"
	"sigs.k8s.io/controller-runtime/pkg/client"
)

// patchObjectStatus re-reads obj, applies mutate to the latest copy, and patches only the
// fields changed by mutate. This avoids clobbering concurrent status writers.
func patchObjectStatus[T client.Object](ctx context.Context, c client.Client, key types.NamespacedName, obj T, mutate func(T) error) error {
	if err := c.Get(ctx, key, obj); err != nil {
		return err
	}

	original := obj.DeepCopyObject().(T)
	if err := mutate(obj); err != nil {
		return err
	}

	return c.Status().Patch(ctx, obj, client.MergeFrom(original))
}
