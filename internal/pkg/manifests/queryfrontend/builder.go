package queryfrontend

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
	// Name is the name of the Thanos Query Frontend component.
	Name = "thanos-query-frontend"

	// ComponentName is the name of the Thanos Query Frontend component.
	ComponentName = "query-frontend"

	HTTPPort     = 9090
	HTTPPortName = "http"

	externalCacheEnvVarName = "CACHE_CONFIG"
)

// Options for Thanos Query Frontend
type Options struct {
	manifests.Options
	QueryService           string
	QueryPort              int32
	LogQueriesLongerThan   manifests.Duration
	CompressResponses      bool
	ResponseCacheConfig    manifests.CacheConfig
	RangeSplitInterval     manifests.Duration
	LabelsSplitInterval    manifests.Duration
	RangeMaxRetries        int
	LabelsMaxRetries       int
	LabelsDefaultTimeRange manifests.Duration
}

func (opts Options) Build() []client.Object {
	var objs []client.Object
	selectorLabels := opts.GetSelectorLabels()
	objectMetaLabels := GetLabels(opts)
	name := opts.GetGeneratedResourceName()

	objs = append(objs, manifests.BuildServiceAccount(opts.GetGeneratedResourceName(), opts.Namespace, selectorLabels, opts.Annotations))
	objs = append(objs, newQueryFrontendDeployment(opts, selectorLabels, objectMetaLabels))
	objs = append(objs, newQueryFrontendService(opts, selectorLabels, objectMetaLabels))

	if opts.PodDisruptionConfig != nil {
		objs = append(objs, manifests.NewPodDisruptionBudget(name, opts.Namespace, selectorLabels, objectMetaLabels, opts.Annotations, *opts.PodDisruptionConfig))
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

func NewQueryFrontendDeployment(opts Options) *appsv1.Deployment {
	selectorLabels := opts.GetSelectorLabels()
	objectMetaLabels := GetLabels(opts)
	return newQueryFrontendDeployment(opts, selectorLabels, objectMetaLabels)
}

func newQueryFrontendDeployment(opts Options, selectorLabels, objectMetaLabels map[string]string) *appsv1.Deployment {
	name := opts.GetGeneratedResourceName()
	var env []corev1.EnvVar

	if opts.ResponseCacheConfig.FromSecret != nil {
		env = make([]corev1.EnvVar, 1)
		cacheConfigEnv := corev1.EnvVar{
			Name: externalCacheEnvVarName,
			ValueFrom: &corev1.EnvVarSource{
				SecretKeyRef: opts.ResponseCacheConfig.FromSecret,
			},
		}
		env[0] = cacheConfigEnv
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
					ServiceAccountName: name,
					SecurityContext:    &corev1.PodSecurityContext{},
					Containers: []corev1.Container{
						{
							Name:  Name,
							Image: opts.GetContainerImage(),
							Args:  queryFrontendArgs(opts),
							Ports: []corev1.ContainerPort{
								{
									Name:          HTTPPortName,
									ContainerPort: HTTPPort,
									Protocol:      corev1.ProtocolTCP,
								},
							},
							Env: env,
							SecurityContext: &corev1.SecurityContext{
								AllowPrivilegeEscalation: ptr.To(false),
								RunAsNonRoot:             ptr.To(true),
								Capabilities: &corev1.Capabilities{
									Drop: []corev1.Capability{
										"ALL",
									},
								},
							},
							LivenessProbe: &corev1.Probe{
								ProbeHandler: corev1.ProbeHandler{
									HTTPGet: &corev1.HTTPGetAction{
										Path: "/-/healthy",
										Port: intstr.FromInt32(HTTPPort),
									},
								},
							},
							ReadinessProbe: &corev1.Probe{
								ProbeHandler: corev1.ProbeHandler{
									HTTPGet: &corev1.HTTPGetAction{
										Path: "/-/ready",
										Port: intstr.FromInt32(HTTPPort),
									},
								},
							},
						},
					},
				},
			},
		},
	}
	manifests.AugmentWithOptions(deployment, opts.Options)
	return deployment
}

func NewQueryFrontendService(opts Options) *corev1.Service {
	selectorLabels := opts.GetSelectorLabels()
	objectMetaLabels := GetLabels(opts)
	return newQueryFrontendService(opts, selectorLabels, objectMetaLabels)
}

func newQueryFrontendService(opts Options, selectorLabels, objectMetaLabels map[string]string) *corev1.Service {
	service := &corev1.Service{
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
			Ports: []corev1.ServicePort{
				{
					Name:       HTTPPortName,
					Port:       HTTPPort,
					TargetPort: intstr.FromInt32(HTTPPort),
					Protocol:   corev1.ProtocolTCP,
				},
			},
			Selector: selectorLabels,
		},
	}

	if opts.Additional.ServicePorts != nil {
		service.Spec.Ports = append(service.Spec.Ports, opts.Additional.ServicePorts...)
	}

	return service
}

func queryFrontendArgs(opts Options) []string {
	args := []string{
		"query-frontend",
		fmt.Sprintf("--http-address=0.0.0.0:%d", HTTPPort),
		fmt.Sprintf("--query-frontend.downstream-url=http://%s.%s.svc.cluster.local:%d", opts.QueryService, opts.Namespace, opts.QueryPort),
		fmt.Sprintf("--query-frontend.log-queries-longer-than=%s", opts.LogQueriesLongerThan),
		fmt.Sprintf("--query-range.split-interval=%s", opts.RangeSplitInterval),
		fmt.Sprintf("--labels.split-interval=%s", opts.LabelsSplitInterval),
		fmt.Sprintf("--query-range.max-retries-per-request=%d", opts.RangeMaxRetries),
		fmt.Sprintf("--labels.max-retries-per-request=%d", opts.LabelsMaxRetries),
		fmt.Sprintf("--labels.default-time-range=%s", opts.LabelsDefaultTimeRange),
		"--cache-compression-type=snappy",
	}

	if opts.ResponseCacheConfig.FromSecret != nil {
		args = append(args, fmt.Sprintf("--query-range.response-cache-config=$(%s)", externalCacheEnvVarName))
		args = append(args, fmt.Sprintf("--labels.response-cache-config=$(%s)", externalCacheEnvVarName))
	} else if opts.ResponseCacheConfig.InMemoryCacheConfig != nil {
		conf := opts.ResponseCacheConfig.InMemoryCacheConfig.String()
		args = append(args, fmt.Sprintf("--query-range.response-cache-config=%s", conf))
		args = append(args, fmt.Sprintf("--labels.response-cache-config=%s", conf))
	}

	if opts.CompressResponses {
		args = append(args, "--query-frontend.compress-responses")
	}

	if opts.Additional.Args != nil {
		args = append(args, opts.Additional.Args...)
	}

	return manifests.PruneEmptyArgs(args)
}

// GetRequiredLabels returns a map of labels that can be used to look up qfe resources.
// These labels are guaranteed to be present on all resources created by this package.
func GetRequiredLabels() map[string]string {
	return map[string]string{
		manifests.NameLabel:      Name,
		manifests.ComponentLabel: ComponentName,
		manifests.PartOfLabel:    manifests.DefaultPartOfLabel,
		manifests.ManagedByLabel: manifests.DefaultManagedByLabel,
	}
}

// GetSelectorLabels returns a map of labels that can be used to look up qfe resources.
func (opts Options) GetSelectorLabels() map[string]string {
	labels := GetRequiredLabels()
	labels[manifests.InstanceLabel] = manifests.ValidateAndSanitizeNameToValidLabelValue(opts.GetGeneratedResourceName())
	labels[manifests.OwnerLabel] = manifests.ValidateAndSanitizeNameToValidLabelValue(opts.getOwner())
	return labels
}

// GetLabels returns a map of labels that can be used to look up qfe resources.
func GetLabels(opts Options) map[string]string {
	return manifests.MergeLabels(opts.Labels, opts.GetSelectorLabels())
}
