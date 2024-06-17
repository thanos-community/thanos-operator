package k8s

import (
	monitoringthanosiov1alpha1 "github.com/thanos-community/thanos-operator/api/v1alpha1"
	corev1 "k8s.io/api/core/v1"
	rbacv1 "k8s.io/api/rbac/v1"
	"k8s.io/utils/ptr"
	"sigs.k8s.io/controller-runtime/pkg/client"
)

// IsNamespacedResource returns true if the given object is namespaced.
func IsNamespacedResource(obj client.Object) bool {
	switch obj.(type) {
	case *rbacv1.ClusterRole, *rbacv1.ClusterRoleBinding:
		return false
	default:
		return true
	}
}

func ToSecretKeySelector(objStoreConfig monitoringthanosiov1alpha1.ObjectStorageConfig) corev1.SecretKeySelector {
	return corev1.SecretKeySelector{
		LocalObjectReference: corev1.LocalObjectReference{Name: objStoreConfig.Name},
		Key:                  objStoreConfig.Key,
		Optional:             ptr.To(false),
	}
}
