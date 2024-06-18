package query

import (
	"fmt"

	appsv1 "k8s.io/api/apps/v1"
	corev1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/util/intstr"
	"k8s.io/utils/ptr"
	"sigs.k8s.io/controller-runtime/pkg/client"

	"github.com/thanos-community/thanos-operator/internal/pkg/manifests"
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

// QuerierOptions for Thanos Querier
type QuerierOptions struct {
	manifests.Options
	ReplicaLabels []string
	Timeout       string
	LookbackDelta string
	MaxConcurrent int

	Endpoints []Endpoint
}

type EndpointType string

const (
	Regular     EndpointType = "regular"
	Strict      EndpointType = "strict"
	Group       EndpointType = "group"
	GroupStrict EndpointType = "groupStrict"
)

// Endpoint represents a single StoreAPI DNS formatted address.
// TODO(saswatamcode): Add validation.
type Endpoint struct {
	ServiceName string
	Namespace   string
	Type        EndpointType
}

func BuildQuerier(opts QuerierOptions) []client.Object {
	var objs []client.Object
	objs = append(objs, manifests.BuildServiceAccount(opts.Options))
	objs = append(objs, NewQuerierDeployment(opts))
	objs = append(objs, NewQuerierService(opts))
	return objs
}

func NewQuerierDeployment(opts QuerierOptions) *appsv1.Deployment {
	defaultLabels := labelsForQuerier(opts)
	aggregatedLabels := manifests.MergeLabels(opts.Labels, defaultLabels)
	podAffinity := corev1.Affinity{
		PodAntiAffinity: &corev1.PodAntiAffinity{
			PreferredDuringSchedulingIgnoredDuringExecution: []corev1.WeightedPodAffinityTerm{{
				Weight: 100,
				PodAffinityTerm: corev1.PodAffinityTerm{
					LabelSelector: &metav1.LabelSelector{
						MatchExpressions: []metav1.LabelSelectorRequirement{{
							Key:      manifests.NameLabel,
							Operator: metav1.LabelSelectorOpIn,
							Values:   []string{opts.Name},
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
			RunAsUser:                ptr.To(int64(10001)),
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
		Args:                     querierArgs(opts),
	}

	deployment := appsv1.Deployment{
		TypeMeta: metav1.TypeMeta{
			Kind:       "Deployment",
			APIVersion: appsv1.SchemeGroupVersion.String(),
		},
		ObjectMeta: metav1.ObjectMeta{
			Name:      opts.Name,
			Namespace: opts.Namespace,
			Labels:    aggregatedLabels,
		},
		Spec: appsv1.DeploymentSpec{
			Replicas: &opts.Replicas,
			Selector: &metav1.LabelSelector{
				MatchLabels: defaultLabels,
			},
			Template: corev1.PodTemplateSpec{
				ObjectMeta: metav1.ObjectMeta{
					Labels: aggregatedLabels,
				},
				Spec: corev1.PodSpec{
					Affinity:           &podAffinity,
					Containers:         []corev1.Container{queryContainer},
					ServiceAccountName: opts.Name,
				},
			},
		},
	}

	return &deployment
}

func NewQuerierService(opts QuerierOptions) *corev1.Service {
	defaultLabels := labelsForQuerier(opts)
	aggregatedLabels := manifests.MergeLabels(opts.Labels, defaultLabels)
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

	return &corev1.Service{
		ObjectMeta: metav1.ObjectMeta{
			Name:      opts.Name,
			Namespace: opts.Namespace,
			Labels:    aggregatedLabels,
		},
		Spec: corev1.ServiceSpec{
			Selector:  defaultLabels,
			Ports:     servicePorts,
			ClusterIP: corev1.ClusterIPNone,
		},
	}
}

func querierArgs(opts QuerierOptions) []string {
	args := []string{
		"query",
		fmt.Sprintf("--log.level=%s", opts.LogLevel),
		fmt.Sprintf("--log.format=%s", opts.LogFormat),
		fmt.Sprintf("--grpc-address=0.0.0.0:%d", GRPCPort),
		fmt.Sprintf("--http-address=0.0.0.0:%d", HTTPPort),
		"--web.prefix-header=X-Forwarded-Prefix",
		fmt.Sprintf("--query.timeout=%s", opts.Timeout),
		fmt.Sprintf("--query.lookback-delta=%s", opts.LookbackDelta),
		"--query.auto-downsampling",
		"--grpc.proxy-strategy=eager",
		"--query.promql-engine=thanos",
		fmt.Sprintf("--query.max-concurrent=%d", opts.MaxConcurrent),
	}

	for _, label := range opts.ReplicaLabels {
		args = append(args, fmt.Sprintf("--query.replica-label=%s", label))
	}

	for _, ep := range opts.Endpoints {
		switch ep.Type {
		case Regular:
			// TODO(saswatamcode): For regular probably use SD file.
			args = append(args, fmt.Sprintf("--endpoint=dnssrv+_grpc._tcp.%s.%s.svc.cluster.local", ep.ServiceName, ep.Namespace))
		case Strict:
			args = append(args, fmt.Sprintf("--endpoint-strict=dnssrv+_grpc._tcp.%s.%s.svc.cluster.local", ep.ServiceName, ep.Namespace))
		case Group:
			args = append(args, fmt.Sprintf("--endpoint-group=dnssrv+_grpc._tcp.%s.%s.svc.cluster.local", ep.ServiceName, ep.Namespace))
		case GroupStrict:
			args = append(args, fmt.Sprintf("--endpoint-group-strict=dnssrv+_grpc._tcp.%s.%s.svc.cluster.local", ep.ServiceName, ep.Namespace))
		}
	}

	return args
}

func labelsForQuerier(opts QuerierOptions) map[string]string {
	return map[string]string{
		manifests.NameLabel:      Name,
		manifests.ComponentLabel: Name,
		manifests.InstanceLabel:  opts.Name,
		manifests.PartOfLabel:    manifests.DefaultPartOfLabel,
		manifests.ManagedByLabel: manifests.DefaultManagedByLabel,
	}
}
