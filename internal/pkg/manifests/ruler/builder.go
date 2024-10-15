package ruler

import (
	"fmt"

	"github.com/thanos-community/thanos-operator/internal/pkg/manifests"

	appsv1 "k8s.io/api/apps/v1"
	corev1 "k8s.io/api/core/v1"
	"k8s.io/apimachinery/pkg/api/resource"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/util/intstr"
	"k8s.io/utils/ptr"

	"sigs.k8s.io/controller-runtime/pkg/client"
)

const (
	// Name is the name of the Thanos Ruler component.
	Name = "thanos-ruler"

	// ComponentName is the name of the Thanos Ruler component.
	ComponentName = "rule-evaluation-engine"

	GRPCPort     = 10901
	GRPCPortName = "grpc"

	HTTPPort     = 9090
	HTTPPortName = "http"
)

// Options for Thanos Ruler
type Options struct {
	manifests.Options
	Endpoints          []Endpoint
	RuleFiles          []corev1.ConfigMapKeySelector
	ObjStoreSecret     corev1.SecretKeySelector
	Retention          manifests.Duration
	AlertmanagerURL    string
	ExternalLabels     map[string]string
	AlertLabelDrop     []string
	StorageSize        resource.Quantity
	EvaluationInterval manifests.Duration
}

// Endpoint represents a single QueryAPI DNS formatted address.
// TODO(saswatamcode): Add validation.
type Endpoint struct {
	ServiceName string
	Namespace   string
	Port        int32
}

func (opts Options) Build() []client.Object {
	var objs []client.Object
	selectorLabels := opts.GetSelectorLabels()
	objectMetaLabels := GetLabels(opts)
	name := opts.GetGeneratedResourceName()

	objs = append(objs, manifests.BuildServiceAccount(opts.GetGeneratedResourceName(), opts.Namespace, selectorLabels, opts.Annotations))
	objs = append(objs, newRulerStatefulSet(opts, selectorLabels, objectMetaLabels))
	objs = append(objs, newRulerService(opts, selectorLabels, objectMetaLabels))

	if opts.PodDisruptionConfig != nil {
		objs = append(objs, manifests.NewPodDisruptionBudget(name, opts.Namespace, selectorLabels, objectMetaLabels, opts.Annotations, *opts.PodDisruptionConfig))
	}

	if opts.ServiceMonitorConfig.Enabled {
		smLabels := manifests.MergeLabels(opts.ServiceMonitorConfig.Labels, objectMetaLabels)
		objs = append(objs, manifests.BuildServiceMonitor(name, opts.Namespace, selectorLabels, smLabels, serviceMonitorOpts(opts.ServiceMonitorConfig)))
	}
	return objs
}

func (opts Options) GetGeneratedResourceName() string {
	name := fmt.Sprintf("%s-%s", Name, opts.Owner)
	return manifests.ValidateAndSanitizeResourceName(name)
}

const (
	rulerObjectStoreEnvVarName = "OBJSTORE_CONFIG"

	dataVolumeName      = "data"
	dataVolumeMountPath = "var/thanos/rule"
)

func NewRulerStatefulSet(opts Options) *appsv1.StatefulSet {
	selectorLabels := opts.GetSelectorLabels()
	objectMetaLabels := GetLabels(opts)
	return newRulerStatefulSet(opts, selectorLabels, objectMetaLabels)
}

func newRulerStatefulSet(opts Options, selectorLabels, objectMetaLabels map[string]string) *appsv1.StatefulSet {
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

	volumeMounts := []corev1.VolumeMount{
		{
			Name:      dataVolumeName,
			MountPath: dataVolumeMountPath,
		},
	}

	for _, ruleFile := range opts.RuleFiles {
		volumeMounts = append(volumeMounts, corev1.VolumeMount{
			Name:      ruleFile.Name,
			MountPath: fmt.Sprintf("/etc/thanos/rules/%s", ruleFile.Key),
			SubPath:   ruleFile.Key,
		})
	}

	rulerContainer := corev1.Container{
		Image:           opts.GetContainerImage(),
		Name:            Name,
		ImagePullPolicy: corev1.PullIfNotPresent,
		// Ensure restrictive context for the container
		// More info: https://kubernetes.io/docs/concepts/security/pod-security-standards/#restricted
		SecurityContext: &corev1.SecurityContext{
			AllowPrivilegeEscalation: ptr.To(false),
			RunAsNonRoot:             ptr.To(true),
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
		Env: []corev1.EnvVar{
			{
				Name: "NAME",
				ValueFrom: &corev1.EnvVarSource{
					FieldRef: &corev1.ObjectFieldSelector{
						FieldPath: "metadata.name",
					},
				},
			},
			{
				Name: rulerObjectStoreEnvVarName,
				ValueFrom: &corev1.EnvVarSource{
					SecretKeyRef: &corev1.SecretKeySelector{
						LocalObjectReference: corev1.LocalObjectReference{
							Name: opts.ObjStoreSecret.Name,
						},
						Key:      opts.ObjStoreSecret.Key,
						Optional: ptr.To(false),
					},
				},
			},
		},
		VolumeMounts: volumeMounts,
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
		Args:                     rulerArgs(opts),
	}

	vc := []corev1.PersistentVolumeClaim{{
		ObjectMeta: metav1.ObjectMeta{
			Name:      dataVolumeName,
			Namespace: opts.Namespace,
			Labels:    objectMetaLabels,
		},
		Spec: corev1.PersistentVolumeClaimSpec{
			AccessModes: []corev1.PersistentVolumeAccessMode{
				corev1.ReadWriteOnce,
			},
			Resources: corev1.VolumeResourceRequirements{
				Requests: corev1.ResourceList{
					corev1.ResourceStorage: opts.StorageSize,
				},
			},
		},
	},
	}

	volumes := []corev1.Volume{}
	for _, ruleFile := range opts.RuleFiles {
		volumes = append(volumes, corev1.Volume{
			Name: ruleFile.Name,
			VolumeSource: corev1.VolumeSource{
				ConfigMap: &corev1.ConfigMapVolumeSource{
					LocalObjectReference: corev1.LocalObjectReference{
						Name: ruleFile.Name,
					},
				},
			},
		})
	}

	sts := &appsv1.StatefulSet{
		TypeMeta: metav1.TypeMeta{
			Kind:       "StatefulSet",
			APIVersion: appsv1.SchemeGroupVersion.String(),
		},
		ObjectMeta: metav1.ObjectMeta{
			Name:        name,
			Namespace:   opts.Namespace,
			Labels:      objectMetaLabels,
			Annotations: opts.Annotations,
		},
		Spec: appsv1.StatefulSetSpec{
			Replicas:             &opts.Replicas,
			VolumeClaimTemplates: vc,
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
					Containers:         []corev1.Container{rulerContainer},
					ServiceAccountName: name,
					Volumes:            volumes,
				},
			},
			ServiceName: name,
		},
	}

	manifests.AugmentWithOptions(sts, opts.Options)
	return sts
}

func NewRulerService(opts Options) *corev1.Service {
	selectorLabels := opts.GetSelectorLabels()
	objectMetaLabels := GetLabels(opts)
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

func newRulerService(opts Options, selectorLabels, objectMetaLabels map[string]string) *corev1.Service {
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

func rulerArgs(opts Options) []string {
	args := []string{"rule"}
	args = append(args, opts.ToFlags()...)
	args = append(args,
		fmt.Sprintf("--http-address=0.0.0.0:%d", HTTPPort),
		fmt.Sprintf("--grpc-address=0.0.0.0:%d", GRPCPort),
		fmt.Sprintf("--tsdb.retention=%s", string(opts.Retention)),
		"--data-dir=/var/thanos/rule",
		fmt.Sprintf("--objstore.config=$(%s)", rulerObjectStoreEnvVarName),
		fmt.Sprintf("--alertmanagers.url=%s", opts.AlertmanagerURL),
	)

	if opts.EvaluationInterval != "" {
		args = append(args, fmt.Sprintf("--eval-interval=%s", string(opts.EvaluationInterval)))
	}

	for key, val := range opts.ExternalLabels {
		args = append(args, fmt.Sprintf("--label=%s=\"%s\"", key, val))
	}

	for _, ruleFile := range opts.RuleFiles {
		args = append(args, fmt.Sprintf("--rule-file=%s", fmt.Sprintf("/etc/thanos/rules/%s", ruleFile.Key)))
	}

	for _, endpoint := range opts.Endpoints {
		args = append(args, fmt.Sprintf("--query=dnssrv+_http._tcp.%s.%s.svc.cluster.local", endpoint.ServiceName, endpoint.Namespace))
	}

	for _, label := range opts.AlertLabelDrop {
		args = append(args, fmt.Sprintf("---alert.label-drop=%s", label))
	}

	// TODO(saswatamcode): Add some validation.
	if opts.Additional.Args != nil {
		args = append(args, opts.Additional.Args...)
	}

	return args
}

// GetRequiredLabels returns a map of labels that can be used to look up thanos ruler resources.
// These labels are guaranteed to be present on all resources created by this package.
func GetRequiredLabels() map[string]string {
	return map[string]string{
		manifests.NameLabel:      Name,
		manifests.ComponentLabel: ComponentName,
		manifests.PartOfLabel:    manifests.DefaultPartOfLabel,
		manifests.ManagedByLabel: manifests.DefaultManagedByLabel,
	}
}

func (opts Options) GetSelectorLabels() map[string]string {
	labels := GetRequiredLabels()
	labels[manifests.InstanceLabel] = manifests.ValidateAndSanitizeNameToValidLabelValue(opts.GetGeneratedResourceName())
	labels[manifests.OwnerLabel] = manifests.ValidateAndSanitizeNameToValidLabelValue(opts.Owner)
	return labels
}

func GetLabels(opts Options) map[string]string {
	return manifests.MergeLabels(opts.Labels, opts.GetSelectorLabels())
}

func serviceMonitorOpts(from manifests.ServiceMonitorConfig) manifests.ServiceMonitorOptions {
	return manifests.ServiceMonitorOptions{
		Port:     ptr.To(HTTPPortName),
		Interval: from.Interval,
	}
}
