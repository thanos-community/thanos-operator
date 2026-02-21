package controller

import (
	"github.com/thanos-community/thanos-operator/api/v1alpha1"
	"github.com/thanos-community/thanos-operator/internal/pkg/featuregate"
	"github.com/thanos-community/thanos-operator/internal/pkg/manifests"
	manifestscompact "github.com/thanos-community/thanos-operator/internal/pkg/manifests/compact"
	manifestquery "github.com/thanos-community/thanos-operator/internal/pkg/manifests/query"
	manifestqueryfrontend "github.com/thanos-community/thanos-operator/internal/pkg/manifests/queryfrontend"
	manifestreceive "github.com/thanos-community/thanos-operator/internal/pkg/manifests/receive"
	manifestruler "github.com/thanos-community/thanos-operator/internal/pkg/manifests/ruler"
	manifestsstore "github.com/thanos-community/thanos-operator/internal/pkg/manifests/store"

	"k8s.io/utils/ptr"

	"sigs.k8s.io/controller-runtime/pkg/client"
)

func queryV1Alpha1ToOptions(in v1alpha1.ThanosQuery, featureGate featuregate.Config) manifestquery.Options {
	labels := manifests.MergeLabels(in.GetLabels(), in.Spec.Labels)
	opts := commonToOpts(&in, in.Spec.Replicas, labels, in.GetAnnotations(), in.Spec.CommonFields, nil, featureGate, in.Spec.Additional)
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
	opts := manifestquery.Options{Options: manifests.Options{Owner: resourceName}}
	if err := opts.Valid(); err != nil {
		panic("invalid query options")
	}
	return opts.GetGeneratedResourceName()
}

// queryV1Alpha1ToQueryFrontEndOptions transforms a v1alpha1.ThanosQuery to a build Options
func queryV1Alpha1ToQueryFrontEndOptions(in v1alpha1.ThanosQuery, featureGate featuregate.Config) manifestqueryfrontend.Options {
	// Parent labels + query spec labels + query-frontend specific labels
	// Query-frontend labels take precedence in case of conflicts
	frontend := in.Spec.QueryFrontend
	labels := manifests.MergeLabels(manifests.MergeLabels(in.GetLabels(), in.Spec.Labels), frontend.Labels)

	// Parent annotations + query-frontend specific annotations
	// Query-frontend annotations take precedence in case of conflicts
	annotations := manifests.MergeLabels(in.GetAnnotations(), frontend.Annotations)

	opts := commonToOpts(&in, frontend.Replicas, labels, annotations, frontend.CommonFields, nil, featureGate, frontend.Additional)

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
	opts := manifestqueryfrontend.Options{Options: manifests.Options{Owner: resourceName}}
	if err := opts.Valid(); err != nil {
		panic("invalid query frontend options")
	}
	return opts.GetGeneratedResourceName()
}

func rulerV1Alpha1ToOptions(in v1alpha1.ThanosRuler, featureGate featuregate.Config) manifestruler.Options {
	labels := manifests.MergeLabels(in.GetLabels(), in.Spec.Labels)
	opts := commonToOpts(&in, in.Spec.Replicas, labels, in.GetAnnotations(), in.Spec.CommonFields, &in.Spec.StatefulSetFields, featureGate, in.Spec.Additional)
	return manifestruler.Options{
		Options:         opts,
		ObjStoreSecret:  in.Spec.ObjectStorageConfig.ToSecretKeySelector(),
		Retention:       manifests.Duration(in.Spec.Retention),
		AlertmanagerURL: in.Spec.AlertmanagerURL,
		ExternalLabels:  in.Spec.ExternalLabels,
		AlertLabelDrop:  in.Spec.AlertLabelDrop,
		StorageConfig: manifests.StorageConfig{
			StorageSize:      in.Spec.StorageConfiguration.Size.ToResourceQuantity(),
			StorageClassName: in.Spec.StorageConfiguration.StorageClass,
		},
		EvaluationInterval: manifests.Duration(in.Spec.EvaluationInterval),
	}
}

// RulerNameFromParent returns the name of the Thanos Ruler component.
func RulerNameFromParent(resourceName string) string {
	opts := manifestruler.Options{Options: manifests.Options{Owner: resourceName}}
	if err := opts.Valid(); err != nil {
		panic("invalid ruler options")
	}
	return opts.GetGeneratedResourceName()
}

func receiverV1Alpha1ToIngesterOptions(in v1alpha1.ThanosReceive, spec v1alpha1.IngesterHashringSpec, featureGate featuregate.Config) manifestreceive.IngesterOptions {
	labels := manifests.MergeLabels(in.GetLabels(), spec.Labels)
	common := spec.CommonFields
	additional := in.Spec.Ingester.Additional
	secret := in.Spec.Ingester.DefaultObjectStorageConfig.ToSecretKeySelector()
	if spec.ObjectStorageConfig != nil {
		secret = spec.ObjectStorageConfig.ToSecretKeySelector()
	}

	opts := commonToOpts(&in, spec.Replicas, labels, in.GetAnnotations(), common, &in.Spec.StatefulSetFields, featureGate, additional)
	ingestOpts := manifestreceive.IngesterOptions{
		Options:        opts,
		ObjStoreSecret: secret,
		TSDBOpts: manifestreceive.TSDBOpts{
			Retention: string(spec.TSDBConfig.Retention),
		},
		AsyncForwardWorkerCount:  manifests.OptionalToString(spec.AsyncForwardWorkerCount),
		TooFarInFutureTimeWindow: manifests.Duration(manifests.OptionalToString(spec.TooFarInFutureTimeWindow)),
		StorageConfig: manifests.StorageConfig{
			StorageSize:      spec.StorageConfiguration.Size.ToResourceQuantity(),
			StorageClassName: spec.StorageConfiguration.StorageClass,
		},
		ExternalLabels: spec.ExternalLabels,
	}

	if in.Spec.Router.ReplicationProtocol != nil {
		ingestOpts.ReplicationProtocol = string(*in.Spec.Router.ReplicationProtocol)
	}

	if spec.GRPCCompression != nil {
		ingestOpts.GRPCCompression = string(*spec.GRPCCompression)
	}

	if spec.TenancyConfig != nil {
		ingestOpts.TenancyOpts = manifestreceive.TenancyOpts{
			TenantHeader:           spec.TenancyConfig.TenantHeader,
			TenantCertificateField: manifests.OptionalToString(spec.TenancyConfig.TenantCertificateField),
			DefaultTenantID:        spec.TenancyConfig.DefaultTenantID,
			SplitTenantLabelName:   manifests.OptionalToString(spec.TenancyConfig.SplitTenantLabelName),
			TenantLabelName:        spec.TenancyConfig.TenantLabelName,
		}
	}

	if spec.StoreLimitsOptions != nil {
		ingestOpts.StoreLimitsOpts = manifests.StoreLimitsOpts{
			StoreLimitsRequestSamples: spec.StoreLimitsOptions.StoreLimitsRequestSamples,
			StoreLimitsRequestSeries:  spec.StoreLimitsOptions.StoreLimitsRequestSeries,
		}
	}

	return ingestOpts
}

func receiverV1Alpha1ToRouterOptions(in v1alpha1.ThanosReceive, featureGate featuregate.Config) manifestreceive.RouterOptions {
	router := in.Spec.Router
	labels := manifests.MergeLabels(in.GetLabels(), router.Labels)
	opts := commonToOpts(&in, router.Replicas, labels, in.GetAnnotations(), router.CommonFields, &in.Spec.StatefulSetFields, featureGate, router.Additional)

	ropts := manifestreceive.RouterOptions{
		Options:           opts,
		ReplicationFactor: router.ReplicationFactor,
		ExternalLabels:    router.ExternalLabels,
	}

	if featureGate.KubeResourceSyncEnabled() {
		ropts.FeatureGateConfig = &manifestreceive.FeatureGateConfig{
			KubeResourceSyncEnabled: featureGate.KubeResourceSyncEnabled(),
			KubeResourceSyncImage:   featureGate.GetKubeResourceSyncImage(),
		}
	}

	if router.ReplicationProtocol != nil {
		ropts.ReplicationProtocol = string(*router.ReplicationProtocol)
	}

	return ropts
}

// ReceiveIngesterNameFromParent returns the name of the Thanos Receive Ingester component.
func ReceiveIngesterNameFromParent(resourceName, hashringName string) string {
	opts := manifestreceive.IngesterOptions{Options: manifests.Options{Owner: resourceName}, HashringName: hashringName}
	if err := opts.Valid(); err != nil {
		panic("invalid ingester options")
	}
	return opts.GetGeneratedResourceName()
}

// ReceiveRouterNameFromParent returns the name of the Thanos Receive Router component.
func ReceiveRouterNameFromParent(resourceName string) string {
	opts := manifestreceive.RouterOptions{Options: manifests.Options{Owner: resourceName}}
	if err := opts.Valid(); err != nil {
		panic("invalid router options")
	}
	return opts.GetGeneratedResourceName()
}

func storeV1Alpha1ToOptions(in v1alpha1.ThanosStore, featureGate featuregate.Config) manifestsstore.Options {
	labels := manifests.MergeLabels(in.GetLabels(), in.Spec.Labels)
	opts := commonToOpts(&in, in.Spec.Replicas, labels, in.GetAnnotations(), in.Spec.CommonFields, &in.Spec.StatefulSetFields, featureGate, in.Spec.Additional)
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
		IgnoreDeletionMarksDelay: manifests.Duration(in.Spec.IgnoreDeletionMarksDelay),
		IndexHeaderOptions:       indexHeaderOpts,
		BlockConfigOptions:       blockConfigOpts,
		StorageConfig: manifests.StorageConfig{
			StorageSize:      in.Spec.StorageConfiguration.Size.ToResourceQuantity(),
			StorageClassName: in.Spec.StorageConfiguration.StorageClass,
		},
		Options: opts,
	}

	if in.Spec.StoreLimitsOptions != nil {
		sops.StoreLimitsOpts = manifests.StoreLimitsOpts{
			StoreLimitsRequestSamples: in.Spec.StoreLimitsOptions.StoreLimitsRequestSamples,
			StoreLimitsRequestSeries:  in.Spec.StoreLimitsOptions.StoreLimitsRequestSeries,
		}
	}

	if in.Spec.TimeRangeConfig != nil {
		sops.Min = manifests.Duration(manifests.OptionalToString(in.Spec.TimeRangeConfig.MinTime))
		sops.Max = manifests.Duration(manifests.OptionalToString(in.Spec.TimeRangeConfig.MaxTime))
	}

	return sops
}

func compactV1Alpha1ToOptions(in v1alpha1.ThanosCompact, fg featuregate.Config) manifestscompact.Options {
	labels := manifests.MergeLabels(in.GetLabels(), in.Spec.Labels)
	opts := commonToOpts(&in, 1, labels, in.GetAnnotations(), in.Spec.CommonFields, &in.Spec.StatefulSetFields, fg, in.Spec.Additional)
	// we always set nil for compactor since it should run as single pod
	opts.PodDisruptionConfig = nil
	opts.PodManagementPolicy = string(ptr.Deref(in.Spec.PodManagementPolicy, ""))

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
		opts := &manifestscompact.CompactionOptions{
			CompactConcurrency:           in.Spec.CompactConfig.CompactConcurrency,
			CompactCleanupInterval:       ptr.To(manifests.Duration(*in.Spec.CompactConfig.CleanupInterval)),
			ConsistencyDelay:             ptr.To(manifests.Duration(*in.Spec.CompactConfig.ConsistencyDelay)),
			CompactBlockFetchConcurrency: in.Spec.CompactConfig.BlockFetchConcurrency,
		}
		if in.Spec.VerticalCompactionConfig != nil {
			opts.VerticalCompaction = &manifestscompact.VerticalCompactionOptions{
				ReplicaLabels:     in.Spec.VerticalCompactionConfig.ReplicaLabels,
				DeduplicationFunc: in.Spec.VerticalCompactionConfig.DeduplicationFunc,
			}
		}
		return opts
	}
	blockDiscovery := func() *manifestscompact.BlockConfigOptions {
		if in.Spec.BlockConfig == nil && in.Spec.BlockViewerGlobalSync == nil {
			return nil
		}
		opts := &manifestscompact.BlockConfigOptions{}

		if in.Spec.BlockConfig != nil {
			opts.BlockDiscoveryStrategy = ptr.To(string(in.Spec.BlockConfig.BlockDiscoveryStrategy))
			opts.BlockFilesConcurrency = in.Spec.BlockConfig.BlockFilesConcurrency
			opts.BlockMetaFetchConcurrency = in.Spec.BlockConfig.BlockMetaFetchConcurrency
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

		var mcl int32
		if in.Spec.DebugConfig.MaxCompactionLevel != nil {
			mcl = *in.Spec.DebugConfig.MaxCompactionLevel
		}
		return &manifestscompact.DebugConfigOptions{
			AcceptMalformedIndex: in.Spec.DebugConfig.AcceptMalformedIndex != nil && *in.Spec.DebugConfig.AcceptMalformedIndex,
			MaxCompactionLevel:   mcl,
			HaltOnError:          in.Spec.DebugConfig.HaltOnError != nil && *in.Spec.DebugConfig.HaltOnError,
		}
	}

	cops := manifestscompact.Options{
		Options: opts,
		RetentionOptions: &manifestscompact.RetentionOptions{
			Raw:         ptr.To(manifests.Duration(in.Spec.RetentionConfig.Raw)),
			FiveMinutes: ptr.To(manifests.Duration(in.Spec.RetentionConfig.FiveMinutes)),
			OneHour:     ptr.To(manifests.Duration(in.Spec.RetentionConfig.OneHour)),
		},
		BlockConfig:  blockDiscovery(),
		Compaction:   compaction(),
		Downsampling: downsamplingConfig(),
		DebugConfig:  debugConfig(),
		StorageConfig: manifests.StorageConfig{
			StorageSize:      in.Spec.StorageConfiguration.Size.ToResourceQuantity(),
			StorageClassName: in.Spec.StorageConfiguration.StorageClass,
		},
		ObjStoreSecret: in.Spec.ObjectStorageConfig.ToSecretKeySelector(),
	}

	if in.Spec.TimeRangeConfig != nil {
		cops.Min = ptr.To(manifests.Duration(manifests.OptionalToString(in.Spec.TimeRangeConfig.MinTime)))
		cops.Max = ptr.To(manifests.Duration(manifests.OptionalToString(in.Spec.TimeRangeConfig.MaxTime)))
	}

	return cops
}

// CompactNameFromParent returns the name of the Thanos Compact component.
func CompactNameFromParent(resourceName string) string {
	opts := manifestscompact.Options{Options: manifests.Options{Owner: resourceName}}
	if err := opts.Valid(); err != nil {
		panic("invalid compact options")
	}
	return opts.GetGeneratedResourceName()
}

// StoreNameFromParent returns the name of the Thanos Store component.
func StoreNameFromParent(resourceName string, index *int32) string {
	opts := manifestsstore.Options{Options: manifests.Options{Owner: resourceName}, ShardIndex: index}
	if err := opts.Valid(); err != nil {
		panic("invalid store options")
	}
	return opts.GetGeneratedResourceName()
}

func commonToOpts(
	owner client.Object,
	replicas int32,
	labels map[string]string,
	annotations map[string]string,
	common v1alpha1.CommonFields,
	statefulSet *v1alpha1.StatefulSetFields,
	featureGate featuregate.Config,
	additional v1alpha1.Additional,
) manifests.Options {

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
		ServiceMonitorConfig: serviceMonitorConfigToOptsGlobal(featureGate, labels),
		PodDisruptionConfig:  podDisruptionBudgetConfigToOpts(replicas, common.PodDisruptionBudgetConfig),
		PlacementConfig: &manifests.Placement{
			NodeSelector: common.NodeSelector,
			Affinity:     common.Affinity,
			Tolerations:  common.Tolerations,
		},
		StatefulSet:     statefulSetToOpts(statefulSet),
		SecurityContext: common.SecurityContext,
	}
}

func statefulSetToOpts(in *v1alpha1.StatefulSetFields) manifests.StatefulSet {
	if in == nil {
		return manifests.StatefulSet{}
	}

	return manifests.StatefulSet{
		PodManagementPolicy: string(ptr.Deref(in.PodManagementPolicy, "")),
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

func serviceMonitorConfigToOptsGlobal(fg featuregate.Config, labels map[string]string) *manifests.ServiceMonitorConfig {
	if !fg.ServiceMonitorEnabled() {
		return nil
	}
	return &manifests.ServiceMonitorConfig{
		Labels: labels,
	}
}

func podDisruptionBudgetConfigToOpts(replicas int32, pdb *v1alpha1.PodDisruptionBudgetConfig) *manifests.PodDisruptionBudgetOptions {
	if replicas < 2 || pdb == nil {
		return nil
	}

	if pdb.Enable != nil && !*pdb.Enable {
		return nil
	}

	// set the basic pdb config
	return &manifests.PodDisruptionBudgetOptions{
		MaxUnavailable: ptr.To(int32(1)),
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
