package controller

import (
	"fmt"

	"k8s.io/utils/ptr"

	"github.com/thanos-community/thanos-operator/api/v1alpha1"
	"github.com/thanos-community/thanos-operator/internal/pkg/manifests"
	manifestscompact "github.com/thanos-community/thanos-operator/internal/pkg/manifests/compact"
	manifestquery "github.com/thanos-community/thanos-operator/internal/pkg/manifests/query"
	manifestqueryfrontend "github.com/thanos-community/thanos-operator/internal/pkg/manifests/queryfrontend"
	manifestreceive "github.com/thanos-community/thanos-operator/internal/pkg/manifests/receive"
	manifestruler "github.com/thanos-community/thanos-operator/internal/pkg/manifests/ruler"
	manifestsstore "github.com/thanos-community/thanos-operator/internal/pkg/manifests/store"

	"k8s.io/apimachinery/pkg/api/resource"
)

func queryV1Alpha1ToOptions(in v1alpha1.ThanosQuery) manifestquery.Options {
	labels := manifests.MergeLabels(in.GetLabels(), in.Spec.Labels)
	name := QueryNameFromParent(in.GetName())
	opts := commonToOpts(name, in.GetNamespace(), in.Spec.Replicas, labels, in.Spec.CommonThanosFields, in.Spec.Additional)
	return manifestquery.Options{
		Options:       opts,
		ReplicaLabels: in.Spec.QuerierReplicaLabels,
		Timeout:       "15m",
		LookbackDelta: "5m",
		MaxConcurrent: 20,
	}
}

// QueryNameFromParent returns the name of the Thanos Query component.
func QueryNameFromParent(resourceName string) string {
	return fmt.Sprintf("thanos-query-%s", resourceName)
}

// queryV1Alpha1ToQueryFrontEndOptions transforms a v1alpha1.ThanosQuery to a build Options
func queryV1Alpha1ToQueryFrontEndOptions(in v1alpha1.ThanosQuery) manifestqueryfrontend.Options {
	labels := manifests.MergeLabels(in.GetLabels(), in.Spec.Labels)
	name := QueryFrontendNameFromParent(in.GetName())
	frontend := in.Spec.QueryFrontend
	opts := commonToOpts(name, in.GetNamespace(), frontend.Replicas, labels, frontend.CommonThanosFields, frontend.Additional)

	return manifestqueryfrontend.Options{
		Options:                opts,
		QueryService:           QueryNameFromParent(in.GetName()),
		QueryPort:              manifestquery.HTTPPort,
		LogQueriesLongerThan:   manifests.Duration(manifests.OptionalToString(frontend.LogQueriesLongerThan)),
		CompressResponses:      frontend.CompressResponses,
		ResponseCacheConfig:    frontend.QueryRangeResponseCacheConfig,
		RangeSplitInterval:     manifests.Duration(manifests.OptionalToString(frontend.QueryRangeSplitInterval)),
		LabelsSplitInterval:    manifests.Duration(manifests.OptionalToString(frontend.LabelsSplitInterval)),
		RangeMaxRetries:        frontend.QueryRangeMaxRetries,
		LabelsMaxRetries:       frontend.LabelsMaxRetries,
		LabelsDefaultTimeRange: manifests.Duration(manifests.OptionalToString(frontend.LabelsDefaultTimeRange)),
	}
}

// QueryFrontendNameFromParent returns the name of the Thanos Query Frontend component.
func QueryFrontendNameFromParent(resourceName string) string {
	return fmt.Sprintf("thanos-query-frontend-%s", resourceName)
}

func rulerV1Alpha1ToOptions(in v1alpha1.ThanosRuler) manifestruler.Options {
	labels := manifests.MergeLabels(in.GetLabels(), nil)
	name := RulerNameFromParent(in.GetName())
	opts := commonToOpts(name, in.GetNamespace(), in.Spec.Replicas, labels, in.Spec.CommonThanosFields, in.Spec.Additional)
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
	return fmt.Sprintf("thanos-ruler-%s", resourceName)
}

func receiverV1Alpha1ToIngesterOptions(in v1alpha1.ThanosReceive, spec v1alpha1.IngesterHashringSpec) manifestreceive.IngesterOptions {
	labels := manifests.MergeLabels(in.GetLabels(), spec.Labels)
	name := ReceiveIngesterNameFromParent(in.GetName(), spec.Name)
	common := spec.CommonThanosFields
	additional := in.Spec.Ingester.Additional
	secret := in.Spec.Ingester.DefaultObjectStorageConfig.ToSecretKeySelector()
	if spec.ObjectStorageConfig != nil {
		secret = spec.ObjectStorageConfig.ToSecretKeySelector()
	}

	opts := commonToOpts(name, in.GetNamespace(), spec.Replicas, labels, common, additional)
	return manifestreceive.IngesterOptions{
		Options:        opts,
		ObjStoreSecret: secret,
		TSDBOpts: manifestreceive.TSDBOpts{
			Retention: string(spec.TSDBConfig.Retention),
		},
		StorageSize:    resource.MustParse(string(spec.StorageSize)),
		Instance:       in.GetName(),
		ExternalLabels: spec.ExternalLabels,
	}
}

func receiverV1Alpha1ToRouterOptions(in v1alpha1.ThanosReceive) manifestreceive.RouterOptions {
	router := in.Spec.Router
	labels := manifests.MergeLabels(in.GetLabels(), router.Labels)
	name := ReceiveRouterNameFromParent(in.GetName())

	opts := commonToOpts(name, in.GetNamespace(), router.Replicas, labels, router.CommonThanosFields, router.Additional)

	return manifestreceive.RouterOptions{
		Options:           opts,
		ReplicationFactor: router.ReplicationFactor,
		ExternalLabels:    router.ExternalLabels,
	}
}

// ReceiveIngesterNameFromParent returns the name of the Thanos Receive Ingester component.
func ReceiveIngesterNameFromParent(resourceName, hashringName string) string {
	return fmt.Sprintf("thanos-receive-hashring-%s-%s", resourceName, hashringName)
}

// ReceiveRouterNameFromParent returns the name of the Thanos Receive Router component.
func ReceiveRouterNameFromParent(resourceName string) string {
	return fmt.Sprintf("thanos-receive-router-%s", resourceName)
}

func storeV1Alpha1ToOptions(in v1alpha1.ThanosStore) manifestsstore.Options {
	labels := manifests.MergeLabels(in.GetLabels(), in.Spec.Labels)
	name := StoreNameFromParent(in.GetName())
	opts := commonToOpts(name, in.GetNamespace(), in.Spec.ShardingStrategy.ShardReplicas, labels, in.Spec.CommonThanosFields, in.Spec.Additional)
	return manifestsstore.Options{
		ObjStoreSecret:           in.Spec.ObjectStorageConfig.ToSecretKeySelector(),
		IndexCacheConfig:         in.Spec.IndexCacheConfig,
		CachingBucketConfig:      in.Spec.CachingBucketConfig,
		Min:                      manifests.Duration(manifests.OptionalToString(in.Spec.MinTime)),
		Max:                      manifests.Duration(manifests.OptionalToString(in.Spec.MaxTime)),
		IgnoreDeletionMarksDelay: manifests.Duration(in.Spec.IgnoreDeletionMarksDelay),
		StorageSize:              resource.MustParse(string(in.Spec.StorageSize)),
		Options:                  opts,
	}
}

func compactV1Alpha1ToOptions(in v1alpha1.ThanosCompact) manifestscompact.Options {
	labels := manifests.MergeLabels(in.GetLabels(), in.Spec.Labels)
	name := CompactNameFromParent(in.GetName())
	opts := commonToOpts(name, in.GetNamespace(), 1, labels, in.Spec.CommonThanosFields, in.Spec.Additional)

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
	return fmt.Sprintf("thanos-compact-%s", resourceName)
}

// CompactShardName returns the name of the Thanos Compact shard.
func CompactShardName(resourceName, shardName string, shardIndex int) string {
	return fmt.Sprintf("%s-%s-%d", CompactNameFromParent(resourceName), shardName, shardIndex)
}

// StoreNameFromParent returns the name of the Thanos Store component.
func StoreNameFromParent(resourceName string) string {
	return fmt.Sprintf("thanos-store-%s", resourceName)
}

// StoreShardName returns the name of the Thanos Store shard.
func StoreShardName(resourceName string, shard int32) string {
	return fmt.Sprintf("%s-shard-%d", StoreNameFromParent(resourceName), shard)
}

func commonToOpts(
	name,
	namespace string,
	replicas int32,
	labels map[string]string,
	common v1alpha1.CommonThanosFields,
	additional v1alpha1.Additional) manifests.Options {
	return manifests.Options{
		Name:                 name,
		Namespace:            namespace,
		Replicas:             replicas,
		Labels:               labels,
		Image:                common.Image,
		ResourceRequirements: common.ResourceRequirements,
		LogLevel:             common.LogLevel,
		LogFormat:            common.LogFormat,
		Additional:           additionalToOpts(additional),
		ServiceMonitorConfig: serviceMonitorConfigToOpts(common.ServiceMonitorConfig, namespace, labels),
	}
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

func serviceMonitorConfigToOpts(in *v1alpha1.ServiceMonitorConfig, namespace string, labels map[string]string) manifests.ServiceMonitorConfig {
	if in == nil {
		return manifests.ServiceMonitorConfig{
			Enabled:   true,
			Namespace: namespace,
			Labels:    labels,
		}
	}

	if in != nil && in.Enabled == nil {
		in.Enabled = ptr.To(true)
	}
	if in != nil && in.Namespace == nil {
		in.Namespace = &namespace
	}
	if in != nil && in.Labels == nil {
		in.Labels = labels
	}
	return manifests.ServiceMonitorConfig{
		Enabled:   *in.Enabled,
		Labels:    in.Labels,
		Namespace: *in.Namespace,
	}
}
