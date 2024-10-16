package compact

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
	// Name is the name of the Thanos Compact component.
	Name = "thanos-compact"

	// ComponentName is the name of the Thanos Compact component.
	ComponentName = "thanos-compactor"

	// HTTPPort is the port used by the HTTP server.
	HTTPPort     = 10902
	HTTPPortName = "http"
)

const (
	objectStoreEnvVarName = "OBJSTORE_CONFIG"
	dataVolumeName        = "data"
	dataVolumeMountPath   = "var/thanos/compact"
)

// Options for Thanos Compact
type Options struct {
	manifests.Options
	RetentionOptions *RetentionOptions
	BlockConfig      *BlockConfigOptions
	Compaction       *CompactionOptions
	Downsampling     *DownsamplingOptions
	// RelabelConfig is the relabel configuration for the Thanos Compact shard.
	RelabelConfigs manifests.RelabelConfigs
	// StorageSize is the size of the PVC to create for the Thanos Compact shard.
	StorageSize resource.Quantity
	// ObjStoreSecret is the secret key selector for the object store configuration.
	ObjStoreSecret corev1.SecretKeySelector
	// Min and Max time for the compactor
	Min, Max   *manifests.Duration
	ShardName  *string
	ShardIndex *int
}

// Build compiles all the Kubernetes objects for the Thanos Compact shard.
// This includes the ServiceAccount, StatefulSet, and Service.
// Build ignores any manifests.PodDisruptionBudgetOptions as well as replicas set in the Options since
// enforce running compactor as a single replica.
func (opts Options) Build() []client.Object {
	var objs []client.Object
	selectorLabels := opts.GetSelectorLabels()
	objectMetaLabels := manifests.MergeLabels(opts.Labels, selectorLabels)

	objs = append(objs, manifests.BuildServiceAccount(opts.GetGeneratedResourceName(), opts.Namespace, selectorLabels, opts.Annotations))
	objs = append(objs, newShardStatefulSet(opts, selectorLabels, objectMetaLabels))
	objs = append(objs, newService(opts, selectorLabels, objectMetaLabels))

	return objs
}

// GetGeneratedResourceName returns the generated name for the Thanos Compact or shard.
// If no sharding is configured, the name will be generated from the Options.Owner.
// If sharding is configured, the name will be generated from the Options.Owner, ShardName, and ShardIndex.
func (opts Options) GetGeneratedResourceName() string {
	name := fmt.Sprintf("%s-%s", Name, opts.getOwner())
	if opts.ShardName != nil && opts.ShardIndex != nil {
		name = fmt.Sprintf("%s-%s-%d", name, *opts.ShardName, *opts.ShardIndex)
	}
	return manifests.ValidateAndSanitizeResourceName(name)
}

func (opts Options) getOwner() string {
	return opts.Owner
}

// NewStatefulSet creates a new StatefulSet for the Thanos Compact shard.
func NewStatefulSet(opts Options) *appsv1.StatefulSet {
	selectorLabels := opts.GetSelectorLabels()
	return newShardStatefulSet(opts, selectorLabels, manifests.MergeLabels(opts.Labels, selectorLabels))
}

func newShardStatefulSet(opts Options, selectorLabels map[string]string, metaLabels map[string]string) *appsv1.StatefulSet {
	name := opts.GetGeneratedResourceName()
	vc := []corev1.PersistentVolumeClaim{
		{
			ObjectMeta: metav1.ObjectMeta{
				Name:      dataVolumeName,
				Namespace: opts.Namespace,
				Labels:    metaLabels,
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
			Name: objectStoreEnvVarName,
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

	sts := &appsv1.StatefulSet{
		TypeMeta: metav1.TypeMeta{
			Kind:       "StatefulSet",
			APIVersion: appsv1.SchemeGroupVersion.String(),
		},
		ObjectMeta: metav1.ObjectMeta{
			Name:        name,
			Namespace:   opts.Namespace,
			Labels:      metaLabels,
			Annotations: opts.Annotations,
		},
		Spec: appsv1.StatefulSetSpec{
			PersistentVolumeClaimRetentionPolicy: &appsv1.StatefulSetPersistentVolumeClaimRetentionPolicy{
				WhenDeleted: appsv1.DeletePersistentVolumeClaimRetentionPolicyType,
				WhenScaled:  appsv1.DeletePersistentVolumeClaimRetentionPolicyType,
			},
			ServiceName: name,
			Replicas:    ptr.To(int32(1)),
			Selector: &metav1.LabelSelector{
				MatchLabels: selectorLabels,
			},
			VolumeClaimTemplates: vc,
			Template: corev1.PodTemplateSpec{
				ObjectMeta: metav1.ObjectMeta{
					Labels: metaLabels,
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
									ContainerPort: HTTPPort,
									Name:          HTTPPortName,
								},
							},
							TerminationMessagePolicy: corev1.TerminationMessageFallbackToLogsOnError,
							TerminationMessagePath:   corev1.TerminationMessagePathDefault,
							Args:                     compactorArgsFrom(opts),
						},
					},
				},
			},
		},
	}
	manifests.AugmentWithOptions(sts, opts.Options)
	return sts
}

// NewService creates a new Service for each Thanos Compact shard.
// The Service name will be the same as the Shard name if provided.
// The Service name will be the same as the Options.Name if Shard name is not provided.
func NewService(opts Options) *corev1.Service {
	selectorLabels := opts.GetSelectorLabels()
	objectMetaLabels := manifests.MergeLabels(opts.Labels, selectorLabels)
	svc := newService(opts, selectorLabels, objectMetaLabels)
	if opts.Additional.ServicePorts != nil {
		svc.Spec.Ports = append(svc.Spec.Ports, opts.Additional.ServicePorts...)
	}

	return svc
}

func newService(opts Options, selectorLabels, objectMetaLabels map[string]string) *corev1.Service {
	servicePorts := []corev1.ServicePort{
		{
			Name:       HTTPPortName,
			Port:       HTTPPort,
			TargetPort: intstr.FromInt32(HTTPPort),
		},
	}

	svc := &corev1.Service{
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
			Selector: selectorLabels,
			Ports:    servicePorts,
		},
	}
	return svc
}

func compactorArgsFrom(opts Options) []string {
	args := []string{"compact"}
	args = append(args, opts.ToFlags()...)
	args = append(args,
		"--wait",
		fmt.Sprintf("--http-address=0.0.0.0:%d", HTTPPort),
		fmt.Sprintf("--objstore.config=$(%s)", objectStoreEnvVarName),
		fmt.Sprintf("--data-dir=%s", dataVolumeMountPath),
	)

	if len(opts.RelabelConfigs) > 0 {
		args = append(args, opts.RelabelConfigs.ToFlags())
	}

	if opts.Min != nil {
		args = append(args, fmt.Sprintf("--min-time=%s", string(*opts.Min)))
	}
	if opts.Max != nil {
		args = append(args, fmt.Sprintf("--max-time=%s", string(*opts.Max)))
	}

	args = append(args, opts.RetentionOptions.toArgs()...)
	args = append(args, opts.BlockConfig.toArgs()...)
	args = append(args, opts.Compaction.toArgs()...)
	args = append(args, opts.Downsampling.toArgs()...)

	if opts.Additional.Args != nil {
		args = append(args, opts.Additional.Args...)
	}

	return manifests.PruneEmptyArgs(args)
}

// GetRequiredLabels returns a map of labels that can be used to look up ThanosCompact resources.
func GetRequiredLabels() map[string]string {
	return map[string]string{
		manifests.NameLabel:      Name,
		manifests.ComponentLabel: ComponentName,
		manifests.PartOfLabel:    manifests.DefaultPartOfLabel,
		manifests.ManagedByLabel: manifests.DefaultManagedByLabel,
	}
}

// GetSelectorLabels returns a map of labels that can be used to look up ThanosCompact resources.
func (opts Options) GetSelectorLabels() map[string]string {
	labels := GetRequiredLabels()
	labels[manifests.InstanceLabel] = manifests.ValidateAndSanitizeNameToValidLabelValue(opts.GetGeneratedResourceName())
	labels[manifests.OwnerLabel] = manifests.ValidateAndSanitizeNameToValidLabelValue(opts.getOwner())
	return labels
}

// GetLabels returns the ObjectMeta labels for Thanos Compact.
func GetLabels(opts Options) map[string]string {
	return manifests.MergeLabels(opts.Labels, opts.GetSelectorLabels())
}

// RetentionOptions for Thanos Compact
type RetentionOptions struct {
	// Raw is the retention configuration for the raw samples.
	Raw *manifests.Duration
	// FiveMinutes is the retention configuration for samples of resolution 1 (5 minutes).
	FiveMinutes *manifests.Duration
	// OneHour is the retention configuration for samples of resolution 2 (1 hour).
	OneHour *manifests.Duration
}

func (ro *RetentionOptions) toArgs() []string {
	var args []string
	if ro == nil {
		return args
	}

	if ro.Raw != nil {
		args = append(args, fmt.Sprintf("--retention.resolution-raw=%s", string(*ro.Raw)))
	}
	if ro.FiveMinutes != nil {
		args = append(args, fmt.Sprintf("--retention.resolution-5m=%s", string(*ro.FiveMinutes)))
	}
	if ro.OneHour != nil {
		args = append(args, fmt.Sprintf("--retention.resolution-1h=%s", string(*ro.OneHour)))
	}
	return args
}

// BlockConfigOptions for Thanos Compact
type BlockConfigOptions struct {
	// BlockDiscoveryStrategy is the discovery strategy to use for block discovery in storage.
	BlockDiscoveryStrategy *string
	// BlockFilesConcurrency is the number of goroutines to use when to use when
	BlockFilesConcurrency *int32
	// BlockMetaFetchConcurrency is the number of goroutines to use when fetching block metadata from object storage.
	BlockMetaFetchConcurrency *int32
	// BlockViewerGlobalSyncInterval for syncing the blocks between local and remote view for /global Block Viewer UI.
	BlockViewerGlobalSyncInterval *manifests.Duration
	// BlockViewerGlobalSyncTimeout is the maximum time for syncing the blocks between local and remote view for /global Block Viewer UI.
	BlockViewerGlobalSyncTimeout *manifests.Duration
}

func (bo *BlockConfigOptions) toArgs() []string {
	var args []string
	if bo == nil {
		return args
	}

	if bo.BlockDiscoveryStrategy != nil {
		args = append(args, fmt.Sprintf("--block-discovery-strategy=%s", *bo.BlockDiscoveryStrategy))
	}
	if bo.BlockFilesConcurrency != nil {
		args = append(args, fmt.Sprintf("--block-files-concurrency=%d", *bo.BlockFilesConcurrency))
	}
	if bo.BlockMetaFetchConcurrency != nil {
		args = append(args, fmt.Sprintf("--block-meta-fetch-concurrency=%d", *bo.BlockMetaFetchConcurrency))
	}
	if bo.BlockViewerGlobalSyncInterval != nil {
		args = append(args, fmt.Sprintf("--block-viewer.global.sync-block-interval=%s", string(*bo.BlockViewerGlobalSyncInterval)))
	}
	if bo.BlockViewerGlobalSyncTimeout != nil {
		args = append(args, fmt.Sprintf("--block-viewer.global.sync-block-timeout=%s", string(*bo.BlockViewerGlobalSyncTimeout)))
	}
	return args
}

type CompactionOptions struct {
	// CompactBlockFetchConcurrency is the number of goroutines to use when fetching blocks from object storage.
	CompactBlockFetchConcurrency *int32 `json:"compactBlockFetchConcurrency,omitempty"`
	// CompactCleanupInterval configures how often we should clean up partially uploaded blocks and blocks that are marked for deletion.
	CompactCleanupInterval *manifests.Duration `json:"compactCleanupInterval,omitempty"`
	// ConsistencyDelay is the minimum age of fresh (non-compacted) blocks before they are being processed.
	// Malformed blocks older than the maximum of consistency-delay and 48h0m0s will be removed.
	ConsistencyDelay *manifests.Duration `json:"blockConsistencyDelay,omitempty"`
}

func (co *CompactionOptions) toArgs() []string {
	var args []string
	if co == nil {
		return args
	}

	if co.CompactBlockFetchConcurrency != nil {
		args = append(args, fmt.Sprintf("--compact.block-fetch-concurrency=%d", *co.CompactBlockFetchConcurrency))
	}
	if co.CompactCleanupInterval != nil {
		args = append(args, fmt.Sprintf("--compact.cleanup-interval=%s", string(*co.CompactCleanupInterval)))
	}
	if co.ConsistencyDelay != nil {
		args = append(args, fmt.Sprintf("--consistency-delay=%s", string(*co.ConsistencyDelay)))
	}
	return args
}

type DownsamplingOptions struct {
	// Disable downsampling.
	Disable bool
	// Concurrency is the number of goroutines to use when downsampling blocks.
	Concurrency *int32
}

func (do *DownsamplingOptions) toArgs() []string {
	var args []string
	if do == nil {
		return args
	}

	if do.Disable {
		args = append(args, "--downsampling.disable")
	}
	if do.Concurrency != nil {
		args = append(args, fmt.Sprintf("--downsampling.concurrency=%d", *do.Concurrency))
	}
	return args
}
