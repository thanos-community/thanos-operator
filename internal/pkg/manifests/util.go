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

var thanosRepeatableFlags = []string{
	// Query flags
	"--query.replica-label",
	"--query.partition-label",
	"--selector-label",
	"--query.telemetry.request-duration-seconds-quantiles",
	"--query.telemetry.request-samples-quantiles",
	"--query.telemetry.request-series-seconds-quantiles",
	"--store.sd-files",
	"--endpoint",
	"--endpoint-group",
	"--endpoint-strict",
	"--endpoint-group-strict",
	"--inject-test-addresses",
	// Rule flags
	"--label",
	"--rule-file",
	"--grpc-query-endpoint",
	"--query",
	"--query.sd-files",
	// Receive flags
	"--receive.otlp-promote-resource-attributes",
	// Query Frontend flags
	"--query-frontend.forward-header",
}

// isRepeatableFlag checks if a flag allows multiple values
func isRepeatableFlag(flag string) bool {
	for _, repeatableFlag := range thanosRepeatableFlags {
		if flag == repeatableFlag {
			return true
		}
	}
	return false
}

// MergeArgs merges existing args with additional args. For most flags, additional args
// replace existing ones with the same flag name. For repeatable flags (listed in
// thanosRepeatableFlags), additional args are appended rather than replaced.
// The first argument (if it's not a flag) is preserved as the command name.
// Args should be in the format "--flag=value" or "--flag".
func MergeArgs(existingArgs, additionalArgs []string) []string {
	// Prune empty args from inputs
	existingArgs = PruneEmptyArgs(existingArgs)
	additionalArgs = PruneEmptyArgs(additionalArgs)

	if len(additionalArgs) == 0 {
		return existingArgs
	}

	// Preserve the first argument if it's not a flag (cmd name)
	var cmdName string
	var flagArgs []string

	if len(existingArgs) > 0 && !strings.HasPrefix(existingArgs[0], "--") {
		cmdName = existingArgs[0]
		flagArgs = existingArgs[1:]
	} else {
		flagArgs = existingArgs
	}

	// For non-repeatable flags, use map for deduplication
	argMap := make(map[string]string)
	var orderKeys []string

	// For repeatable flags, collect all values
	repeatableArgs := make(map[string][]string)

	// Parse existing flag args (excluding binary name)
	for _, arg := range flagArgs {
		flag := extractFlag(arg)
		if flag != "" {
			if isRepeatableFlag(flag) {
				repeatableArgs[flag] = append(repeatableArgs[flag], arg)
				// Add to order only if not seen before
				if len(repeatableArgs[flag]) == 1 {
					orderKeys = append(orderKeys, flag)
				}
			} else {
				if _, exists := argMap[flag]; !exists {
					orderKeys = append(orderKeys, flag)
				}
				argMap[flag] = arg
			}
		}
	}

	// Parse additional args and override/add
	for _, arg := range additionalArgs {
		flag := extractFlag(arg)
		if flag != "" {
			if isRepeatableFlag(flag) {
				// For repeatable flags, append the additional args
				if _, exists := repeatableArgs[flag]; !exists {
					orderKeys = append(orderKeys, flag)
				}
				repeatableArgs[flag] = append(repeatableArgs[flag], arg)
			} else {
				// For non-repeatable flags, replace existing
				if _, exists := argMap[flag]; !exists {
					orderKeys = append(orderKeys, flag)
				}
				argMap[flag] = arg
			}
		}
	}

	// Rebuild args slice maintaining order
	result := make([]string, 0, len(orderKeys)*2+1) // Estimate capacity including binary name

	// Add binary name first if it exists
	if cmdName != "" {
		result = append(result, cmdName)
	}

	// Add flags in order
	for _, flag := range orderKeys {
		if isRepeatableFlag(flag) {
			// Add all instances of repeatable flags
			result = append(result, repeatableArgs[flag]...)
		} else {
			// Add single instance of non-repeatable flags
			result = append(result, argMap[flag])
		}
	}

	// Apply final pruning to ensure no empty args slip through
	return PruneEmptyArgs(result)
}

// extractFlag extracts the flag name from an argument string.
// For "--flag=value" returns "--flag", for "--flag" returns "--flag".
func extractFlag(arg string) string {
	if !strings.HasPrefix(arg, "--") {
		return ""
	}

	if idx := strings.Index(arg, "="); idx != -1 {
		return arg[:idx]
	}

	return arg
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
