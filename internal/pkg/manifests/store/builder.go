package store

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
	// Name is the name of the Thanos Store component.
	Name = "thanos-store"

	// ComponentName is the name of the Thanos Store component.
	ComponentName = "object-storage-gateway"

	// HTTPPortName is the name of the HTTP port for the Thanos Store components.
	HTTPPortName = "http"
	// HTTPPort is the port number for the HTTP port for the Thanos Store components.
	HTTPPort = 10902
	// GRPCPortName is the name of the gRPC port for the Thanos Store components.
	GRPCPortName = "grpc"
	// GRPCPort is the port number for the gRPC port for the Thanos Store components.
	GRPCPort = 10901
)

// Options for Thanos Store components
// Name is the name of the Thanos Store component
type Options struct {
	manifests.Options
	StorageSize              resource.Quantity
	ObjStoreSecret           corev1.SecretKeySelector
	IndexCacheConfig         manifests.CacheConfig
	CachingBucketConfig      manifests.CacheConfig
	IgnoreDeletionMarksDelay manifests.Duration
	Min, Max                 manifests.Duration
	RelabelConfigs           manifests.RelabelConfigs
	ShardIndex               *int32
}

// Build builds Thanos Store shards.
func (opts Options) Build() []client.Object {
	var objs []client.Object
	selectorLabels := opts.GetSelectorLabels()
	objectMetaLabels := GetLabels(opts)
	name := opts.GetGeneratedResourceName()

	objs = append(objs, manifests.BuildServiceAccount(name, opts.Namespace, selectorLabels, opts.Annotations))
	objs = append(objs, newStoreService(opts, selectorLabels, objectMetaLabels))
	objs = append(objs, newStoreShardStatefulSet(opts, selectorLabels, objectMetaLabels))

	if opts.PodDisruptionConfig != nil {
		objs = append(objs, manifests.NewPodDisruptionBudget(name, opts.Namespace, selectorLabels, objectMetaLabels, opts.Annotations, *opts.PodDisruptionConfig))
	}

	if opts.ServiceMonitorConfig.Enabled {
		objs = append(objs, manifests.BuildServiceMonitor(name, opts.Namespace, objectMetaLabels, selectorLabels, serviceMonitorOpts(opts.ServiceMonitorConfig)))
	}
	return objs
}

// GetGeneratedResourceName returns the name of the Thanos Store component.
// If a shard index is provided, the name will be suffixed with the shard index.
func (opts Options) GetGeneratedResourceName() string {
	name := fmt.Sprintf("%s-%s", Name, opts.Owner)
	if opts.ShardIndex == nil {
		return manifests.ValidateAndSanitizeResourceName(name)
	}
	return manifests.ValidateAndSanitizeResourceName(fmt.Sprintf("%s-shard-%d", name, *opts.ShardIndex))
}

const (
	storeObjectStoreEnvVarName    = "OBJSTORE_CONFIG"
	indexCacheConfigEnvVarName    = "INDEX_CACHE_CONFIG"
	cachingBucketConfigEnvVarName = "CACHING_BUCKET_CONFIG"

	dataVolumeName      = "data"
	dataVolumeMountPath = "var/thanos/store"
)

// NewStoreStatefulSet creates a new StatefulSet for the Thanos Store.
func NewStoreStatefulSet(opts Options) *appsv1.StatefulSet {
	selectorLabels := opts.GetSelectorLabels()
	objectMetaLabels := manifests.MergeLabels(opts.Labels, selectorLabels)
	return newStoreShardStatefulSet(opts, selectorLabels, objectMetaLabels)
}

func newStoreShardStatefulSet(opts Options, selectorLabels, objectMetaLabels map[string]string) *appsv1.StatefulSet {
	name := opts.GetGeneratedResourceName()
	vc := []corev1.PersistentVolumeClaim{
		{
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

	envVars := []corev1.EnvVar{
		{
			Name: storeObjectStoreEnvVarName,
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
	}

	if opts.IndexCacheConfig.FromSecret != nil {
		indexCacheEnv := corev1.EnvVar{
			Name: indexCacheConfigEnvVarName,
			ValueFrom: &corev1.EnvVarSource{
				SecretKeyRef: &corev1.SecretKeySelector{
					LocalObjectReference: corev1.LocalObjectReference{
						Name: opts.IndexCacheConfig.FromSecret.Name,
					},
					Key:      opts.IndexCacheConfig.FromSecret.Key,
					Optional: ptr.To(false),
				},
			},
		}
		envVars = append(envVars, indexCacheEnv)
	}

	if opts.CachingBucketConfig.FromSecret != nil {
		cachingBucketEnv := corev1.EnvVar{
			Name: cachingBucketConfigEnvVarName,
			ValueFrom: &corev1.EnvVarSource{
				SecretKeyRef: &corev1.SecretKeySelector{
					LocalObjectReference: corev1.LocalObjectReference{
						Name: opts.CachingBucketConfig.FromSecret.Name,
					},
					Key:      opts.CachingBucketConfig.FromSecret.Key,
					Optional: ptr.To(false),
				},
			},
		}
		envVars = append(envVars, cachingBucketEnv)
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
			ServiceName: name,
			Replicas:    ptr.To(opts.Replicas),
			Selector: &metav1.LabelSelector{
				MatchLabels: selectorLabels,
			},
			VolumeClaimTemplates: vc,
			Template: corev1.PodTemplateSpec{
				ObjectMeta: metav1.ObjectMeta{
					Labels: objectMetaLabels,
				},
				Spec: corev1.PodSpec{
					SecurityContext:    &corev1.PodSecurityContext{},
					ServiceAccountName: name,
					Containers: []corev1.Container{
						{
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
										Path: "/-/ready",
										Port: intstr.FromInt32(HTTPPort),
									},
								},
								InitialDelaySeconds: 20,
								TimeoutSeconds:      1,
								PeriodSeconds:       30,
								SuccessThreshold:    1,
								FailureThreshold:    15,
							},
							LivenessProbe: &corev1.Probe{
								ProbeHandler: corev1.ProbeHandler{
									HTTPGet: &corev1.HTTPGetAction{
										Path: "/-/healthy",
										Port: intstr.FromInt32(HTTPPort),
									},
								},
								InitialDelaySeconds: 60,
								TimeoutSeconds:      1,
								PeriodSeconds:       30,
								SuccessThreshold:    1,
								FailureThreshold:    8,
							},
							Env: envVars,
							VolumeMounts: []corev1.VolumeMount{
								{
									Name:      dataVolumeName,
									MountPath: dataVolumeMountPath,
								},
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
							Args:                     storeArgsFrom(opts),
						},
					},
				},
			},
		},
	}
	manifests.AugmentWithOptions(sts, opts.Options)
	return sts
}

// NewStoreService creates a new Service for Thanos Store shard.
func NewStoreService(opts Options) *corev1.Service {
	selectorLabels := opts.GetSelectorLabels()
	return newStoreService(opts, selectorLabels, GetLabels(opts))
}

func newStoreService(opts Options, selectorLabels, objectMetaLabels map[string]string) *corev1.Service {
	svc := newService(opts, selectorLabels, objectMetaLabels)
	svc.Spec.ClusterIP = corev1.ClusterIPNone
	if opts.Additional.ServicePorts != nil {
		svc.Spec.Ports = append(svc.Spec.Ports, opts.Additional.ServicePorts...)
	}

	return svc
}

// newService creates a new Service for the Thanos Store shards.
func newService(opts Options, selectorLabels, objectMetaLabels map[string]string) *corev1.Service {
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

	svc := &corev1.Service{
		TypeMeta: metav1.TypeMeta{
			Kind: "Service",
		},
		ObjectMeta: metav1.ObjectMeta{
			Name:        opts.GetGeneratedResourceName(),
			Namespace:   opts.Namespace,
			Labels:      objectMetaLabels,
			Annotations: opts.Annotations,
		},
		Spec: corev1.ServiceSpec{
			Selector: selectorLabels,
			Ports:    servicePorts,
		},
	}
	return svc
}

func storeArgsFrom(opts Options) []string {
	args := []string{"store"}
	args = append(args, opts.ToFlags()...)
	args = append(args,
		fmt.Sprintf("--grpc-address=0.0.0.0:%d", GRPCPort),
		fmt.Sprintf("--http-address=0.0.0.0:%d", HTTPPort),
		fmt.Sprintf("--objstore.config=$(%s)", storeObjectStoreEnvVarName),
		"--data-dir=/var/thanos/store",
		fmt.Sprintf("--ignore-deletion-marks-delay=%s", string(opts.IgnoreDeletionMarksDelay)),
		fmt.Sprintf("--min-time=%s", string(opts.Min)),
		fmt.Sprintf("--max-time=%s", string(opts.Max)),
	)

	if opts.IndexCacheConfig.FromSecret != nil {
		args = append(args, fmt.Sprintf("--index-cache.config=$(%s)", indexCacheConfigEnvVarName))
	} else if opts.IndexCacheConfig.InMemoryCacheConfig != nil {
		args = append(args, fmt.Sprintf("--index-cache.config=%s", opts.IndexCacheConfig.InMemoryCacheConfig.String()))
	}

	if opts.CachingBucketConfig.FromSecret != nil {
		args = append(args, fmt.Sprintf("--store.caching-bucket.config=$(%s)", cachingBucketConfigEnvVarName))
	} else if opts.CachingBucketConfig.InMemoryCacheConfig != nil {
		args = append(args, fmt.Sprintf("--store.caching-bucket.config=%s", opts.CachingBucketConfig.InMemoryCacheConfig.String()))
	}

	if len(opts.RelabelConfigs) > 0 {
		args = append(args, opts.RelabelConfigs.ToFlags())
	}

	// TODO(saswatamcode): Add some validation.
	if opts.Additional.Args != nil {
		args = append(args, opts.Additional.Args...)
	}

	return manifests.PruneEmptyArgs(args)
}

// GetRequiredStoreServiceLabel returns the minimum set of labels that can be used to look up Services
// that implement the Store API. Implementations of manifests.Buildable that provide Store API services
// should include these labels in their Service ObjectMeta.
func GetRequiredStoreServiceLabel() map[string]string {
	return map[string]string{
		manifests.PartOfLabel:          manifests.DefaultPartOfLabel,
		manifests.DefaultStoreAPILabel: manifests.DefaultStoreAPIValue,
	}
}

// GetRequiredLabels returns a map of labels that can be used to look up store resources.
// These labels are guaranteed to be present on all resources created by this package.
func GetRequiredLabels() map[string]string {
	labels := map[string]string{
		manifests.NameLabel:      Name,
		manifests.ComponentLabel: ComponentName,
		manifests.ManagedByLabel: manifests.DefaultManagedByLabel,
	}
	return manifests.MergeLabels(labels, GetRequiredStoreServiceLabel())
}

// GetSelectorLabels returns a map of labels that can be used to select store resources.
func (opts Options) GetSelectorLabels() map[string]string {
	labels := GetRequiredLabels()
	labels[manifests.InstanceLabel] = manifests.ValidateAndSanitizeNameToValidLabelValue(opts.GetGeneratedResourceName())
	labels[manifests.OwnerLabel] = manifests.ValidateAndSanitizeNameToValidLabelValue(opts.Owner)
	return labels
}

// GetLabels returns the labels that will be set as ObjectMeta labels for store resources.
func GetLabels(opts Options) map[string]string {
	lbls := manifests.MergeLabels(opts.Labels, opts.GetSelectorLabels())
	if opts.Replicas > 1 {
		lbls[string(manifests.GroupLabel)] = "true"
	}
	return lbls
}

func serviceMonitorOpts(from manifests.ServiceMonitorConfig) manifests.ServiceMonitorOptions {
	return manifests.ServiceMonitorOptions{
		Port:     ptr.To(HTTPPortName),
		Interval: from.Interval,
	}
}
