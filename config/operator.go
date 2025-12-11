package config

import (
	"fmt"

	monitoringv1 "github.com/prometheus-operator/prometheus-operator/pkg/apis/monitoring/v1"

	appsv1 "k8s.io/api/apps/v1"
	corev1 "k8s.io/api/core/v1"
	"k8s.io/apimachinery/pkg/api/resource"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/util/intstr"
	"k8s.io/utils/ptr"
)

const (
	ManagerName            = "controller-manager"
	ServiceAccountName     = "controller-manager"
	ManagerRoleName        = "manager-role"
	LeaderElectionRoleName = "leader-election-role"
	ProxyRoleName          = "proxy-role"
	MetricsReaderName      = "metrics-reader"
)

// CRD represents a Custom Resource Definition
type CRD struct {
	Kind        string // e.g., "ThanosQuery"
	Plural      string // e.g., "thanosqueries"
	ShortName   string // e.g., "thanosquery"
	Description string // e.g., "thanosqueries"
}

var CRDList = []CRD{
	{
		Kind:        "ThanosQuery",
		Plural:      "thanosqueries",
		ShortName:   "thanosquery",
		Description: "thanosqueries",
	},
	{
		Kind:        "ThanosStore",
		Plural:      "thanosstores",
		ShortName:   "thanosstore",
		Description: "thanosstores",
	},
	{
		Kind:        "ThanosReceive",
		Plural:      "thanosreceives",
		ShortName:   "thanosreceive",
		Description: "thanosreceives",
	},
	{
		Kind:        "ThanosCompact",
		Plural:      "thanoscompacts",
		ShortName:   "thanoscompact",
		Description: "thanoscompacts",
	},
	{
		Kind:        "ThanosRuler",
		Plural:      "thanosrulers",
		ShortName:   "thanosruler",
		Description: "thanosrulers",
	},
}

var (
	DefaultCreatedBy = "thanos-operator"
	DefaultPartOf    = "thanos-operator"
	DefaultManagedBy = "kustomize"

	DefaultNamePrefix     = ""
	RecommendedNamePrefix = "thanos-operator-"

	DefaultNamespace     = "system"
	RecommendedNamespace = "thanos-operator-system"

	DefaultManagerImage     = "controller:latest"
	RecommendedManagerImage = "quay.io/thanos/thanos-operator:main"

	DefaultAuthProxyImage = "gcr.io/kubebuilder/kube-rbac-proxy:v0.16.0"
)

// SetGlobalCommonLabels sets the global common labels for all resources.
func SetGlobalCommonLabels(createdByOverride, partOfOverride, managedByOverride string) {
	DefaultCreatedBy = createdByOverride
	DefaultPartOf = partOfOverride
	DefaultManagedBy = managedByOverride
}

// SetGlobalPrefix sets the global name prefix for all resources.
func SetGlobalPrefix(prefix string) {
	DefaultNamePrefix = prefix
}

// SetGlobalNamespace sets the global namespace for all resources.
func SetGlobalNamespace(namespace string) {
	DefaultNamespace = namespace
}

// SetGlobalManagerImage sets the global manager image for all resources.
func SetGlobalManagerImage(image string) {
	DefaultManagerImage = image
}

// SetGlobalAuthProxyImage sets the global auth proxy image for all resources.
func SetGlobalAuthProxyImage(image string) {
	DefaultAuthProxyImage = image
}

// commonLabels returns the standard labels used across all resources
func commonLabels(component, instance, kind string) map[string]string {
	return map[string]string{
		"app.kubernetes.io/name":       kind,
		"app.kubernetes.io/instance":   instance,
		"app.kubernetes.io/component":  component,
		"app.kubernetes.io/created-by": DefaultCreatedBy,
		"app.kubernetes.io/part-of":    DefaultPartOf,
		"app.kubernetes.io/managed-by": DefaultManagedBy,
	}
}

// commonControllerManagerLabels returns the standard labels used across all controller manager resources.
func commonControllerManagerLabels(component, instance, kind string) map[string]string {
	return map[string]string{
		"control-plane":                "controller-manager",
		"app.kubernetes.io/name":       kind,
		"app.kubernetes.io/instance":   instance,
		"app.kubernetes.io/component":  component,
		"app.kubernetes.io/created-by": DefaultCreatedBy,
		"app.kubernetes.io/part-of":    DefaultPartOf,
		"app.kubernetes.io/managed-by": DefaultManagedBy,
	}
}

// prefixName prefixes the name with the default name prefix.
func prefixName(name string) string {
	return fmt.Sprintf("%s%s", DefaultNamePrefix, name)
}

// ManagerServiceAccount creates the controller manager service account.
func ManagerServiceAccount() *corev1.ServiceAccount {
	return &corev1.ServiceAccount{
		TypeMeta: metav1.TypeMeta{
			APIVersion: "v1",
			Kind:       "ServiceAccount",
		},
		ObjectMeta: metav1.ObjectMeta{
			Name:      prefixName(ServiceAccountName),
			Namespace: DefaultNamespace,
			Labels:    commonLabels("rbac", "controller-manager-sa", "serviceaccount"),
		},
	}
}

// ControllerManagerServiceMonitor creates the ServiceMonitor for the controller manager metrics.
func ControllerManagerServiceMonitor() *monitoringv1.ServiceMonitor {
	return &monitoringv1.ServiceMonitor{
		TypeMeta: metav1.TypeMeta{
			APIVersion: "monitoring.coreos.com/v1",
			Kind:       "ServiceMonitor",
		},
		ObjectMeta: metav1.ObjectMeta{
			Name:      prefixName("controller-manager-metrics-monitor"),
			Namespace: DefaultNamespace,
			Labels:    commonControllerManagerLabels("metrics", "controller-manager-metrics-monitor", "servicemonitor"),
		},
		Spec: monitoringv1.ServiceMonitorSpec{
			Endpoints: []monitoringv1.Endpoint{
				{
					Path:            "/metrics",
					Port:            "https",
					Scheme:          ptr.To(monitoringv1.SchemeHTTPS),
					BearerTokenFile: "/var/run/secrets/kubernetes.io/serviceaccount/token",
					TLSConfig: &monitoringv1.TLSConfig{
						SafeTLSConfig: monitoringv1.SafeTLSConfig{
							InsecureSkipVerify: ptr.To(true),
						},
					},
				},
			},
			Selector: metav1.LabelSelector{
				MatchLabels: map[string]string{
					"control-plane": "controller-manager",
				},
			},
		},
	}
}

// ControllerManagerNamespace creates the Namespace for the controller manager.
func ControllerManagerNamespace() *corev1.Namespace {
	return &corev1.Namespace{
		TypeMeta: metav1.TypeMeta{
			APIVersion: "v1",
			Kind:       "Namespace",
		},
		ObjectMeta: metav1.ObjectMeta{
			Name:   prefixName(DefaultNamespace),
			Labels: commonControllerManagerLabels("manager", DefaultNamespace, "namespace"),
		},
	}
}

// ControllerManagerDeployment creates the Deployment for the controller manager.
func ControllerManagerDeployment(enableAuthProxy bool) *appsv1.Deployment {
	replicas := int32(1)
	deployment := &appsv1.Deployment{
		TypeMeta: metav1.TypeMeta{
			APIVersion: "apps/v1",
			Kind:       "Deployment",
		},
		ObjectMeta: metav1.ObjectMeta{
			Name:      prefixName(ManagerName),
			Namespace: DefaultNamespace,
			Labels:    commonControllerManagerLabels("manager", ManagerName, "deployment"),
		},
		Spec: appsv1.DeploymentSpec{
			Replicas: &replicas,
			Selector: &metav1.LabelSelector{
				MatchLabels: map[string]string{
					"control-plane": "controller-manager",
				},
			},
			Template: corev1.PodTemplateSpec{
				ObjectMeta: metav1.ObjectMeta{
					Labels: map[string]string{
						"control-plane": "controller-manager",
					},
					Annotations: map[string]string{
						"kubectl.kubernetes.io/default-container": "manager",
					},
				},
				Spec: corev1.PodSpec{
					SecurityContext: &corev1.PodSecurityContext{
						RunAsNonRoot: ptr.To(true),
					},
					ServiceAccountName:            prefixName(ServiceAccountName),
					TerminationGracePeriodSeconds: ptr.To(int64(10)),
					Containers: []corev1.Container{
						{
							Name:    "manager",
							Image:   DefaultManagerImage,
							Command: []string{"/manager"},
							Args:    []string{"--leader-elect"},
							SecurityContext: &corev1.SecurityContext{
								AllowPrivilegeEscalation: ptr.To(false),
								Capabilities: &corev1.Capabilities{
									Drop: []corev1.Capability{"ALL"},
								},
							},
							LivenessProbe: &corev1.Probe{
								ProbeHandler: corev1.ProbeHandler{
									HTTPGet: &corev1.HTTPGetAction{
										Path: "/healthz",
										Port: intstr.FromInt(8081),
									},
								},
								InitialDelaySeconds: 15,
								PeriodSeconds:       20,
							},
							ReadinessProbe: &corev1.Probe{
								ProbeHandler: corev1.ProbeHandler{
									HTTPGet: &corev1.HTTPGetAction{
										Path: "/readyz",
										Port: intstr.FromInt(8081),
									},
								},
								InitialDelaySeconds: 5,
								PeriodSeconds:       10,
							},
							Resources: corev1.ResourceRequirements{
								Limits: corev1.ResourceList{
									corev1.ResourceCPU:    resource.MustParse("500m"),
									corev1.ResourceMemory: resource.MustParse("128Mi"),
								},
								Requests: corev1.ResourceList{
									corev1.ResourceCPU:    resource.MustParse("10m"),
									corev1.ResourceMemory: resource.MustParse("64Mi"),
								},
							},
						},
					},
				},
			},
		},
	}

	if enableAuthProxy {
		deployment.Spec.Template.Spec.Containers = append(deployment.Spec.Template.Spec.Containers, corev1.Container{
			Name:  "kube-rbac-proxy",
			Image: DefaultAuthProxyImage,
			Args:  []string{"--secure-listen-address=0.0.0.0:8443", "--upstream=http://127.0.0.1:8080/", "--logtostderr=true", "--v=0"},
			SecurityContext: &corev1.SecurityContext{
				AllowPrivilegeEscalation: ptr.To(false),
				Capabilities: &corev1.Capabilities{
					Drop: []corev1.Capability{"ALL"},
				},
			},
			Ports: []corev1.ContainerPort{
				{
					ContainerPort: 8443,
					Name:          "https",
					Protocol:      corev1.ProtocolTCP,
				},
			},
			Resources: corev1.ResourceRequirements{
				Limits: corev1.ResourceList{
					corev1.ResourceCPU:    resource.MustParse("500m"),
					corev1.ResourceMemory: resource.MustParse("128Mi"),
				},
				Requests: corev1.ResourceList{
					corev1.ResourceCPU:    resource.MustParse("5m"),
					corev1.ResourceMemory: resource.MustParse("64Mi"),
				},
			},
		})

		for i, container := range deployment.Spec.Template.Spec.Containers {
			if container.Name == "manager" {
				deployment.Spec.Template.Spec.Containers[i].Args = append(deployment.Spec.Template.Spec.Containers[i].Args, "--metrics-bind-address=:8080")
				deployment.Spec.Template.Spec.Containers[i].Args = append(deployment.Spec.Template.Spec.Containers[i].Args, "--health-probe-bind-address=:8081")
				deployment.Spec.Template.Spec.Containers[i].Args = append(deployment.Spec.Template.Spec.Containers[i].Args, "--log.format=logfmt")
				deployment.Spec.Template.Spec.Containers[i].Args = append(deployment.Spec.Template.Spec.Containers[i].Args, "--log.level=debug")
			}
		}
	}

	return deployment
}
