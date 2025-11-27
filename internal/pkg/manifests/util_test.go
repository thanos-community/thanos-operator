package manifests

import (
	"testing"

	corev1 "k8s.io/api/core/v1"
	rbacv1 "k8s.io/api/rbac/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
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

func TestOptionalToString(t *testing.T) {
	type Custom string

	var intPointer *int
	var strPointer *string
	var boolPointer *bool
	var floatPointer *float64
	var customPointer *Custom
	var unsetNilPointer *int

	c := Custom("custom")
	i := 42
	s := "hello"
	b := true
	f := 3.14

	intPointer = &i
	strPointer = &s
	boolPointer = &b
	floatPointer = &f
	customPointer = &c

	tests := []struct {
		name     string
		input    any
		expected string
	}{
		{"int pointer", intPointer, "42"},
		{"string pointer", strPointer, "hello"},
		{"bool pointer", boolPointer, "true"},
		{"float pointer", floatPointer, "3.14"},
		{"custom pointer", customPointer, "custom"},
		{"unset nil pointer", unsetNilPointer, ""},
		{"nil int pointer", (*int)(nil), ""},
		{"nil string pointer", (*string)(nil), ""},
		{"nil bool pointer", (*bool)(nil), ""},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			var result string
			switch v := tt.input.(type) {
			case *int:
				result = OptionalToString(v)
			case *string:
				result = OptionalToString(v)
			case *bool:
				result = OptionalToString(v)
			case *float64:
				result = OptionalToString(v)
			case *Custom:
				result = OptionalToString(v)
			default:
				t.Fatalf("unsupported type: %T", v)
			}
			if result != tt.expected {
				t.Errorf("expected %v, got %v", tt.expected, result)
			}
		})
	}
}

func TestIsGrpcServiceWithLabels(t *testing.T) {
	svc := &corev1.Service{
		ObjectMeta: metav1.ObjectMeta{
			Labels: map[string]string{
				"app": "thanos-query",
			},
		},
		Spec: corev1.ServiceSpec{
			Ports: []corev1.ServicePort{
				{
					Name: "grpc",
					Port: 9090,
				},
			},
		},
	}

	// Test for missing required labels
	if _, ok := IsGrpcServiceWithLabels(svc, map[string]string{"app": "thanos-query", "env": "prod"}); ok {
		t.Errorf("expected false, got true")
	}

	// Test for valid service
	port, ok := IsGrpcServiceWithLabels(svc, map[string]string{"app": "thanos-query"})
	if !ok {
		t.Errorf("expected true, got false")
	}
	if port != 9090 {
		t.Errorf("expected port 9090, got %d", port)
	}
}

func TestHasRequiredLabels(t *testing.T) {
	svc := &corev1.Service{
		ObjectMeta: metav1.ObjectMeta{
			Labels: map[string]string{
				"app": "thanos-query",
			},
		},
	}

	// Test for missing required labels
	if HasRequiredLabels(svc, map[string]string{"app": "thanos-query", "env": "prod"}) {
		t.Errorf("expected false, got true")
	}

	// Test for valid service
	if !HasRequiredLabels(svc, map[string]string{"app": "thanos-query"}) {
		t.Errorf("expected true, got false")
	}
}

func TestMergeArgs(t *testing.T) {
	tests := []struct {
		name       string
		existing   []string
		additional []string
		expected   []string
	}{
		{
			name:       "empty additional args",
			existing:   []string{"--log.level=info", "--query.timeout=5m"},
			additional: []string{},
			expected:   []string{"--log.level=info", "--query.timeout=5m"},
		},
		{
			name:       "empty existing args",
			existing:   []string{},
			additional: []string{"--log.level=debug", "--query.timeout=10m"},
			expected:   []string{"--log.level=debug", "--query.timeout=10m"},
		},
		{
			name:       "no duplicate flags",
			existing:   []string{"--log.level=info", "--query.timeout=5m"},
			additional: []string{"--store.grpc.max-connections=100"},
			expected:   []string{"--log.level=info", "--query.timeout=5m", "--store.grpc.max-connections=100"},
		},
		{
			name:       "replace duplicate flag",
			existing:   []string{"--log.level=info", "--query.timeout=5m"},
			additional: []string{"--log.level=debug"},
			expected:   []string{"--log.level=debug", "--query.timeout=5m"},
		},
		{
			name:       "replace multiple duplicate flags",
			existing:   []string{"--log.level=info", "--query.timeout=5m", "--store.grpc.max-connections=50"},
			additional: []string{"--log.level=debug", "--store.grpc.max-connections=100"},
			expected:   []string{"--log.level=debug", "--query.timeout=5m", "--store.grpc.max-connections=100"},
		},
		{
			name:       "flag without value",
			existing:   []string{"--log.level=info", "--help"},
			additional: []string{"--verbose"},
			expected:   []string{"--log.level=info", "--help", "--verbose"},
		},
		{
			name:       "replace flag without value",
			existing:   []string{"--log.level=info", "--help"},
			additional: []string{"--help"},
			expected:   []string{"--log.level=info", "--help"},
		},
		{
			name:       "mixed flag types",
			existing:   []string{"--log.level=info", "--help", "--query.timeout=5m"},
			additional: []string{"--log.level=debug", "--verbose", "--help"},
			expected:   []string{"--log.level=debug", "--help", "--query.timeout=5m", "--verbose"},
		},
		{
			name:       "preserve order from existing args",
			existing:   []string{"--log.level=info", "--query.timeout=5m", "--store.grpc.max-connections=50"},
			additional: []string{"--query.timeout=10m", "--log.level=debug"},
			expected:   []string{"--log.level=debug", "--query.timeout=10m", "--store.grpc.max-connections=50"},
		},
		{
			name:       "ignore non-flag arguments",
			existing:   []string{"--log.level=info", "query", "--query.timeout=5m"},
			additional: []string{"--log.level=debug", "store"},
			expected:   []string{"--log.level=debug", "--query.timeout=5m"},
		},
		{
			name:       "empty string args filtered out",
			existing:   []string{"--log.level=info", "", "--query.timeout=5m"},
			additional: []string{"--log.level=debug", ""},
			expected:   []string{"--log.level=debug", "--query.timeout=5m"},
		},
		{
			name:       "empty value args filtered out",
			existing:   []string{"--log.level=info", "--empty=", "--query.timeout=5m"},
			additional: []string{"--log.level=debug", "--another-empty="},
			expected:   []string{"--log.level=debug", "--query.timeout=5m"},
		},
		{
			name:       "repeatable flag - append additional args",
			existing:   []string{"--log.level=info", "--query.replica-label=replica"},
			additional: []string{"--query.replica-label=instance"},
			expected:   []string{"--log.level=info", "--query.replica-label=replica", "--query.replica-label=instance"},
		},
		{
			name:       "repeatable flag - multiple existing and additional",
			existing:   []string{"--query.replica-label=replica", "--query.replica-label=cluster", "--log.level=info"},
			additional: []string{"--query.replica-label=instance", "--query.replica-label=zone"},
			expected:   []string{"--query.replica-label=replica", "--query.replica-label=cluster", "--query.replica-label=instance", "--query.replica-label=zone", "--log.level=info"},
		},
		{
			name:       "mix repeatable and non-repeatable flags",
			existing:   []string{"--log.level=info", "--query.replica-label=replica", "--query.timeout=5m"},
			additional: []string{"--log.level=debug", "--query.replica-label=instance", "--query.timeout=10m"},
			expected:   []string{"--log.level=debug", "--query.replica-label=replica", "--query.replica-label=instance", "--query.timeout=10m"},
		},
		{
			name:       "repeatable flag - only additional args",
			existing:   []string{"--log.level=info"},
			additional: []string{"--query.replica-label=replica", "--query.replica-label=instance"},
			expected:   []string{"--log.level=info", "--query.replica-label=replica", "--query.replica-label=instance"},
		},
		{
			name:       "repeatable flag - only existing args",
			existing:   []string{"--query.replica-label=replica", "--query.replica-label=instance"},
			additional: []string{"--log.level=debug"},
			expected:   []string{"--query.replica-label=replica", "--query.replica-label=instance", "--log.level=debug"},
		},
		{
			name:       "multiple different repeatable flags",
			existing:   []string{"--query.replica-label=replica", "--endpoint=store1"},
			additional: []string{"--query.replica-label=instance", "--endpoint=store2"},
			expected:   []string{"--query.replica-label=replica", "--query.replica-label=instance", "--endpoint=store1", "--endpoint=store2"},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result := MergeArgs(tt.existing, tt.additional)

			if len(result) != len(tt.expected) {
				t.Errorf("expected %d args, got %d: %v", len(tt.expected), len(result), result)
				return
			}

			for i, expected := range tt.expected {
				if i >= len(result) || result[i] != expected {
					t.Errorf("at index %d: expected %q, got %q", i, expected, result[i])
				}
			}
		})
	}
}

func TestExtractFlag(t *testing.T) {
	tests := []struct {
		name     string
		arg      string
		expected string
	}{
		{
			name:     "flag with value",
			arg:      "--log.level=info",
			expected: "--log.level",
		},
		{
			name:     "flag without value",
			arg:      "--help",
			expected: "--help",
		},
		{
			name:     "single dash flag",
			arg:      "-v",
			expected: "",
		},
		{
			name:     "non-flag argument",
			arg:      "query",
			expected: "",
		},
		{
			name:     "empty string",
			arg:      "",
			expected: "",
		},
		{
			name:     "flag with multiple equals",
			arg:      "--config=key=value",
			expected: "--config",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result := extractFlag(tt.arg)
			if result != tt.expected {
				t.Errorf("expected %q, got %q", tt.expected, result)
			}
		})
	}
}

func TestIsRepeatableFlag(t *testing.T) {
	tests := []struct {
		name     string
		flag     string
		expected bool
	}{
		{
			name:     "repeatable query replica label",
			flag:     "--query.replica-label",
			expected: true,
		},
		{
			name:     "repeatable endpoint",
			flag:     "--endpoint",
			expected: true,
		},
		{
			name:     "repeatable label flag",
			flag:     "--label",
			expected: true,
		},
		{
			name:     "non-repeatable log level",
			flag:     "--log.level",
			expected: false,
		},
		{
			name:     "non-repeatable query timeout",
			flag:     "--query.timeout",
			expected: false,
		},
		{
			name:     "empty flag",
			flag:     "",
			expected: false,
		},
		{
			name:     "invalid flag format",
			flag:     "not-a-flag",
			expected: false,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result := isRepeatableFlag(tt.flag)
			if result != tt.expected {
				t.Errorf("expected %v, got %v", tt.expected, result)
			}
		})
	}
}
