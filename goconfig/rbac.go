package goconfig

import (
	corev1 "k8s.io/api/core/v1"
	rbacv1 "k8s.io/api/rbac/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/util/intstr"

	thanosv1alpha1 "github.com/thanos-community/thanos-operator/api/v1alpha1"
)

// ManagerClusterRole creates the ClusterRole for the manager with all necessary permissions
func ManagerClusterRole() *rbacv1.ClusterRole {
	resources := []string{}
	for _, crd := range CRDList {
		resources = append(resources, crd.Plural)
	}

	finalizerResources := []string{}
	statusResources := []string{}
	for _, crd := range CRDList {
		finalizerResources = append(finalizerResources, crd.Plural+"/finalizers")
		statusResources = append(statusResources, crd.Plural+"/status")
	}

	return &rbacv1.ClusterRole{
		TypeMeta: metav1.TypeMeta{
			APIVersion: "rbac.authorization.k8s.io/v1",
			Kind:       "ClusterRole",
		},
		ObjectMeta: metav1.ObjectMeta{
			Name: prefixName(ManagerRoleName),
		},
		Rules: []rbacv1.PolicyRule{
			{
				APIGroups: []string{""},
				Resources: []string{"configmaps", "serviceaccounts", "services"},
				Verbs:     []string{"create", "delete", "get", "list", "patch", "update", "watch"},
			},
			{
				APIGroups: []string{"apps"},
				Resources: []string{"deployments", "statefulsets"},
				Verbs:     []string{"create", "delete", "get", "list", "patch", "update", "watch"},
			},
			{
				APIGroups: []string{"discovery.k8s.io"},
				Resources: []string{"endpointslices"},
				Verbs:     []string{"get", "list", "watch"},
			},
			{
				APIGroups: []string{"monitoring.coreos.com"},
				Resources: []string{"prometheusrules"},
				Verbs:     []string{"get", "list", "watch"},
			},
			{
				APIGroups: []string{"monitoring.coreos.com"},
				Resources: []string{"servicemonitors"},
				Verbs:     []string{"create", "delete", "get", "list", "patch", "update", "watch"},
			},
			{
				APIGroups: []string{thanosv1alpha1.GroupVersion.Group},
				Resources: resources,
				Verbs:     []string{"create", "delete", "get", "list", "patch", "update", "watch"},
			},
			{
				APIGroups: []string{thanosv1alpha1.GroupVersion.Group},
				Resources: finalizerResources,
				Verbs:     []string{"update"},
			},
			{
				APIGroups: []string{thanosv1alpha1.GroupVersion.Group},
				Resources: statusResources,
				Verbs:     []string{"get", "patch", "update"},
			},
			{
				APIGroups: []string{"policy"},
				Resources: []string{"poddisruptionbudgets"},
				Verbs:     []string{"create", "delete", "get", "list", "patch", "update", "watch"},
			},
		},
	}
}

// generateManagerClusterRoleBinding creates the ClusterRoleBinding for the manager role
func ManagerClusterRoleBinding() *rbacv1.ClusterRoleBinding {
	return &rbacv1.ClusterRoleBinding{
		TypeMeta: metav1.TypeMeta{
			APIVersion: "rbac.authorization.k8s.io/v1",
			Kind:       "ClusterRoleBinding",
		},
		ObjectMeta: metav1.ObjectMeta{
			Name:   prefixName("manager-rolebinding"),
			Labels: commonLabels("rbac", "manager-rolebinding", "clusterrolebinding"),
		},
		RoleRef: rbacv1.RoleRef{
			APIGroup: "rbac.authorization.k8s.io",
			Kind:     "ClusterRole",
			Name:     prefixName(ManagerRoleName),
		},
		Subjects: []rbacv1.Subject{
			{
				Kind:      "ServiceAccount",
				Name:      prefixName(ServiceAccountName),
				Namespace: DefaultNamespace,
			},
		},
	}
}

// LeaderElectionRole creates the Role for leader election
func LeaderElectionRole() *rbacv1.Role {
	return &rbacv1.Role{
		TypeMeta: metav1.TypeMeta{
			APIVersion: "rbac.authorization.k8s.io/v1",
			Kind:       "Role",
		},
		ObjectMeta: metav1.ObjectMeta{
			Name:   prefixName(LeaderElectionRoleName),
			Labels: commonLabels("rbac", LeaderElectionRoleName, "role"),
		},
		Rules: []rbacv1.PolicyRule{
			{
				APIGroups: []string{""},
				Resources: []string{"configmaps"},
				Verbs:     []string{"get", "list", "watch", "create", "update", "patch", "delete"},
			},
			{
				APIGroups: []string{"coordination.k8s.io"},
				Resources: []string{"leases"},
				Verbs:     []string{"get", "list", "watch", "create", "update", "patch", "delete"},
			},
			{
				APIGroups: []string{""},
				Resources: []string{"events"},
				Verbs:     []string{"create", "patch"},
			},
		},
	}
}

// LeaderElectionRoleBinding creates the RoleBinding for leader election
func LeaderElectionRoleBinding() *rbacv1.RoleBinding {
	return &rbacv1.RoleBinding{
		TypeMeta: metav1.TypeMeta{
			APIVersion: "rbac.authorization.k8s.io/v1",
			Kind:       "RoleBinding",
		},
		ObjectMeta: metav1.ObjectMeta{
			Name:   prefixName("leader-election-rolebinding"),
			Labels: commonLabels("rbac", "leader-election-rolebinding", "rolebinding"),
		},
		RoleRef: rbacv1.RoleRef{
			APIGroup: "rbac.authorization.k8s.io",
			Kind:     "Role",
			Name:     prefixName(LeaderElectionRoleName),
		},
		Subjects: []rbacv1.Subject{
			{
				Kind:      "ServiceAccount",
				Name:      prefixName(ServiceAccountName),
				Namespace: DefaultNamespace,
			},
		},
	}
}

// AuthProxyClusterRole creates the ClusterRole for the auth proxy
func AuthProxyClusterRole() *rbacv1.ClusterRole {
	return &rbacv1.ClusterRole{
		TypeMeta: metav1.TypeMeta{
			APIVersion: "rbac.authorization.k8s.io/v1",
			Kind:       "ClusterRole",
		},
		ObjectMeta: metav1.ObjectMeta{
			Name:   prefixName(ProxyRoleName),
			Labels: commonLabels("kube-rbac-proxy", ProxyRoleName, "clusterrole"),
		},
		Rules: []rbacv1.PolicyRule{
			{
				APIGroups: []string{"authentication.k8s.io"},
				Resources: []string{"tokenreviews"},
				Verbs:     []string{"create"},
			},
			{
				APIGroups: []string{"authorization.k8s.io"},
				Resources: []string{"subjectaccessreviews"},
				Verbs:     []string{"create"},
			},
		},
	}
}

// AuthProxyClusterRoleBinding creates the ClusterRoleBinding for the auth proxy
func AuthProxyClusterRoleBinding() *rbacv1.ClusterRoleBinding {
	return &rbacv1.ClusterRoleBinding{
		TypeMeta: metav1.TypeMeta{
			APIVersion: "rbac.authorization.k8s.io/v1",
			Kind:       "ClusterRoleBinding",
		},
		ObjectMeta: metav1.ObjectMeta{
			Name:   prefixName("proxy-rolebinding"),
			Labels: commonLabels("kube-rbac-proxy", "proxy-rolebinding", "clusterrolebinding"),
		},
		RoleRef: rbacv1.RoleRef{
			APIGroup: "rbac.authorization.k8s.io",
			Kind:     "ClusterRole",
			Name:     prefixName(ProxyRoleName),
		},
		Subjects: []rbacv1.Subject{
			{
				Kind:      "ServiceAccount",
				Name:      prefixName(ServiceAccountName),
				Namespace: DefaultNamespace,
			},
		},
	}
}

// AuthProxyClientClusterRole creates the ClusterRole for metrics reading.
func AuthProxyClientClusterRole() *rbacv1.ClusterRole {
	return &rbacv1.ClusterRole{
		TypeMeta: metav1.TypeMeta{
			APIVersion: "rbac.authorization.k8s.io/v1",
			Kind:       "ClusterRole",
		},
		ObjectMeta: metav1.ObjectMeta{
			Name:   prefixName(MetricsReaderName),
			Labels: commonLabels("kube-rbac-proxy", MetricsReaderName, "clusterrole"),
		},
		Rules: []rbacv1.PolicyRule{
			{
				NonResourceURLs: []string{"/metrics"},
				Verbs:           []string{"get"},
			},
		},
	}
}

// AuthProxyService creates the Service for the auth proxy client for metrics.
func AuthProxyService() *corev1.Service {
	return &corev1.Service{
		TypeMeta: metav1.TypeMeta{
			APIVersion: "v1",
			Kind:       "Service",
		},
		ObjectMeta: metav1.ObjectMeta{
			Name:      prefixName("controller-manager-metrics-service"),
			Namespace: DefaultNamespace,
			Labels: map[string]string{
				"control-plane":                "controller-manager",
				"app.kubernetes.io/name":       "service",
				"app.kubernetes.io/instance":   "controller-manager-metrics-service",
				"app.kubernetes.io/component":  "kube-rbac-proxy",
				"app.kubernetes.io/created-by": DefaultCreatedBy,
				"app.kubernetes.io/part-of":    DefaultPartOf,
				"app.kubernetes.io/managed-by": DefaultManagedBy,
			},
		},
		Spec: corev1.ServiceSpec{
			Ports: []corev1.ServicePort{
				{
					Name:       "https",
					Port:       8443,
					Protocol:   corev1.ProtocolTCP,
					TargetPort: intstr.FromString("https"),
				},
			},
			Selector: map[string]string{
				"control-plane": "controller-manager",
			},
		},
	}
}

// CRDEditorClusterRole creates an editor ClusterRole for a CRD
func CRDEditorClusterRole(crd CRD) *rbacv1.ClusterRole {
	return &rbacv1.ClusterRole{
		TypeMeta: metav1.TypeMeta{
			APIVersion: "rbac.authorization.k8s.io/v1",
			Kind:       "ClusterRole",
		},
		ObjectMeta: metav1.ObjectMeta{
			Name:   prefixName(crd.ShortName + "-editor-role"),
			Labels: commonLabels("rbac", crd.ShortName+"-editor-role", "clusterrole"),
		},
		Rules: []rbacv1.PolicyRule{
			{
				APIGroups: []string{thanosv1alpha1.GroupVersion.Group},
				Resources: []string{crd.Plural},
				Verbs:     []string{"create", "delete", "get", "list", "patch", "update", "watch"},
			},
			{
				APIGroups: []string{thanosv1alpha1.GroupVersion.Group},
				Resources: []string{crd.Plural + "/status"},
				Verbs:     []string{"get"},
			},
		},
	}
}

// CRDViewerClusterRole creates a viewer ClusterRole for a CRD
func CRDViewerClusterRole(crd CRD) *rbacv1.ClusterRole {
	return &rbacv1.ClusterRole{
		TypeMeta: metav1.TypeMeta{
			APIVersion: "rbac.authorization.k8s.io/v1",
			Kind:       "ClusterRole",
		},
		ObjectMeta: metav1.ObjectMeta{
			Name:   prefixName(crd.ShortName + "-viewer-role"),
			Labels: commonLabels("rbac", crd.ShortName+"-viewer-role", "clusterrole"),
		},
		Rules: []rbacv1.PolicyRule{
			{
				APIGroups: []string{thanosv1alpha1.GroupVersion.Group},
				Resources: []string{crd.Plural},
				Verbs:     []string{"get", "list", "watch"},
			},
			{
				APIGroups: []string{thanosv1alpha1.GroupVersion.Group},
				Resources: []string{crd.Plural + "/status"},
				Verbs:     []string{"get"},
			},
		},
	}
}
