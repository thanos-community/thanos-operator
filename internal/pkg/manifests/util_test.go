package manifests

import (
	"testing"

	rbacv1 "k8s.io/api/rbac/v1"
)

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
