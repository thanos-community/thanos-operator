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
	opts := commonToOpts(&in, in.Spec.Replicas, labels, in.GetAnnotations(), in.Spec.CommonFields, in.Spec.FeatureGates, in.Spec.Additional)
	var webOptions manifestquery.WebOptions
	if in.Spec.WebConfig != nil {
		webOptions = manifestquery.WebOptions{
			RoutePrefix:    manifests.OptionalToString(in.Spec.WebConfig.RoutePrefix),
			ExternalPrefix: manifests.OptionalToString(in.Spec.WebConfig.ExternalPrefix),
			PrefixHeader:   manifests.OptionalToString(in.Spec.WebConfig.PrefixHeader),
			DisableCORS:    in.Spec.WebConfig.DisableCORS != nil && *in.Spec.WebConfig.DisableCORS,
		}
	}
	var telemetryQuantiles manifestquery.TelemetryQuantiles
	if in.Spec.TelemetryQuantiles != nil {
		telemetryQuantiles = manifestquery.TelemetryQuantiles{
			Duration: in.Spec.TelemetryQuantiles.Duration,
			Samples:  in.Spec.TelemetryQuantiles.Samples,
			Series:   in.Spec.TelemetryQuantiles.Series,
		}
	}
	return manifestquery.Options{
		Options:            opts,
		ReplicaLabels:      in.Spec.ReplicaLabels,
		Timeout:            "15m",
		LookbackDelta:      "5m",
		MaxConcurrent:      20,
		WebOptions:         webOptions,
		TelemetryQuantiles: telemetryQuantiles,
		GRPCProxyStrategy:  in.Spec.GRPCProxyStrategy,
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
	opts := commonToOpts(&in, frontend.Replicas, labels, in.GetAnnotations(), frontend.CommonFields, in.Spec.FeatureGates, frontend.Additional)

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
	labels := manifests.MergeLabels(in.GetLabels(), in.Spec.Labels)
	opts := commonToOpts(&in, in.Spec.Replicas, labels, in.GetAnnotations(), in.Spec.CommonFields, in.Spec.FeatureGates, in.Spec.Additional)
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
	common := spec.CommonFields
	additional := in.Spec.Ingester.Additional
	secret := in.Spec.Ingester.DefaultObjectStorageConfig.ToSecretKeySelector()
	if spec.ObjectStorageConfig != nil {
		secret = spec.ObjectStorageConfig.ToSecretKeySelector()
	}

	opts := commonToOpts(&in, spec.Replicas, labels, in.GetAnnotations(), common, in.Spec.FeatureGates, additional)
	igops := manifestreceive.IngesterOptions{
		Options:        opts,
		ObjStoreSecret: secret,
		TSDBOpts: manifestreceive.TSDBOpts{
			Retention: string(spec.TSDBConfig.Retention),
		},
		AsyncForwardWorkerCount:  manifests.OptionalToString(spec.AsyncForwardWorkerCount),
		TooFarInFutureTimeWindow: manifests.Duration(manifests.OptionalToString(spec.TooFarInFutureTimeWindow)),
		StorageSize:              resource.MustParse(string(spec.StorageSize)),
		ExternalLabels:           spec.ExternalLabels,
	}

	if spec.TenancyConfig != nil {
		igops.TenancyOpts = manifestreceive.TenancyOpts{
			TenantHeader:           spec.TenancyConfig.TenantHeader,
			TenantCertificateField: manifests.OptionalToString(spec.TenancyConfig.TenantCertificateField),
			DefaultTenantID:        spec.TenancyConfig.DefaultTenantID,
			SplitTenantLabelName:   manifests.OptionalToString(spec.TenancyConfig.SplitTenantLabelName),
			TenantLabelName:        spec.TenancyConfig.TenantLabelName,
		}
	}

	if spec.StoreLimitsOptions != nil {
		igops.StoreLimitsOpts = manifests.StoreLimitsOpts{
			StoreLimitsRequestSamples: spec.StoreLimitsOptions.StoreLimitsRequestSamples,
			StoreLimitsRequestSeries:  spec.StoreLimitsOptions.StoreLimitsRequestSeries,
		}
	}

	return igops
}

func receiverV1Alpha1ToRouterOptions(in v1alpha1.ThanosReceive) manifestreceive.RouterOptions {
	router := in.Spec.Router
	labels := manifests.MergeLabels(in.GetLabels(), router.Labels)
	opts := commonToOpts(&in, router.Replicas, labels, in.GetAnnotations(), router.CommonFields, in.Spec.FeatureGates, router.Additional)

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
	opts := commonToOpts(&in, in.Spec.Replicas, labels, in.GetAnnotations(), in.Spec.CommonFields, in.Spec.FeatureGates, in.Spec.Additional)
	var indexHeaderOpts *manifestsstore.IndexHeaderOptions
	if in.Spec.IndexHeaderConfig != nil {
		indexHeaderOpts = &manifestsstore.IndexHeaderOptions{
			EnableLazyReader:      in.Spec.IndexHeaderConfig.EnableLazyReader != nil && *in.Spec.IndexHeaderConfig.EnableLazyReader,
			LazyReaderIdleTimeout: manifests.Duration(manifests.OptionalToString(in.Spec.IndexHeaderConfig.LazyReaderIdleTimeout)),
			LazyDownloadStrategy:  manifests.OptionalToString(in.Spec.IndexHeaderConfig.LazyDownloadStrategy),
		}
	}
	var blockConfigOpts *manifestsstore.BlockConfigOptions
	if in.Spec.BlockConfig != nil {
		blockConfigOpts = &manifestsstore.BlockConfigOptions{
			BlockDiscoveryStrategy:    ptr.To(string(in.Spec.BlockConfig.BlockDiscoveryStrategy)),
			BlockMetaFetchConcurrency: in.Spec.BlockConfig.BlockMetaFetchConcurrency,
		}
	}
	sops := manifestsstore.Options{
		ObjStoreSecret:           in.Spec.ObjectStorageConfig.ToSecretKeySelector(),
		IndexCacheConfig:         toManifestCacheConfig(in.Spec.IndexCacheConfig),
		CachingBucketConfig:      toManifestCacheConfig(in.Spec.CachingBucketConfig),
		Min:                      manifests.Duration(manifests.OptionalToString(in.Spec.MinTime)),
		Max:                      manifests.Duration(manifests.OptionalToString(in.Spec.MaxTime)),
		IgnoreDeletionMarksDelay: manifests.Duration(in.Spec.IgnoreDeletionMarksDelay),
		IndexHeaderOptions:       indexHeaderOpts,
		BlockConfigOptions:       blockConfigOpts,
		StorageSize:              resource.MustParse(string(in.Spec.StorageSize)),
		Options:                  opts,
	}

	if in.Spec.StoreLimitsOptions != nil {
		sops.StoreLimitsOpts = manifests.StoreLimitsOpts{
			StoreLimitsRequestSamples: in.Spec.StoreLimitsOptions.StoreLimitsRequestSamples,
			StoreLimitsRequestSeries:  in.Spec.StoreLimitsOptions.StoreLimitsRequestSeries,
		}
	}

	return sops
}

func compactV1Alpha1ToOptions(in v1alpha1.ThanosCompact) manifestscompact.Options {
	labels := manifests.MergeLabels(in.GetLabels(), in.Spec.Labels)
	opts := commonToOpts(&in, 1, labels, in.GetAnnotations(), in.Spec.CommonFields, in.Spec.FeatureGates, in.Spec.Additional)

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
			CompactConcurrency:           in.Spec.CompactConfig.CompactConcurrency,
			CompactCleanupInterval:       ptr.To(manifests.Duration(*in.Spec.CompactConfig.CleanupInterval)),
			ConsistencyDelay:             ptr.To(manifests.Duration(*in.Spec.CompactConfig.ConsistencyDelay)),
			CompactBlockFetchConcurrency: in.Spec.CompactConfig.BlockFetchConcurrency,
		}
	}
	blockDiscovery := func() *manifestscompact.BlockConfigOptions {
		if in.Spec.BlockConfig == nil {
			return nil
		}
		opts := &manifestscompact.BlockConfigOptions{
			BlockDiscoveryStrategy:    ptr.To(string(in.Spec.BlockConfig.BlockDiscoveryStrategy)),
			BlockFilesConcurrency:     in.Spec.BlockConfig.BlockFilesConcurrency,
			BlockMetaFetchConcurrency: in.Spec.BlockConfig.BlockMetaFetchConcurrency,
		}

		if in.Spec.BlockViewerGlobalSync != nil {
			opts.BlockViewerGlobalSyncInterval = ptr.To(manifests.Duration(*in.Spec.BlockViewerGlobalSync.BlockViewerGlobalSyncInterval))
			opts.BlockViewerGlobalSyncTimeout = ptr.To(manifests.Duration(*in.Spec.BlockViewerGlobalSync.BlockViewerGlobalSyncTimeout))
		}
		return opts
	}
	debugConfig := func() *manifestscompact.DebugConfigOptions {
		if in.Spec.DebugConfig == nil {
			return nil
		}
		return &manifestscompact.DebugConfigOptions{
			AcceptMalformedIndex: in.Spec.DebugConfig.AcceptMalformedIndex != nil && *in.Spec.DebugConfig.AcceptMalformedIndex,
			MaxCompactionLevel:   *in.Spec.DebugConfig.MaxCompactionLevel,
			HaltOnError:          in.Spec.DebugConfig.HaltOnError != nil && *in.Spec.DebugConfig.HaltOnError,
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
		DebugConfig:    debugConfig(),
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
	common v1alpha1.CommonFields,
	featureGates *v1alpha1.FeatureGates,
	additional v1alpha1.Additional) manifests.Options {

	return manifests.Options{
		Owner:                owner.GetName(),
		Namespace:            owner.GetNamespace(),
		Replicas:             replicas,
		Labels:               labels,
		Annotations:          annotations,
		Image:                common.Image,
		Version:              common.Version,
		ResourceRequirements: common.ResourceRequirements,
		LogLevel:             common.LogLevel,
		LogFormat:            common.LogFormat,
		Additional:           additionalToOpts(additional),
		ServiceMonitorConfig: serviceMonitorConfigToOpts(featureGates, labels),
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

func serviceMonitorConfigToOpts(in *v1alpha1.FeatureGates, labels map[string]string) manifests.ServiceMonitorConfig {
	disable := manifests.ServiceMonitorConfig{Enabled: false}

	if in == nil {
		return disable
	}

	if in.ServiceMonitorConfig == nil {
		return disable
	}

	sm := in.ServiceMonitorConfig
	return manifests.ServiceMonitorConfig{
		Enabled: *sm.Enable,
		Labels:  manifests.MergeLabels(sm.Labels, labels),
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
