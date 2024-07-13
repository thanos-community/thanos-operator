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
