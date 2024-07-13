package manifests

import (
	"fmt"

	rbacv1 "k8s.io/api/rbac/v1"
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

// OptionalToString returns the string representation of the given pointer
// or an empty string if the pointer is nil. Helps avoid bunch of typecasts and nil-checks throughout.
// Works only for basic types and not complex ones like slices, structs and maps.
func OptionalToString[T any](ptr *T) string {
	if ptr == nil {
		return ""
	}
	return fmt.Sprintf("%v", *ptr)
}
