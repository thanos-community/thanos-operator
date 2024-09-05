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
		input    interface{}
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
