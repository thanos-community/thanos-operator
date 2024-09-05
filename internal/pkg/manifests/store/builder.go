package store

import (
	"fmt"

	"github.com/thanos-community/thanos-operator/internal/pkg/manifests"

	appsv1 "k8s.io/api/apps/v1"
	corev1 "k8s.io/api/core/v1"
	"k8s.io/apimachinery/pkg/api/resource"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/util/intstr"
	"k8s.io/apimachinery/pkg/util/validation"
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

// StoreOptions for Thanos Store components
type StoreOptions struct {
	manifests.Options
	StorageSize              resource.Quantity
	ObjStoreSecret           corev1.SecretKeySelector
	IndexCacheConfig         *corev1.ConfigMapKeySelector
	CachingBucketConfig      *corev1.ConfigMapKeySelector
	IgnoreDeletionMarksDelay manifests.Duration
	Min, Max                 manifests.Duration
	Shards                   int32
	Additional               manifests.Additional
}

// BuildStores builds Thanos Store shards.
func BuildStores(opts StoreOptions) []client.Object {
	var objs []client.Object
	objs = append(objs, manifests.BuildServiceAccount(opts.Options))
	objs = append(objs, NewStoreServices(opts)...)
	objs = append(objs, NewStoreStatefulSets(opts)...)

	if opts.IndexCacheConfig == nil || opts.CachingBucketConfig == nil {
		objs = append(objs, NewStoreInMemoryConfigMap(opts))
	}
	return objs
}

const (
	storeObjectStoreEnvVarName    = "OBJSTORE_CONFIG"
	indexCacheConfigEnvVarName    = "INDEX_CACHE_CONFIG"
	cachingBucketConfigEnvVarName = "CACHING_BUCKET_CONFIG"

	dataVolumeName      = "data"
	dataVolumeMountPath = "var/thanos/store"

	defaultInMemoryConfigmapName = "thanos-store-inmemory-config"
	defaultInMemoryConfigmapKey  = "config.yaml"

	// InMemoryConfig is the default configuration for the in-memory cache.
	// Only used if user does not provide an index cache or caching bucket configuration.
	// Set to have conservative limits.
	InMemoryConfig = `type: IN-MEMORY
config:
  max_size: 512MiB
  max_item_size: 5MiB`
)

func NewStoreInMemoryConfigMap(opts StoreOptions) client.Object {
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

// NewStoreStatefulSets creates a new StatefulSet for the Thanos Store.
func NewStoreStatefulSets(opts StoreOptions) []client.Object {
	defaultLabels := labelsForStoreShard(opts)
	aggregatedLabels := manifests.MergeLabels(opts.Labels, defaultLabels)

	shardSts := make([]client.Object, opts.Shards)
	originalName := opts.Name
	for i := 0; i < int(opts.Shards); i++ {
		opts.Name = StoreShardName(originalName, i)
		shardSts[i] = newStoreShardStatefulSet(opts, originalName, defaultLabels, aggregatedLabels, i)
	}

	return shardSts
}

func newStoreShardStatefulSet(opts StoreOptions, SAName string, defaultLabels map[string]string, aggregatedLabels map[string]string, shardIndex int) *appsv1.StatefulSet {
	vc := []corev1.PersistentVolumeClaim{
		{
			ObjectMeta: metav1.ObjectMeta{
				Name:      dataVolumeName,
				Namespace: opts.Namespace,
				Labels:    aggregatedLabels,
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

	indexCacheEnv := corev1.EnvVar{
		Name: indexCacheConfigEnvVarName,
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
	if opts.IndexCacheConfig != nil {
		indexCacheEnv = corev1.EnvVar{
			Name: indexCacheConfigEnvVarName,
			ValueFrom: &corev1.EnvVarSource{
				ConfigMapKeyRef: &corev1.ConfigMapKeySelector{
					LocalObjectReference: corev1.LocalObjectReference{
						Name: opts.IndexCacheConfig.Name,
					},
					Key:      opts.IndexCacheConfig.Key,
					Optional: ptr.To(false),
				},
			},
		}
	}

	cachingBucketEnv := corev1.EnvVar{
		Name: cachingBucketConfigEnvVarName,
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
	if opts.CachingBucketConfig != nil {
		cachingBucketEnv = corev1.EnvVar{
			Name: cachingBucketConfigEnvVarName,
			ValueFrom: &corev1.EnvVarSource{
				ConfigMapKeyRef: &corev1.ConfigMapKeySelector{
					LocalObjectReference: corev1.LocalObjectReference{
						Name: opts.CachingBucketConfig.Name,
					},
					Key:      opts.CachingBucketConfig.Key,
					Optional: ptr.To(false),
				},
			},
		}
	}
	envVars = append(envVars, indexCacheEnv, cachingBucketEnv)

	sts := &appsv1.StatefulSet{
		TypeMeta: metav1.TypeMeta{
			Kind:       "StatefulSet",
			APIVersion: appsv1.SchemeGroupVersion.String(),
		},
		ObjectMeta: metav1.ObjectMeta{
			Name:      opts.Name,
			Namespace: opts.Namespace,
			Labels:    aggregatedLabels,
		},
		Spec: appsv1.StatefulSetSpec{
			ServiceName: opts.Name,
			Replicas:    ptr.To(opts.Replicas),
			Selector: &metav1.LabelSelector{
				MatchLabels: defaultLabels,
			},
			VolumeClaimTemplates: vc,
			Template: corev1.PodTemplateSpec{
				ObjectMeta: metav1.ObjectMeta{
					Labels: aggregatedLabels,
				},
				Spec: corev1.PodSpec{
					SecurityContext:    &corev1.PodSecurityContext{},
					ServiceAccountName: SAName,
					Containers: []corev1.Container{
						{
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
							Args:                     storeArgsFrom(opts, shardIndex),
						},
					},
				},
			},
		},
	}

	if opts.Additional.VolumeMounts != nil {
		sts.Spec.Template.Spec.Containers[0].VolumeMounts = append(
			sts.Spec.Template.Spec.Containers[0].VolumeMounts,
			opts.Additional.VolumeMounts...)
	}

	if opts.Additional.Containers != nil {
		sts.Spec.Template.Spec.Containers = append(
			sts.Spec.Template.Spec.Containers,
			opts.Additional.Containers...)
	}

	if opts.Additional.Volumes != nil {
		sts.Spec.Template.Spec.Volumes = append(
			sts.Spec.Template.Spec.Volumes,
			opts.Additional.Volumes...)
	}

	if opts.Additional.Ports != nil {
		sts.Spec.Template.Spec.Containers[0].Ports = append(
			sts.Spec.Template.Spec.Containers[0].Ports,
			opts.Additional.Ports...)
	}

	if opts.Additional.Env != nil {
		sts.Spec.Template.Spec.Containers[0].Env = append(
			sts.Spec.Template.Spec.Containers[0].Env,
			opts.Additional.Env...)
	}

	return sts
}

// NewStoreServices creates a new Services for each Thanos Store shard.
func NewStoreServices(opts StoreOptions) []client.Object {
	shardSvc := make([]client.Object, opts.Shards)
	originalName := opts.Name
	for i := 0; i < int(opts.Shards); i++ {
		defaultLabels := labelsForStoreShard(opts)
		opts.Labels = manifests.MergeLabels(opts.Labels, defaultLabels)
		opts.Name = StoreShardName(originalName, i)

		svc := newService(opts.Options, defaultLabels)
		svc.Spec.ClusterIP = corev1.ClusterIPNone
		if opts.Additional.ServicePorts != nil {
			svc.Spec.Ports = append(svc.Spec.Ports, opts.Additional.ServicePorts...)
		}

		shardSvc[i] = svc
	}

	return shardSvc
}

// newService creates a new Service for the Thanos Store shards.
func newService(opts manifests.Options, selectorLabels map[string]string) *corev1.Service {
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
		ObjectMeta: metav1.ObjectMeta{
			Name:      opts.Name,
			Namespace: opts.Namespace,
			Labels:    opts.Labels,
		},
		Spec: corev1.ServiceSpec{
			Selector: selectorLabels,
			Ports:    servicePorts,
		},
	}
	return svc
}

// StoreShardName generates name for a Thanos Store shard.
func StoreShardName(parentName string, shardIndex int) string {
	name := fmt.Sprintf("%s-shard-%d", parentName, shardIndex)
	// check if the name is a valid DNS-1123 subdomain
	if len(validation.IsDNS1123Subdomain(name)) == 0 {
		return name
	}

	// default to standard simple shard name.
	return fmt.Sprintf("%s-%d", Name, shardIndex)
}

func storeArgsFrom(opts StoreOptions, shardIndex int) []string {
	opts.Options = opts.ApplyDefaults()
	args := []string{
		"store",
		fmt.Sprintf("--log.level=%s", *opts.LogLevel),
		fmt.Sprintf("--log.format=%s", *opts.LogFormat),
		fmt.Sprintf("--grpc-address=0.0.0.0:%d", GRPCPort),
		fmt.Sprintf("--http-address=0.0.0.0:%d", HTTPPort),
		fmt.Sprintf("--objstore.config=$(%s)", storeObjectStoreEnvVarName),
		fmt.Sprintf("--index-cache.config=$(%s)", indexCacheConfigEnvVarName),
		fmt.Sprintf("--store.caching-bucket.config=$(%s)", cachingBucketConfigEnvVarName),
		"--data-dir=/var/thanos/store",
		fmt.Sprintf("--ignore-deletion-marks-delay=%s", string(opts.IgnoreDeletionMarksDelay)),
		fmt.Sprintf(`--selector.relabel-config=
              - action: hashmod
                source_labels: ["__block_id"]
                target_label: shard
                modulus: %d
              - action: keep
                source_labels: ["shard"]
                regex: %d`, opts.Shards, shardIndex),
		fmt.Sprintf("--min-time=%s", string(opts.Min)),
		fmt.Sprintf("--max-time=%s", string(opts.Max)),
	}

	// TODO(saswatamcode): Add some validation.
	if opts.Additional.Args != nil {
		args = append(args, opts.Additional.Args...)
	}

	return manifests.PruneEmptyArgs(args)
}

func labelsForStoreShard(opts StoreOptions) map[string]string {
	return map[string]string{
		manifests.NameLabel:            Name,
		manifests.ComponentLabel:       ComponentName,
		manifests.InstanceLabel:        opts.Name,
		manifests.PartOfLabel:          manifests.DefaultPartOfLabel,
		manifests.ManagedByLabel:       manifests.DefaultManagedByLabel,
		manifests.DefaultStoreAPILabel: manifests.DefaultStoreAPIValue,
	}
}
