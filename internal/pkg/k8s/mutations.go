package k8s

import (
	"context"
	"fmt"

	monitoringthanosiov1alpha1 "github.com/thanos-community/thanos-operator/api/v1alpha1"

	corev1 "k8s.io/api/core/v1"
	"k8s.io/utils/ptr"

	"sigs.k8s.io/controller-runtime/pkg/client"
)

// GetDependentAnnotations returns the annotations of the dependent object.
func GetDependentAnnotations(ctx context.Context, k client.Client, obj client.Object) (map[string]string, error) {
	a := obj.GetAnnotations()
	saName, ok := a[corev1.ServiceAccountNameKey]
	if !ok || saName == "" {
		return nil, nil
	}

	key := client.ObjectKey{Name: saName, Namespace: obj.GetNamespace()}

	sa := corev1.ServiceAccount{}
	if err := k.Get(ctx, key, &sa); err != nil {
		return nil, fmt.Errorf("failed to fetch associated serviceaccount uid with name %s:%w", key.Name, err)
	}

	return map[string]string{
		corev1.ServiceAccountUIDKey: string(sa.UID),
	}, nil
}

func ToSecretKeySelector(objStoreConfig monitoringthanosiov1alpha1.ObjectStorageConfig) corev1.SecretKeySelector {
	return corev1.SecretKeySelector{
		LocalObjectReference: corev1.LocalObjectReference{Name: objStoreConfig.Name},
		Key:                  objStoreConfig.Key,
		Optional:             ptr.To(false),
	}
}
