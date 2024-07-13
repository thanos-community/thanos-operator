package manifests

import (
	"fmt"
	"strings"

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

// PruneEmptyArgs removes empty or empty value arguments from the given slice.
func PruneEmptyArgs(args []string) []string {
	for i := 0; i < len(args); i++ {
		if args[i] == "" || strings.HasSuffix(args[i], "=") {
			args = append(args[:i], args[i+1:]...)
		}
	}

	return args
}
