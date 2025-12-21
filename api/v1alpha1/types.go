package v1alpha1

import (
	corev1 "k8s.io/api/core/v1"
	"k8s.io/apimachinery/pkg/api/resource"
	"k8s.io/utils/ptr"
)

// Duration is a valid time duration that can be parsed by Prometheus model.ParseDuration() function.
// Supported units: y, w, d, h, m, s, ms
// Examples: `30s`, `1m`, `1h20m15s`, `15d`
// +kubebuilder:validation:Pattern="^(-?(0|(([0-9]+)y)?(([0-9]+)w)?(([0-9]+)d)?(([0-9]+)h)?(([0-9]+)m)?(([0-9]+)s)?(([0-9]+)ms)?)|([0-9]{4}-[0-9]{2}-[0-9]{2}T[0-9]{2}:[0-9]{2}:[0-9]{2}(\\.[0-9]+)?(Z|[+-][0-9]{2}:[0-9]{2})))$"
type Duration string

// ObjectStorageConfig is the secret that contains the object storage configuration.
// The secret needs to be in the same namespace as the ReceiveHashring object.
// See https://thanos.io/tip/thanos/storage.md/#supported-clients for relevant documentation.
type ObjectStorageConfig corev1.SecretKeySelector

// CacheConfig is the configuration for the cache.
// If both InMemoryCacheConfig and ExternalCacheConfig are specified, the operator will prefer the ExternalCacheConfig.
// +kubebuilder:validation:Optional
type CacheConfig struct {
	// InMemoryCacheConfig is the configuration for the in-memory cache.
	// +kubebuilder:validation:Optional
	InMemoryCacheConfig *InMemoryCacheConfig `json:"inMemoryCacheConfig,omitempty"`
	// ExternalCacheConfig is the configuration for the external cache.
	// +kubebuilder:validation:Optional
	ExternalCacheConfig *corev1.SecretKeySelector `json:"externalCacheConfig,omitempty"`
}

// InMemoryCacheConfig is the configuration for the in-memory cache.
type InMemoryCacheConfig struct {
	MaxSize     *StorageSize `json:"maxSize,omitempty"`
	MaxItemSize *StorageSize `json:"maxItemSize,omitempty"`
}

// ExternalLabels are the labels to add to the metrics.
// POD_NAME and POD_NAMESPACE are available via the downward API.
// +kubebuilder:validation:MinProperties=1
// +kubebuilder:validation:Required
// https://thanos.io/tip/thanos/storage.md/#external-labels
type ExternalLabels map[string]string

// StorageConfiguration represents the configuration options for a PVC used by a Thanos component
type StorageConfiguration struct {
	// Size is the size of the PV storage to be used by a Thanos component.
	// +kubebuilder:validation:Required
	Size StorageSize `json:"size"`
	// StorageClass is the name of the storage class to be used. If specified,
	// it will use the default storage class.
	// +kubebuilder:validation:Optional
	StorageClass *string `json:"storageClass,omitempty"`
}

// StorageSize is the size of the PV storage to be used by a Thanos component.
// +kubebuilder:validation:Required
// +kubebuilder:validation:Pattern=`^([+-]?[0-9.]+)([eEinumkKMGTP]*[-+]?[0-9]*)$`
type StorageSize string

// TSDBConfig specifies configuration for any particular Thanos TSDB.
// NOTE: Some of these options will not exist for all components, in which case, even if specified can be ignored.
type TSDBConfig struct {
	// Retention is the duration for which a particular TSDB will retain data.
	// +kubebuilder:default="2h"
	// +kubebuilder:validation:Required
	Retention Duration `json:"retention,omitempty"`
}

// CommonFields are the options available to all Thanos components.
// These fields reflect runtime changes to managed StatefulSet and Deployment resources.
// +kubebuilder:validation:Optional
// +k8s:deepcopy-gen=true
type CommonFields struct {
	// Version of Thanos to be deployed. Could also be image tag in case of custom downstream image.
	// If not specified, the operator assumes the latest upstream version of
	// Thanos available at the time when the version of the operator was released.
	// +kubebuilder:validation:Optional
	Version *string `json:"version,omitempty"`
	// Base container image (without tags) to use for the Thanos components deployed via operator.
	// +kubebuilder:validation:Optional
	Image *string `json:"baseImage,omitempty"`
	// Image pull policy for the Thanos containers.
	// See https://kubernetes.io/docs/concepts/containers/images/#image-pull-policy for more details.
	// +kubebuilder:validation:Enum=Always;Never;IfNotPresent
	// +kubebuilder:default:=IfNotPresent
	// +kubebuilder:validation:Optional
	ImagePullPolicy *corev1.PullPolicy `json:"imagePullPolicy,omitempty"`
	// An optional list of references to Secrets in the same namespace
	// to use for pulling images from registries.
	// See http://kubernetes.io/docs/user-guide/images#specifying-imagepullsecrets-on-a-pod
	// +kubebuilder:validation:Optional
	ImagePullSecrets []corev1.LocalObjectReference `json:"imagePullSecrets,omitempty"`
	// ResourceRequirements for the Thanos component container.
	// +kubebuilder:validation:Optional
	ResourceRequirements *corev1.ResourceRequirements `json:"resourceRequirements,omitempty"`
	// Log level for Thanos.
	// +kubebuilder:validation:Enum=debug;info;warn;error
	// +kubebuilder:validation:Optional
	LogLevel *string `json:"logLevel,omitempty"`
	// Log format for Thanos.
	// +kubebuilder:validation:Enum=logfmt;json
	// +kubebuilder:default:=logfmt
	// +kubebuilder:validation:Optional
	LogFormat *string `json:"logFormat,omitempty"`
	// NodeSelector defines on which Nodes the workloads are scheduled.
	// +kubebuilder:validation:Optional
	NodeSelector map[string]string `json:"nodeSelector,omitempty"`
	// Affinity defines the workloads affinity scheduling rules if specified.
	// +kubebuilder:validation:Optional
	Affinity *corev1.Affinity `json:"affinity,omitempty"`
	// Tolerations defines the workloads tolerations if specified.
	// +kubebuilder:validation:Optional
	Tolerations []corev1.Toleration `json:"tolerations,omitempty"`
	// SecurityContext holds pod-level security attributes and common container settings.
	// This allows setting the FSGroup, RunAsUser, RunAsGroup, etc. for the pod.
	// If not specified, the operator will default to FSGroup=1001.
	// +kubebuilder:validation:Optional
	SecurityContext *corev1.PodSecurityContext `json:"securityContext,omitempty"`
	// PodDisruptionBudgetConfig holds the configuration for the PodDisruptionBudget.
	// This allows enabling or disabling the creation of a PodDisruptionBudget for the Thanos component.
	// When enabled, a resource that has more than one replica will have a PodDisruptionBudget created
	// that sets maxUnavailable to 1.
	// +kubebuilder:validation:Optional
	// +kubebuilder:default={enable: true}
	PodDisruptionBudgetConfig *PodDisruptionBudgetConfig `json:"podDisruptionBudgetConfig,omitempty"`
}

// StatefulSetFields are the options available to all Thanos components.
// These fields reflect runtime changes to managed StatefulSet resources.
// +k8s:deepcopy-gen=true
type StatefulSetFields struct {
	// +kubebuilder:default:=OrderedReady
	// +kubebuilder:validation:Optional
	PodManagementPolicy *PodManagementPolicyType `json:"podManagementPolicy,omitempty"`
}

// PodManagementPolicyType defines the policy for creating pods under a stateful set.
// +kubebuilder:validation:Enum=OrderedReady;Parallel
type PodManagementPolicyType string

const (
	// OrderedReadyPodManagement will create pods in strictly increasing order on
	// scale up and strictly decreasing order on scale down, progressing only when
	// the previous pod is ready or terminated. At most one pod will be changed
	// at any time.
	OrderedReadyPodManagement PodManagementPolicyType = "OrderedReady"
	// ParallelPodManagement will create and delete pods as soon as the stateful set
	// replica count is changed, and will not wait for pods to be ready or complete
	// termination.
	ParallelPodManagement PodManagementPolicyType = "Parallel"
)

// Additional holds additional configuration for the Thanos components.
type Additional struct {
	// Additional arguments to pass to the Thanos components.
	// An additional argument will override an existing argument provided by the operator if there is a conflict.
	// +kubebuilder:validation:Optional
	Args []string `json:"additionalArgs,omitempty"`
	// Additional containers to add to the Thanos components.
	// +kubebuilder:validation:Optional
	Containers []corev1.Container `json:"additionalContainers,omitempty"`
	// Additional volumes to add to the Thanos components.
	// +kubebuilder:validation:Optional
	Volumes []corev1.Volume `json:"additionalVolumes,omitempty"`
	// Additional volume mounts to add to the Thanos component container in a Deployment or StatefulSet
	// controlled by the operator.
	// +kubebuilder:validation:Optional
	VolumeMounts []corev1.VolumeMount `json:"additionalVolumeMounts,omitempty"`
	// Additional ports to expose on the Thanos component container in a Deployment or StatefulSet
	// controlled by the operator.
	// +kubebuilder:validation:Optional
	Ports []corev1.ContainerPort `json:"additionalPorts,omitempty"`
	// Additional environment variables to add to the Thanos component container in a Deployment or StatefulSet
	// controlled by the operator.
	// +kubebuilder:validation:Optional
	Env []corev1.EnvVar `json:"additionalEnv,omitempty"`
	// AdditionalServicePorts are additional ports to expose on the Service for the Thanos component.
	// +kubebuilder:validation:Optional
	ServicePorts []corev1.ServicePort `json:"additionalServicePorts,omitempty"`
}

// FeatureGates holds the configuration for behaviour that is behind feature flags in the operator.
type FeatureGates struct {
	// PrometheusRuleEnabled enables the loading of PrometheusRules into the Thanos Ruler.
	// This setting is only applicable to ThanosRuler CRD, will be ignored for other components.
	// +kubebuilder:validation:Optional
	// +kubebuilder:default:=true
	PrometheusRuleEnabled *bool `json:"prometheusRuleEnabled,omitempty"`
}

// PodDisruptionBudgetConfig is the configuration for the PodDisruptionBudget.
// +kubebuilder:validation:Optional
type PodDisruptionBudgetConfig struct {
	// Enabled enables the creation of a PodDisruptionBudget for the Thanos component.
	// +kubebuilder:validation:Optional
	Enable *bool `json:"enable,omitempty"`
}

func (osc *ObjectStorageConfig) ToSecretKeySelector() corev1.SecretKeySelector {
	return corev1.SecretKeySelector{
		LocalObjectReference: corev1.LocalObjectReference{Name: osc.Name},
		Key:                  osc.Key,
		Optional:             ptr.To(false),
	}
}

// ToResourceQuantity converts a StorageSize to a resource.Quantity.
func (s StorageSize) ToResourceQuantity() resource.Quantity {
	return resource.MustParse(string(s))
}

// StoreLimitsOptions is the configuration for the store API limits options.
type StoreLimitsOptions struct {
	// StoreLimitsRequestSamples is the maximum samples allowed for a single StoreAPI Series request.
	// 0 means no limit.
	// +kubebuilder:default=0
	StoreLimitsRequestSamples uint64 `json:"storeLimitsRequestSamples,omitempty"`
	// StoreLimitsRequestSeries is the maximum series allowed for a single StoreAPI Series request.
	// 0 means no limit.
	// +kubebuilder:default=0
	StoreLimitsRequestSeries uint64 `json:"storeLimitsRequestSeries,omitempty"`
}

// BlockDiscoveryStrategy represents the strategy to use for block discovery.
type BlockDiscoveryStrategy string

const (
	// BlockDiscoveryStrategyConcurrent means stores will concurrently issue one call
	// per directory to discover active blocks storage.
	BlockDiscoveryStrategyConcurrent BlockDiscoveryStrategy = "concurrent"
	// BlockDiscoveryStrategyRecursive means stores iterate through all objects in storage
	// recursively traversing into each directory.
	// This avoids N+1 calls at the expense of having slower bucket iterations.
	BlockDiscoveryStrategyRecursive BlockDiscoveryStrategy = "recursive"
)

// BlockConfig defines settings for block handling.
type BlockConfig struct {
	// BlockDiscoveryStrategy is the discovery strategy to use for block discovery in storage.
	// +kubebuilder:default="concurrent"
	// +kubebuilder:validation:Enum=concurrent;recursive
	BlockDiscoveryStrategy BlockDiscoveryStrategy `json:"blockDiscoveryStrategy,omitempty"`
	// BlockFilesConcurrency is the number of goroutines to use when to use when
	// fetching/uploading block files from object storage.
	// Only used for Compactor, no-op for store gateway
	// +kubebuilder:default=1
	// +kubebuilder:validation:Optional
	BlockFilesConcurrency *int32 `json:"blockFilesConcurrency,omitempty"`
	// BlockMetaFetchConcurrency is the number of goroutines to use when fetching block metadata from object storage.
	// +kubebuilder:default=32
	// +kubebuilder:validation:Optional
	BlockMetaFetchConcurrency *int32 `json:"blockMetaFetchConcurrency,omitempty"`
}

// TimeRangeConfig configures the time range of data to serve.
type TimeRangeConfig struct {
	// Minimum time range to serve. Any data earlier than this lower time range will be ignored.
	// If not set, will be set as zero value, so most recent blocks will be served.
	// +kubebuilder:validation:Optional
	MinTime *Duration `json:"minTime,omitempty"`
	// Maximum time range to serve. Any data after this upper time range will be ignored.
	// If not set, will be set as max value, so all blocks will be served.
	// +kubebuilder:validation:Optional
	MaxTime *Duration `json:"maxTime,omitempty"`
}

type StatefulSetStatus struct {
	// Replicas is the number of replicas of the StatefulSet.
	Replicas int32 `json:"replicas,omitempty"`
	//  Total number of non-terminating pods targeted by StatefulSet that have the desired template spec..
	UpdatedReplicas int32 `json:"updatedReplicas,omitempty"`
	// Total number of available pods (ready for at least minReadySeconds) targeted by this StatefulSet.
	AvailableReplicas int32 `json:"availableReplicas,omitempty"`
	// ReadyReplicas is the number of pods created for this StatefulSet with a Ready Condition.
	ReadyReplicas int32 `json:"readyReplicas,omitempty"`
	// currentReplicas is the number of Pods created by the StatefulSet.
	CurrentReplicas int32 `json:"currentReplicas,omitempty"`
}

type DeploymentStatus struct {
	// Replicas is the number of replicas of the Deployment.
	Replicas int32 `json:"replicas"`
	// UpdatedReplicas is the number of Pods created by the Deployment.
	UpdatedReplicas int32 `json:"updatedReplicas"`
	// Total number of available pods (ready for at least minReadySeconds) targeted by this Deployment.
	AvailableReplicas int32 `json:"availableReplicas"`
	// UnavailableReplicas is the number of pods that are needed for Deployment to have 100% capacity.
	UnavailableReplicas int32 `json:"unavailableReplicas"`
	// ReadyReplicas is the number of pods created for this Deployment with a Ready Condition.
	ReadyReplicas int32 `json:"readyReplicas"`
}
