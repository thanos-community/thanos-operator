package k8s

import (
	"reflect"
	"testing"

	"github.com/thanos-community/thanos-operator/api/v1alpha1"

	corev1 "k8s.io/api/core/v1"
	rbacv1 "k8s.io/api/rbac/v1"
	"k8s.io/utils/ptr"
)

func TestToSecretKeySelector(t *testing.T) {
	from := v1alpha1.ObjectStorageConfig{
		LocalObjectReference: corev1.LocalObjectReference{
			Name: "test",
		},
		Key: "test",
	}

	expect := corev1.SecretKeySelector{
		LocalObjectReference: corev1.LocalObjectReference{
			Name: "test",
		},
		Key:      "test",
		Optional: ptr.To(false),
	}
	result := ToSecretKeySelector(from)
	if !reflect.DeepEqual(result, expect) {
		t.Fatalf("unexpected result: %v", result)
	}
}

func TestIsNamespacedResource(t *testing.T) {
	// Test for ClusterRole
	if IsNamespacedResource(&rbacv1.ClusterRole{}) {
		t.Errorf("ClusterRole should not be namespaced")
	}

	// Test for ClusterRoleBinding
	if IsNamespacedResource(&rbacv1.ClusterRoleBinding{}) {
		t.Errorf("ClusterRoleBinding should not be namespaced")
	}

	// Test for Role
	if !IsNamespacedResource(&rbacv1.Role{}) {
		t.Errorf("Role should be namespaced")
	}

	// Test for RoleBinding
	if !IsNamespacedResource(&rbacv1.RoleBinding{}) {
		t.Errorf("RoleBinding should be namespaced")
	}
}
