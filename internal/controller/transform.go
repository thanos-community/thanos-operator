package controller

import (
	"github.com/thanos-community/thanos-operator/api/v1alpha1"
	"github.com/thanos-community/thanos-operator/internal/pkg/manifests"
	manifestscompact "github.com/thanos-community/thanos-operator/internal/pkg/manifests/compact"
	manifestquery "github.com/thanos-community/thanos-operator/internal/pkg/manifests/query"
	manifestqueryfrontend "github.com/thanos-community/thanos-operator/internal/pkg/manifests/queryfrontend"
	manifestreceive "github.com/thanos-community/thanos-operator/internal/pkg/manifests/receive"
	manifestruler "github.com/thanos-community/thanos-operator/internal/pkg/manifests/ruler"
	manifestsstore "github.com/thanos-community/thanos-operator/internal/pkg/manifests/store"

	"k8s.io/apimachinery/pkg/api/resource"
	"k8s.io/utils/ptr"

	"sigs.k8s.io/controller-runtime/pkg/client"
)

func queryV1Alpha1ToOptions(in v1alpha1.ThanosQuery) manifestquery.Options {
	labels := manifests.MergeLabels(in.GetLabels(), in.Spec.Labels)
	opts := commonToOpts(&in, in.Spec.Replicas, labels, in.GetAnnotations(), in.Spec.CommonThanosFields, in.Spec.Additional)
	return manifestquery.Options{
		Options:       opts,
		ReplicaLabels: in.Spec.ReplicaLabels,
		Timeout:       "15m",
		LookbackDelta: "5m",
		MaxConcurrent: 20,
	}
}

// QueryNameFromParent returns the name of the Thanos Query component.
func QueryNameFromParent(resourceName string) string {
	return manifestquery.Options{Options: manifests.Options{Owner: resourceName}}.GetGeneratedResourceName()
}

// queryV1Alpha1ToQueryFrontEndOptions transforms a v1alpha1.ThanosQuery to a build Options
func queryV1Alpha1ToQueryFrontEndOptions(in v1alpha1.ThanosQuery) manifestqueryfrontend.Options {
	labels := manifests.MergeLabels(in.GetLabels(), in.Spec.Labels)

	frontend := in.Spec.QueryFrontend
	opts := commonToOpts(&in, frontend.Replicas, labels, in.GetAnnotations(), frontend.CommonThanosFields, frontend.Additional)

	return manifestqueryfrontend.Options{
		Options:                opts,
		QueryService:           QueryNameFromParent(in.GetName()),
		QueryPort:              manifestquery.HTTPPort,
		LogQueriesLongerThan:   manifests.Duration(manifests.OptionalToString(frontend.LogQueriesLongerThan)),
		CompressResponses:      frontend.CompressResponses,
		ResponseCacheConfig:    toManifestCacheConfig(frontend.QueryRangeResponseCacheConfig),
		RangeSplitInterval:     manifests.Duration(manifests.OptionalToString(frontend.QueryRangeSplitInterval)),
		LabelsSplitInterval:    manifests.Duration(manifests.OptionalToString(frontend.LabelsSplitInterval)),
		RangeMaxRetries:        frontend.QueryRangeMaxRetries,
		LabelsMaxRetries:       frontend.LabelsMaxRetries,
		LabelsDefaultTimeRange: manifests.Duration(manifests.OptionalToString(frontend.LabelsDefaultTimeRange)),
	}
}

// QueryFrontendNameFromParent returns the name of the Thanos Query Frontend component.
func QueryFrontendNameFromParent(resourceName string) string {
	return manifestqueryfrontend.Options{Options: manifests.Options{Owner: resourceName}}.GetGeneratedResourceName()
}

func rulerV1Alpha1ToOptions(in v1alpha1.ThanosRuler) manifestruler.Options {
	labels := manifests.MergeLabels(in.GetLabels(), nil)
	opts := commonToOpts(&in, in.Spec.Replicas, labels, in.GetAnnotations(), in.Spec.CommonThanosFields, in.Spec.Additional)
	return manifestruler.Options{
		Options:            opts,
		ObjStoreSecret:     in.Spec.ObjectStorageConfig.ToSecretKeySelector(),
		Retention:          manifests.Duration(in.Spec.Retention),
		AlertmanagerURL:    in.Spec.AlertmanagerURL,
		ExternalLabels:     in.Spec.ExternalLabels,
		AlertLabelDrop:     in.Spec.AlertLabelDrop,
		StorageSize:        resource.MustParse(in.Spec.StorageSize),
		EvaluationInterval: manifests.Duration(in.Spec.EvaluationInterval),
	}
}

// RulerNameFromParent returns the name of the Thanos Ruler component.
func RulerNameFromParent(resourceName string) string {
	return manifestruler.Options{Options: manifests.Options{Owner: resourceName}}.GetGeneratedResourceName()
}

func receiverV1Alpha1ToIngesterOptions(in v1alpha1.ThanosReceive, spec v1alpha1.IngesterHashringSpec) manifestreceive.IngesterOptions {
	labels := manifests.MergeLabels(in.GetLabels(), spec.Labels)
	common := spec.CommonThanosFields
	additional := in.Spec.Ingester.Additional
	secret := in.Spec.Ingester.DefaultObjectStorageConfig.ToSecretKeySelector()
	if spec.ObjectStorageConfig != nil {
		secret = spec.ObjectStorageConfig.ToSecretKeySelector()
	}

	opts := commonToOpts(&in, spec.Replicas, labels, in.GetAnnotations(), common, additional)
	return manifestreceive.IngesterOptions{
		Options:        opts,
		ObjStoreSecret: secret,
		TSDBOpts: manifestreceive.TSDBOpts{
			Retention: string(spec.TSDBConfig.Retention),
		},
		StorageSize:    resource.MustParse(string(spec.StorageSize)),
		ExternalLabels: spec.ExternalLabels,
	}
}

func receiverV1Alpha1ToRouterOptions(in v1alpha1.ThanosReceive) manifestreceive.RouterOptions {
	router := in.Spec.Router
	labels := manifests.MergeLabels(in.GetLabels(), router.Labels)
	opts := commonToOpts(&in, router.Replicas, labels, in.GetAnnotations(), router.CommonThanosFields, router.Additional)

	return manifestreceive.RouterOptions{
		Options:           opts,
		ReplicationFactor: router.ReplicationFactor,
		ExternalLabels:    router.ExternalLabels,
	}
}

// ReceiveIngesterNameFromParent returns the name of the Thanos Receive Ingester component.
func ReceiveIngesterNameFromParent(resourceName, hashringName string) string {
	return manifestreceive.IngesterOptions{Options: manifests.Options{Owner: resourceName}, HashringName: hashringName}.GetGeneratedResourceName()
}

// ReceiveRouterNameFromParent returns the name of the Thanos Receive Router component.
func ReceiveRouterNameFromParent(resourceName string) string {
	return manifestreceive.RouterOptions{Options: manifests.Options{Owner: resourceName}}.GetGeneratedResourceName()
}

func storeV1Alpha1ToOptions(in v1alpha1.ThanosStore) manifestsstore.Options {
	labels := manifests.MergeLabels(in.GetLabels(), in.Spec.Labels)
	opts := commonToOpts(&in, in.Spec.ShardingStrategy.ShardReplicas, labels, in.GetAnnotations(), in.Spec.CommonThanosFields, in.Spec.Additional)
	return manifestsstore.Options{
		ObjStoreSecret:           in.Spec.ObjectStorageConfig.ToSecretKeySelector(),
		IndexCacheConfig:         toManifestCacheConfig(in.Spec.IndexCacheConfig),
		CachingBucketConfig:      toManifestCacheConfig(in.Spec.CachingBucketConfig),
		Min:                      manifests.Duration(manifests.OptionalToString(in.Spec.MinTime)),
		Max:                      manifests.Duration(manifests.OptionalToString(in.Spec.MaxTime)),
		IgnoreDeletionMarksDelay: manifests.Duration(in.Spec.IgnoreDeletionMarksDelay),
		StorageSize:              resource.MustParse(string(in.Spec.StorageSize)),
		Options:                  opts,
	}
}

func compactV1Alpha1ToOptions(in v1alpha1.ThanosCompact) manifestscompact.Options {
	labels := manifests.MergeLabels(in.GetLabels(), in.Spec.Labels)
	opts := commonToOpts(&in, 1, labels, in.GetAnnotations(), in.Spec.CommonThanosFields, in.Spec.Additional)

	downsamplingConfig := func() *manifestscompact.DownsamplingOptions {
		if in.Spec.DownsamplingConfig == nil {
			return nil
		}

		disable := in.Spec.DownsamplingConfig.Disable != nil && *in.Spec.DownsamplingConfig.Disable

		return &manifestscompact.DownsamplingOptions{
			Disable:     disable,
			Concurrency: in.Spec.DownsamplingConfig.Concurrency,
		}
	}

	compaction := func() *manifestscompact.CompactionOptions {
		if in.Spec.CompactConfig == nil {
			return nil
		}
		return &manifestscompact.CompactionOptions{
			CompactCleanupInterval:       ptr.To(manifests.Duration(*in.Spec.CompactConfig.CleanupInterval)),
			ConsistencyDelay:             ptr.To(manifests.Duration(*in.Spec.CompactConfig.ConsistencyDelay)),
			CompactBlockFetchConcurrency: in.Spec.CompactConfig.BlockFetchConcurrency,
		}
	}
	blockDiscovery := func() *manifestscompact.BlockConfigOptions {
		if in.Spec.BlockConfig == nil {
			return nil
		}
		return &manifestscompact.BlockConfigOptions{
			BlockDiscoveryStrategy:        ptr.To(string(in.Spec.BlockConfig.BlockDiscoveryStrategy)),
			BlockFilesConcurrency:         in.Spec.BlockConfig.BlockFilesConcurrency,
			BlockMetaFetchConcurrency:     in.Spec.BlockConfig.BlockMetaFetchConcurrency,
			BlockViewerGlobalSyncInterval: ptr.To(manifests.Duration(*in.Spec.BlockConfig.BlockViewerGlobalSyncInterval)),
			BlockViewerGlobalSyncTimeout:  ptr.To(manifests.Duration(*in.Spec.BlockConfig.BlockViewerGlobalSyncTimeout)),
		}
	}

	return manifestscompact.Options{
		Options: opts,
		RetentionOptions: &manifestscompact.RetentionOptions{
			Raw:         ptr.To(manifests.Duration(in.Spec.RetentionConfig.Raw)),
			FiveMinutes: ptr.To(manifests.Duration(in.Spec.RetentionConfig.FiveMinutes)),
			OneHour:     ptr.To(manifests.Duration(in.Spec.RetentionConfig.OneHour)),
		},
		BlockConfig:    blockDiscovery(),
		Compaction:     compaction(),
		Downsampling:   downsamplingConfig(),
		StorageSize:    in.Spec.StorageSize.ToResourceQuantity(),
		ObjStoreSecret: in.Spec.ObjectStorageConfig.ToSecretKeySelector(),
	}
}

// CompactNameFromParent returns the name of the Thanos Compact component.
func CompactNameFromParent(resourceName string) string {
	return manifestscompact.Options{Options: manifests.Options{Owner: resourceName}}.GetGeneratedResourceName()
}

// StoreNameFromParent returns the name of the Thanos Store component.
func StoreNameFromParent(resourceName string, index *int32) string {
	return manifestsstore.Options{Options: manifests.Options{Owner: resourceName}, ShardIndex: index}.GetGeneratedResourceName()
}

func commonToOpts(
	owner client.Object,
	replicas int32,
	labels map[string]string,
	annotations map[string]string,
	common v1alpha1.CommonThanosFields,
	additional v1alpha1.Additional) manifests.Options {

	return manifests.Options{
		Owner:                owner.GetName(),
		Namespace:            owner.GetNamespace(),
		Replicas:             replicas,
		Labels:               labels,
		Annotations:          annotations,
		Image:                common.Image,
		ResourceRequirements: common.ResourceRequirements,
		LogLevel:             common.LogLevel,
		LogFormat:            common.LogFormat,
		Additional:           additionalToOpts(additional),
		ServiceMonitorConfig: serviceMonitorConfigToOpts(common.ServiceMonitorConfig, labels),
		PodDisruptionConfig:  getPodDisruptionBudget(replicas),
	}
}

// getPodDisruptionBudget returns a PodDisruptionBudgetOptions if replicas is greater than 1 or nil otherwise.
func getPodDisruptionBudget(replicas int32) *manifests.PodDisruptionBudgetOptions {
	if replicas > 1 {
		return &manifests.PodDisruptionBudgetOptions{}
	}
	return nil
}

func additionalToOpts(in v1alpha1.Additional) manifests.Additional {
	return manifests.Additional{
		Args:         in.Args,
		Containers:   in.Containers,
		Volumes:      in.Volumes,
		VolumeMounts: in.VolumeMounts,
		Ports:        in.Ports,
		Env:          in.Env,
		ServicePorts: in.ServicePorts,
	}
}

func serviceMonitorConfigToOpts(in *v1alpha1.ServiceMonitorConfig, labels map[string]string) manifests.ServiceMonitorConfig {
	if in == nil {
		return manifests.ServiceMonitorConfig{Enabled: false}
	}

	if in.Labels == nil {
		in.Labels = labels
	}
	return manifests.ServiceMonitorConfig{
		Enabled: *in.Enable,
		Labels:  in.Labels,
	}
}

func toManifestCacheConfig(config *v1alpha1.CacheConfig) manifests.CacheConfig {
	if config == nil {
		return manifests.CacheConfig{
			InMemoryCacheConfig: nil,
			FromSecret:          nil,
		}
	}

	// prefer the external cache config and return it if it is set
	if config.ExternalCacheConfig != nil {
		return manifests.CacheConfig{
			FromSecret: config.ExternalCacheConfig,
		}
	}

	// if there is no external cache config, try to build the in-memory cache config
	var toInMemoryCacheConfig *manifests.InMemoryCacheConfig
	if config.InMemoryCacheConfig != nil {
		var maxSize, maxItemSize string
		if config.InMemoryCacheConfig.MaxSize != nil {
			maxSize = string(*config.InMemoryCacheConfig.MaxSize)
		}
		if config.InMemoryCacheConfig.MaxItemSize != nil {
			maxItemSize = string(*config.InMemoryCacheConfig.MaxItemSize)
		}
		if maxSize != "" || maxItemSize != "" {
			toInMemoryCacheConfig = &manifests.InMemoryCacheConfig{
				MaxSize:     maxSize,
				MaxItemSize: maxItemSize,
			}
		}
	}
	return manifests.CacheConfig{
		InMemoryCacheConfig: toInMemoryCacheConfig,
		FromSecret:          nil,
	}
}
