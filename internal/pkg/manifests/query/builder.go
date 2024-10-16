package query

import (
	"fmt"

	"github.com/thanos-community/thanos-operator/internal/pkg/manifests"

	appsv1 "k8s.io/api/apps/v1"
	corev1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/util/intstr"
	"k8s.io/utils/ptr"

	"sigs.k8s.io/controller-runtime/pkg/client"
)

const (
	// Name is the name of the Thanos Query component.
	Name = "thanos-query"

	// ComponentName is the name of the Thanos Query component.
	ComponentName = "query-layer"

	GRPCPort     = 10901
	GRPCPortName = "grpc"

	HTTPPort     = 9090
	HTTPPortName = "http"
)

// Options for Thanos Query
type Options struct {
	manifests.Options
	ReplicaLabels []string
	Timeout       string
	LookbackDelta string
	MaxConcurrent int

	Endpoints []Endpoint
}

// Endpoint represents a single StoreAPI DNS formatted address.
// TODO(saswatamcode): Add validation.
type Endpoint struct {
	ServiceName string
	Namespace   string
	Type        manifests.EndpointType
	Port        int32
}

func (opts Options) Build() []client.Object {
	var objs []client.Object
	selectorLabels := opts.GetSelectorLabels()
	objectMetaLabels := GetLabels(opts)
	name := opts.GetGeneratedResourceName()

	objs = append(objs, manifests.BuildServiceAccount(opts.GetGeneratedResourceName(), opts.Namespace, selectorLabels, opts.Annotations))
	objs = append(objs, newQueryDeployment(opts, selectorLabels, objectMetaLabels))
	objs = append(objs, newQueryService(opts, selectorLabels, objectMetaLabels))

	if opts.PodDisruptionConfig != nil {
		objs = append(objs, manifests.NewPodDisruptionBudget(name, opts.Namespace, selectorLabels, objectMetaLabels, opts.Annotations, *opts.PodDisruptionConfig))
	}

	if opts.ServiceMonitorConfig.Enabled {
		objs = append(objs, manifests.BuildServiceMonitor(name, opts.Namespace, objectMetaLabels, selectorLabels, serviceMonitorOpts(opts.ServiceMonitorConfig)))
	}
	return objs
}

func (opts Options) GetGeneratedResourceName() string {
	name := fmt.Sprintf("%s-%s", Name, opts.getOwner())
	return manifests.ValidateAndSanitizeResourceName(name)
}

func (opts Options) getOwner() string {
	return opts.Owner
}

func NewQueryDeployment(opts Options) *appsv1.Deployment {
	selectorLabels := opts.GetSelectorLabels()
	objectMetaLabels := GetLabels(opts)
	return newQueryDeployment(opts, selectorLabels, objectMetaLabels)
}

func newQueryDeployment(opts Options, selectorLabels, objectMetaLabels map[string]string) *appsv1.Deployment {
	name := opts.GetGeneratedResourceName()
	podAffinity := corev1.Affinity{
		PodAntiAffinity: &corev1.PodAntiAffinity{
			PreferredDuringSchedulingIgnoredDuringExecution: []corev1.WeightedPodAffinityTerm{{
				Weight: 100,
				PodAffinityTerm: corev1.PodAffinityTerm{
					LabelSelector: &metav1.LabelSelector{
						MatchExpressions: []metav1.LabelSelectorRequirement{{
							Key:      manifests.NameLabel,
							Operator: metav1.LabelSelectorOpIn,
							Values:   []string{name},
						}},
					},
					Namespaces:  []string{opts.Namespace},
					TopologyKey: "kubernetes.io/hostname",
				},
			}},
		},
	}

	queryContainer := corev1.Container{
		Image:           opts.GetContainerImage(),
		Name:            Name,
		ImagePullPolicy: corev1.PullIfNotPresent,
		// Ensure restrictive context for the container
		// More info: https://kubernetes.io/docs/concepts/security/pod-security-standards/#restricted
		SecurityContext: &corev1.SecurityContext{
			RunAsNonRoot:             ptr.To(true),
			AllowPrivilegeEscalation: ptr.To(false),
			Capabilities: &corev1.Capabilities{
				Drop: []corev1.Capability{
					"ALL",
				},
			},
		},
		ReadinessProbe: &corev1.Probe{
			ProbeHandler: corev1.ProbeHandler{
				HTTPGet: &corev1.HTTPGetAction{
					Path:   "/-/ready",
					Port:   intstr.FromInt32(HTTPPort),
					Scheme: corev1.URISchemeHTTP,
				},
			},
			InitialDelaySeconds: 30,
			TimeoutSeconds:      1,
			PeriodSeconds:       5,
			SuccessThreshold:    1,
			FailureThreshold:    20,
		},
		LivenessProbe: &corev1.Probe{
			ProbeHandler: corev1.ProbeHandler{
				HTTPGet: &corev1.HTTPGetAction{
					Path: "/-/healthy",
					Port: intstr.FromInt32(HTTPPort),
				},
			},
			InitialDelaySeconds: 30,
			TimeoutSeconds:      1,
			PeriodSeconds:       30,
			SuccessThreshold:    1,
			FailureThreshold:    4,
		},
		Ports: []corev1.ContainerPort{
			{
				ContainerPort: GRPCPort,
				Name:          GRPCPortName,
			},
			{
				ContainerPort: HTTPPort,
				Name:          HTTPPortName,
			},
		},
		TerminationMessagePolicy: corev1.TerminationMessageFallbackToLogsOnError,
		TerminationMessagePath:   corev1.TerminationMessagePathDefault,
		Args:                     queryArgs(opts),
	}

	deployment := &appsv1.Deployment{
		TypeMeta: metav1.TypeMeta{
			Kind:       "Deployment",
			APIVersion: appsv1.SchemeGroupVersion.String(),
		},
		ObjectMeta: metav1.ObjectMeta{
			Name:        name,
			Namespace:   opts.Namespace,
			Labels:      objectMetaLabels,
			Annotations: opts.Annotations,
		},
		Spec: appsv1.DeploymentSpec{
			Replicas: &opts.Replicas,
			Selector: &metav1.LabelSelector{
				MatchLabels: selectorLabels,
			},
			Template: corev1.PodTemplateSpec{
				ObjectMeta: metav1.ObjectMeta{
					Labels: objectMetaLabels,
				},
				Spec: corev1.PodSpec{
					Affinity:           &podAffinity,
					SecurityContext:    &corev1.PodSecurityContext{},
					Containers:         []corev1.Container{queryContainer},
					ServiceAccountName: name,
				},
			},
		},
	}

	manifests.AugmentWithOptions(deployment, opts.Options)
	return deployment
}

func NewQueryService(opts Options) *corev1.Service {
	selectorLabels := opts.GetSelectorLabels()
	return newQueryService(opts, selectorLabels, manifests.MergeLabels(opts.Labels, selectorLabels))
}

func newQueryService(opts Options, selectorLabels, objectMetaLabels map[string]string) *corev1.Service {
	servicePorts := []corev1.ServicePort{
		{
			Name:       GRPCPortName,
			Port:       GRPCPort,
			TargetPort: intstr.FromInt32(GRPCPort),
		},
		{
			Name:       HTTPPortName,
			Port:       HTTPPort,
			TargetPort: intstr.FromInt32(HTTPPort),
		},
	}

	if opts.Additional.ServicePorts != nil {
		servicePorts = append(servicePorts, opts.Additional.ServicePorts...)
	}

	return &corev1.Service{
		TypeMeta: metav1.TypeMeta{
			Kind:       "Service",
			APIVersion: corev1.SchemeGroupVersion.String(),
		},
		ObjectMeta: metav1.ObjectMeta{
			Name:        opts.GetGeneratedResourceName(),
			Namespace:   opts.Namespace,
			Labels:      objectMetaLabels,
			Annotations: opts.Annotations,
		},
		Spec: corev1.ServiceSpec{
			Selector:  selectorLabels,
			Ports:     servicePorts,
			ClusterIP: corev1.ClusterIPNone,
		},
	}
}

func queryArgs(opts Options) []string {
	args := []string{"query"}
	args = append(args, opts.ToFlags()...)
	args = append(args,
		fmt.Sprintf("--grpc-address=0.0.0.0:%d", GRPCPort),
		fmt.Sprintf("--http-address=0.0.0.0:%d", HTTPPort),
		"--web.prefix-header=X-Forwarded-Prefix",
		fmt.Sprintf("--query.timeout=%s", opts.Timeout),
		fmt.Sprintf("--query.lookback-delta=%s", opts.LookbackDelta),
		"--query.auto-downsampling",
		"--grpc.proxy-strategy=eager",
		"--query.promql-engine=thanos",
		fmt.Sprintf("--query.max-concurrent=%d", opts.MaxConcurrent),
	)

	for _, label := range opts.ReplicaLabels {
		args = append(args, fmt.Sprintf("--query.replica-label=%s", label))
	}

	for _, ep := range opts.Endpoints {
		switch ep.Type {
		case manifests.RegularLabel:
			// TODO(saswatamcode): For regular probably use SD file.
			args = append(args, fmt.Sprintf("--endpoint=dnssrv+_grpc._tcp.%s.%s.svc.cluster.local", ep.ServiceName, ep.Namespace))
		case manifests.StrictLabel:
			args = append(args, fmt.Sprintf("--endpoint-strict=dnssrv+_grpc._tcp.%s.%s.svc.cluster.local", ep.ServiceName, ep.Namespace))
		case manifests.GroupLabel:
			args = append(args, fmt.Sprintf("--endpoint-group=%s.%s.svc.cluster.local:%d", ep.ServiceName, ep.Namespace, ep.Port))
		case manifests.GroupStrictLabel:
			args = append(args, fmt.Sprintf("--endpoint-group-strict=%s.%s.svc.cluster.local:%d", ep.ServiceName, ep.Namespace, ep.Port))
		default:
			panic("unknown endpoint type")
		}
	}

	// TODO(saswatamcode): Add some validation.
	if opts.Additional.Args != nil {
		args = append(args, opts.Additional.Args...)
	}

	return manifests.PruneEmptyArgs(args)
}

// GetRequiredLabels returns a map of labels that can be used to look up query resources.
// These labels are guaranteed to be present on all resources created by this package.
func GetRequiredLabels() map[string]string {
	return map[string]string{
		manifests.NameLabel:            Name,
		manifests.ComponentLabel:       ComponentName,
		manifests.PartOfLabel:          manifests.DefaultPartOfLabel,
		manifests.ManagedByLabel:       manifests.DefaultManagedByLabel,
		manifests.DefaultQueryAPILabel: manifests.DefaultQueryAPIValue,
	}
}

// GetSelectorLabels returns a map of labels that can be used to look up query resources.
func (opts Options) GetSelectorLabels() map[string]string {
	labels := GetRequiredLabels()
	labels[manifests.InstanceLabel] = manifests.ValidateAndSanitizeNameToValidLabelValue(opts.GetGeneratedResourceName())
	labels[manifests.OwnerLabel] = manifests.ValidateAndSanitizeNameToValidLabelValue(opts.getOwner())
	return labels
}

// GetLabels returns a map of labels that can be used to look up query resources.
func GetLabels(opts Options) map[string]string {
	return manifests.MergeLabels(opts.Labels, opts.GetSelectorLabels())
}

func serviceMonitorOpts(from manifests.ServiceMonitorConfig) manifests.ServiceMonitorOptions {
	return manifests.ServiceMonitorOptions{
		Port:     ptr.To(HTTPPortName),
		Interval: from.Interval,
	}
}
