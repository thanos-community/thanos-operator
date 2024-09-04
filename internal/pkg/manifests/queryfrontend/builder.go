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

// QueryFrontendOptions for Thanos Query Frontend
type QueryFrontendOptions struct {
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
	Additional             manifests.Additional
}

func BuildQueryFrontend(opts QueryFrontendOptions) []client.Object {
	var objs []client.Object
	objs = append(objs, manifests.BuildServiceAccount(opts.Options))
	objs = append(objs, NewQueryFrontendDeployment(opts))
	objs = append(objs, NewQueryFrontendService(opts))

	if opts.ResponseCacheConfig == nil {
		objs = append(objs, NewQueryFrontendInMemoryConfigMap(opts))
	}
	return objs
}

func NewQueryFrontendInMemoryConfigMap(opts QueryFrontendOptions) client.Object {
	return &corev1.ConfigMap{
		ObjectMeta: metav1.ObjectMeta{
			Name:      defaultInMemoryConfigmapName,
			Namespace: opts.Namespace,
			Labels:    opts.Labels,
		},
		Data: map[string]string{
			defaultInMemoryConfigmapKey: InMemoryConfig,
		},
	}
}

func NewQueryFrontendDeployment(opts QueryFrontendOptions) *appsv1.Deployment {
	defaultLabels := labelsForQueryFrontend(opts)
	aggregatedLabels := manifests.MergeLabels(opts.Labels, defaultLabels)

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
					ServiceAccountName: opts.Name,
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

	if opts.Additional.VolumeMounts != nil {
		deployment.Spec.Template.Spec.Containers[0].VolumeMounts = append(
			deployment.Spec.Template.Spec.Containers[0].VolumeMounts,
			opts.Additional.VolumeMounts...)
	}

	if opts.Additional.Containers != nil {
		deployment.Spec.Template.Spec.Containers = append(
			deployment.Spec.Template.Spec.Containers,
			opts.Additional.Containers...)
	}

	if opts.Additional.Volumes != nil {
		deployment.Spec.Template.Spec.Volumes = append(
			deployment.Spec.Template.Spec.Volumes,
			opts.Additional.Volumes...)
	}

	if opts.Additional.Ports != nil {
		deployment.Spec.Template.Spec.Containers[0].Ports = append(
			deployment.Spec.Template.Spec.Containers[0].Ports,
			opts.Additional.Ports...)
	}

	if opts.Additional.Env != nil {
		deployment.Spec.Template.Spec.Containers[0].Env = append(
			deployment.Spec.Template.Spec.Containers[0].Env,
			opts.Additional.Env...)
	}

	return deployment
}

func NewQueryFrontendService(opts QueryFrontendOptions) *corev1.Service {
	defaultLabels := labelsForQueryFrontend(opts)
	aggregatedLabels := manifests.MergeLabels(opts.Labels, defaultLabels)

	service := &corev1.Service{
		ObjectMeta: metav1.ObjectMeta{
			Name:      opts.Name,
			Namespace: opts.Namespace,
			Labels:    aggregatedLabels,
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
			Selector: defaultLabels,
		},
	}

	if opts.Additional.ServicePorts != nil {
		service.Spec.Ports = append(service.Spec.Ports, opts.Additional.ServicePorts...)
	}

	return service
}

func queryFrontendArgs(opts QueryFrontendOptions) []string {
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

func labelsForQueryFrontend(opts QueryFrontendOptions) map[string]string {
	return map[string]string{
		manifests.NameLabel:      Name,
		manifests.ComponentLabel: ComponentName,
		manifests.InstanceLabel:  opts.Name,
		manifests.PartOfLabel:    manifests.DefaultPartOfLabel,
		manifests.ManagedByLabel: manifests.DefaultManagedByLabel,
	}
}
