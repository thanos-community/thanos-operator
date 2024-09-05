package manifests

import (
	"fmt"
	"strings"

	corev1 "k8s.io/api/core/v1"
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
	for i := len(args) - 1; i >= 0; i-- {
		if args[i] == "" || strings.HasSuffix(args[i], "=") {
			args = append(args[:i], args[i+1:]...)
		}
	}

	return args
}

// IsGrpcServiceWithLabels returns true if the given object is a gRPC service with required labels.
// The requiredLabels map is used to match the labels of the object.
// The function returns false if the object is not a service or if it does not have a gRPC port.
// The function returns true, alongside the port if the object is a service with a gRPC port and has the required labels.
func IsGrpcServiceWithLabels(obj client.Object, requiredLabels map[string]string) (int32, bool) {
	if !HasRequiredLabels(obj, requiredLabels) {
		return 0, false
	}

	svc, ok := obj.(*corev1.Service)
	if !ok {
		return 0, false
	}

	for _, port := range svc.Spec.Ports {
		if port.Name == "grpc" {
			return port.Port, true
		}
	}
	return 0, false
}

// HasRequiredLabels returns true if the given object has the required labels.
func HasRequiredLabels(obj client.Object, requiredLabels map[string]string) bool {
	for k, v := range requiredLabels {
		if obj.GetLabels()[k] != v {
			return false
		}
	}
	return true
}
