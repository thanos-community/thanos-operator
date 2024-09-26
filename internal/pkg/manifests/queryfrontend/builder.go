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

	defaultInMemoryConfigmapName = "thanos-query-frontend-inmemory-config"
	defaultInMemoryConfigmapKey  = "config.yaml"

	// InMemoryConfig is the default configuration for the in-memory cache.
	InMemoryConfig = `type: IN-MEMORY
config:
  max_size: 512MiB
  max_item_size: 5MiB`
)

// Options for Thanos Query Frontend
type Options struct {
	manifests.Options
	QueryService           string
	QueryPort              int32
	LogQueriesLongerThan   manifests.Duration
	CompressResponses      bool
	ResponseCacheConfig    *corev1.ConfigMapKeySelector
	RangeSplitInterval     manifests.Duration
	LabelsSplitInterval    manifests.Duration
	RangeMaxRetries        int
	LabelsMaxRetries       int
	LabelsDefaultTimeRange manifests.Duration
}

func BuildQueryFrontend(opts Options) []client.Object {
	var objs []client.Object
	selectorLabels := labelsForQueryFrontend(opts)
	objectMetaLabels := manifests.MergeLabels(opts.Labels, selectorLabels)

	objs = append(objs, manifests.BuildServiceAccount(opts.Name, opts.Namespace, selectorLabels))
	objs = append(objs, newQueryFrontendDeployment(opts, selectorLabels, objectMetaLabels))
	objs = append(objs, newQueryFrontendService(opts, selectorLabels, objectMetaLabels))

	if opts.ResponseCacheConfig == nil {
		objs = append(objs, newQueryFrontendInMemoryConfigMap(opts, objectMetaLabels))
	}
	return objs
}

func NewQueryFrontendInMemoryConfigMap(opts Options) client.Object {
	return newQueryFrontendInMemoryConfigMap(opts, manifests.MergeLabels(opts.Labels, labelsForQueryFrontend(opts)))
}

func newQueryFrontendInMemoryConfigMap(opts Options, labels map[string]string) client.Object {
	return &corev1.ConfigMap{
		ObjectMeta: metav1.ObjectMeta{
			Name:      defaultInMemoryConfigmapName,
			Namespace: opts.Namespace,
			Labels:    labels,
		},
		Data: map[string]string{
			defaultInMemoryConfigmapKey: InMemoryConfig,
		},
	}
}

func NewQueryFrontendDeployment(opts Options) *appsv1.Deployment {
	selectorLabels := labelsForQueryFrontend(opts)
	return newQueryFrontendDeployment(opts, selectorLabels, manifests.MergeLabels(opts.Labels, selectorLabels))
}

func newQueryFrontendDeployment(opts Options, selectorLabels, objectMetaLabels map[string]string) *appsv1.Deployment {
	cacheConfigEnv := corev1.EnvVar{
		Name: "CACHE_CONFIG",
		ValueFrom: &corev1.EnvVarSource{
			ConfigMapKeyRef: &corev1.ConfigMapKeySelector{
				LocalObjectReference: corev1.LocalObjectReference{
					Name: defaultInMemoryConfigmapName,
				},
				Key:      defaultInMemoryConfigmapKey,
				Optional: ptr.To(false),
			},
		},
	}
	if opts.ResponseCacheConfig != nil {
		cacheConfigEnv = corev1.EnvVar{
			Name: "CACHE_CONFIG",
			ValueFrom: &corev1.EnvVarSource{
				ConfigMapKeyRef: opts.ResponseCacheConfig,
			},
		}
	}

	deployment := &appsv1.Deployment{
		ObjectMeta: metav1.ObjectMeta{
			Name:      opts.Name,
			Namespace: opts.Namespace,
			Labels:    objectMetaLabels,
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
					ServiceAccountName: opts.Name,
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
							Env: []corev1.EnvVar{cacheConfigEnv},
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
										Port: intstr.FromInt(HTTPPort),
									},
								},
							},
							ReadinessProbe: &corev1.Probe{
								ProbeHandler: corev1.ProbeHandler{
									HTTPGet: &corev1.HTTPGetAction{
										Path: "/-/ready",
										Port: intstr.FromInt(HTTPPort),
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
	selectorLabels := labelsForQueryFrontend(opts)
	return newQueryFrontendService(opts, selectorLabels, manifests.MergeLabels(opts.Labels, selectorLabels))
}

func newQueryFrontendService(opts Options, selectorLabels, objectMetaLabels map[string]string) *corev1.Service {
	service := &corev1.Service{
		ObjectMeta: metav1.ObjectMeta{
			Name:      opts.Name,
			Namespace: opts.Namespace,
			Labels:    objectMetaLabels,
		},
		Spec: corev1.ServiceSpec{
			Ports: []corev1.ServicePort{
				{
					Name:       HTTPPortName,
					Port:       HTTPPort,
					TargetPort: intstr.FromInt(HTTPPort),
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
		"--query-range.response-cache-config=$(CACHE_CONFIG)",
		"--labels.response-cache-config=$(CACHE_CONFIG)",
		"--cache-compression-type=snappy",
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

func labelsForQueryFrontend(opts Options) map[string]string {
	labels := GetRequiredLabels()
	labels[manifests.InstanceLabel] = opts.Name
	return labels
}
