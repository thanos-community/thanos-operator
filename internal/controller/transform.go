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

// QueryV1Alpha1TransformInput holds input for queryV1Alpha1ToOptions.
type queryV1Alpha1TransformInput struct {
	CRD         v1alpha1.ThanosQuery
	FeatureGate featuregate.Config
}

// QueryV1Alpha1ToQueryFrontEndTransformInput holds input for queryV1Alpha1ToQueryFrontEndOptions.
type queryV1Alpha1ToQueryFrontEndTransformInput struct {
	CRD         v1alpha1.ThanosQuery
	FeatureGate featuregate.Config
}

// RulerV1Alpha1TransformInput holds input for rulerV1Alpha1ToOptions.
type rulerV1Alpha1TransformInput struct {
	CRD                 v1alpha1.ThanosRuler
	FeatureGate         featuregate.Config
	ConfigReloaderImage string
}

// ReceiverV1Alpha1ToIngesterTransformInput holds input for receiverV1Alpha1ToIngesterOptions.
type receiverV1Alpha1ToIngesterTransformInput struct {
	CRD         v1alpha1.ThanosReceive
	Spec        v1alpha1.IngesterHashringSpec
	FeatureGate featuregate.Config
}

// ReceiverV1Alpha1ToRouterTransformInput holds input for receiverV1Alpha1ToRouterOptions.
type receiverV1Alpha1ToRouterTransformInput struct {
	CRD         v1alpha1.ThanosReceive
	FeatureGate featuregate.Config
}

// StoreV1Alpha1TransformInput holds input for storeV1Alpha1ToOptions.
type storeV1Alpha1TransformInput struct {
	CRD         v1alpha1.ThanosStore
	FeatureGate featuregate.Config
}

// CompactV1Alpha1TransformInput holds input for compactV1Alpha1ToOptions.
type compactV1Alpha1TransformInput struct {
	CRD         v1alpha1.ThanosCompact
	FeatureGate featuregate.Config
}

func queryV1Alpha1ToOptions(in queryV1Alpha1TransformInput) manifestquery.Options {
	labels := manifests.MergeLabels(in.CRD.GetLabels(), in.CRD.Spec.CommonFields.Labels)
	opts := commonToOpts(&in.CRD, in.CRD.Spec.Replicas, labels, in.CRD.GetAnnotations(), in.CRD.Spec.CommonFields, nil, in.FeatureGate, in.CRD.Spec.Additional)
	var webOptions manifestquery.WebOptions
	if in.CRD.Spec.WebConfig != nil {
		webOptions = manifestquery.WebOptions{
			RoutePrefix:    manifests.OptionalToString(in.CRD.Spec.WebConfig.RoutePrefix),
			ExternalPrefix: manifests.OptionalToString(in.CRD.Spec.WebConfig.ExternalPrefix),
			PrefixHeader:   manifests.OptionalToString(in.CRD.Spec.WebConfig.PrefixHeader),
			DisableCORS:    in.CRD.Spec.WebConfig.DisableCORS != nil && *in.CRD.Spec.WebConfig.DisableCORS,
		}
	}
	var telemetryQuantiles manifestquery.TelemetryQuantiles
	if in.CRD.Spec.TelemetryQuantiles != nil {
		telemetryQuantiles = manifestquery.TelemetryQuantiles{
			Duration: in.CRD.Spec.TelemetryQuantiles.Duration,
			Samples:  in.CRD.Spec.TelemetryQuantiles.Samples,
			Series:   in.CRD.Spec.TelemetryQuantiles.Series,
		}
	}
	return manifestquery.Options{
		Options:            opts,
		ReplicaLabels:      in.CRD.Spec.ReplicaLabels,
		Timeout:            "15m",
		LookbackDelta:      "5m",
		MaxConcurrent:      20,
		WebOptions:         webOptions,
		TelemetryQuantiles: telemetryQuantiles,
		GRPCProxyStrategy:  in.CRD.Spec.GRPCProxyStrategy,
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
func queryV1Alpha1ToQueryFrontEndOptions(in queryV1Alpha1ToQueryFrontEndTransformInput) manifestqueryfrontend.Options {
	frontend := in.CRD.Spec.QueryFrontend
	labels := manifests.MergeLabels(in.CRD.GetLabels(), frontend.CommonFields.Labels)

	// Parent annotations + query-frontend specific annotations
	// Query-frontend annotations take precedence in case of conflicts
	annotations := manifests.MergeLabels(in.CRD.GetAnnotations(), frontend.Annotations)

	opts := commonToOpts(&in.CRD, frontend.Replicas, labels, annotations, frontend.CommonFields, nil, in.FeatureGate, frontend.Additional)

	return manifestqueryfrontend.Options{
		Options:                opts,
		QueryService:           QueryNameFromParent(in.CRD.GetName()),
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

func rulerV1Alpha1ToOptions(in rulerV1Alpha1TransformInput) manifestruler.Options {
	labels := manifests.MergeLabels(in.CRD.GetLabels(), in.CRD.Spec.CommonFields.Labels)
	opts := commonToOpts(&in.CRD, in.CRD.Spec.Replicas, labels, in.CRD.GetAnnotations(), in.CRD.Spec.CommonFields, &in.CRD.Spec.StatefulSetFields, in.FeatureGate, in.CRD.Spec.Additional)
	return manifestruler.Options{
		Options:         opts,
		ObjStoreSecret:  in.CRD.Spec.ObjectStorageConfig.ToSecretKeySelector(),
		Retention:       manifests.Duration(in.CRD.Spec.Retention),
		AlertmanagerURL: in.CRD.Spec.AlertmanagerURL,
		ExternalLabels:  in.CRD.Spec.ExternalLabels,
		AlertLabelDrop:  in.CRD.Spec.AlertLabelDrop,
		StorageConfig: manifests.StorageConfig{
			StorageSize:      in.CRD.Spec.StorageConfiguration.Size.ToResourceQuantity(),
			StorageClassName: in.CRD.Spec.StorageConfiguration.StorageClass,
		},
		EvaluationInterval:  manifests.Duration(in.CRD.Spec.EvaluationInterval),
		ConfigReloaderImage: in.ConfigReloaderImage,
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

func receiverV1Alpha1ToIngesterOptions(in receiverV1Alpha1ToIngesterTransformInput) manifestreceive.IngesterOptions {
	labels := manifests.MergeLabels(in.CRD.GetLabels(), in.Spec.CommonFields.Labels)
	common := in.Spec.CommonFields
	additional := in.CRD.Spec.Ingester.Additional
	secret := in.CRD.Spec.Ingester.DefaultObjectStorageConfig.ToSecretKeySelector()
	if in.Spec.ObjectStorageConfig != nil {
		secret = in.Spec.ObjectStorageConfig.ToSecretKeySelector()
	}

	opts := commonToOpts(&in.CRD, in.Spec.Replicas, labels, in.CRD.GetAnnotations(), common, &in.CRD.Spec.StatefulSetFields, in.FeatureGate, additional)
	ingestOpts := manifestreceive.IngesterOptions{
		Options:        opts,
		ObjStoreSecret: secret,
		TSDBOpts: manifestreceive.TSDBOpts{
			Retention: string(in.Spec.TSDBConfig.Retention),
		},
		AsyncForwardWorkerCount:  manifests.OptionalToString(in.Spec.AsyncForwardWorkerCount),
		TooFarInFutureTimeWindow: manifests.Duration(manifests.OptionalToString(in.Spec.TooFarInFutureTimeWindow)),
		StorageConfig: manifests.StorageConfig{
			StorageSize:      in.Spec.StorageConfiguration.Size.ToResourceQuantity(),
			StorageClassName: in.Spec.StorageConfiguration.StorageClass,
		},
		ExternalLabels: in.Spec.ExternalLabels,
	}

	if in.CRD.Spec.Router.ReplicationProtocol != nil {
		ingestOpts.ReplicationProtocol = string(*in.CRD.Spec.Router.ReplicationProtocol)
	}

	if in.Spec.GRPCCompression != nil {
		ingestOpts.GRPCCompression = string(*in.Spec.GRPCCompression)
	}

	if in.Spec.TenancyConfig != nil {
		ingestOpts.TenancyOpts = manifestreceive.TenancyOpts{
			TenantHeader:           in.Spec.TenancyConfig.TenantHeader,
			TenantCertificateField: manifests.OptionalToString(in.Spec.TenancyConfig.TenantCertificateField),
			DefaultTenantID:        in.Spec.TenancyConfig.DefaultTenantID,
			SplitTenantLabelName:   manifests.OptionalToString(in.Spec.TenancyConfig.SplitTenantLabelName),
			TenantLabelName:        in.Spec.TenancyConfig.TenantLabelName,
		}
	}

	if in.Spec.StoreLimitsOptions != nil {
		ingestOpts.StoreLimitsOpts = manifests.StoreLimitsOpts{
			StoreLimitsRequestSamples: in.Spec.StoreLimitsOptions.StoreLimitsRequestSamples,
			StoreLimitsRequestSeries:  in.Spec.StoreLimitsOptions.StoreLimitsRequestSeries,
		}
	}

	return ingestOpts
}

func receiverV1Alpha1ToRouterOptions(in receiverV1Alpha1ToRouterTransformInput) manifestreceive.RouterOptions {
	router := in.CRD.Spec.Router
	labels := manifests.MergeLabels(in.CRD.GetLabels(), router.CommonFields.Labels)
	opts := commonToOpts(&in.CRD, router.Replicas, labels, in.CRD.GetAnnotations(), router.CommonFields, &in.CRD.Spec.StatefulSetFields, in.FeatureGate, router.Additional)

	ropts := manifestreceive.RouterOptions{
		Options:           opts,
		ReplicationFactor: router.ReplicationFactor,
		ExternalLabels:    router.ExternalLabels,
	}

	if in.FeatureGate.KubeResourceSyncEnabled() {
		ropts.FeatureGateConfig = &manifestreceive.FeatureGateConfig{
			KubeResourceSyncEnabled: in.FeatureGate.KubeResourceSyncEnabled(),
			KubeResourceSyncImage:   in.FeatureGate.GetKubeResourceSyncImage(),
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

func storeV1Alpha1ToOptions(in storeV1Alpha1TransformInput) manifestsstore.Options {
	labels := manifests.MergeLabels(in.CRD.GetLabels(), in.CRD.Spec.CommonFields.Labels)
	opts := commonToOpts(&in.CRD, in.CRD.Spec.Replicas, labels, in.CRD.GetAnnotations(), in.CRD.Spec.CommonFields, &in.CRD.Spec.StatefulSetFields, in.FeatureGate, in.CRD.Spec.Additional)
	var indexHeaderOpts *manifestsstore.IndexHeaderOptions
	if in.CRD.Spec.IndexHeaderConfig != nil {
		indexHeaderOpts = &manifestsstore.IndexHeaderOptions{
			EnableLazyReader:      in.CRD.Spec.IndexHeaderConfig.EnableLazyReader != nil && *in.CRD.Spec.IndexHeaderConfig.EnableLazyReader,
			LazyReaderIdleTimeout: manifests.Duration(manifests.OptionalToString(in.CRD.Spec.IndexHeaderConfig.LazyReaderIdleTimeout)),
			LazyDownloadStrategy:  manifests.OptionalToString(in.CRD.Spec.IndexHeaderConfig.LazyDownloadStrategy),
		}
	}
	var blockConfigOpts *manifestsstore.BlockConfigOptions
	if in.CRD.Spec.BlockConfig != nil {
		blockConfigOpts = &manifestsstore.BlockConfigOptions{
			BlockDiscoveryStrategy:    ptr.To(string(in.CRD.Spec.BlockConfig.BlockDiscoveryStrategy)),
			BlockMetaFetchConcurrency: in.CRD.Spec.BlockConfig.BlockMetaFetchConcurrency,
		}
	}
	sops := manifestsstore.Options{
		ObjStoreSecret:           in.CRD.Spec.ObjectStorageConfig.ToSecretKeySelector(),
		IndexCacheConfig:         toManifestCacheConfig(in.CRD.Spec.IndexCacheConfig),
		CachingBucketConfig:      toManifestCacheConfig(in.CRD.Spec.CachingBucketConfig),
		IgnoreDeletionMarksDelay: manifests.Duration(in.CRD.Spec.IgnoreDeletionMarksDelay),
		IndexHeaderOptions:       indexHeaderOpts,
		BlockConfigOptions:       blockConfigOpts,
		StorageConfig: manifests.StorageConfig{
			StorageSize:      in.CRD.Spec.StorageConfiguration.Size.ToResourceQuantity(),
			StorageClassName: in.CRD.Spec.StorageConfiguration.StorageClass,
		},
		Options: opts,
	}

	if in.CRD.Spec.StoreLimitsOptions != nil {
		sops.StoreLimitsOpts = manifests.StoreLimitsOpts{
			StoreLimitsRequestSamples: in.CRD.Spec.StoreLimitsOptions.StoreLimitsRequestSamples,
			StoreLimitsRequestSeries:  in.CRD.Spec.StoreLimitsOptions.StoreLimitsRequestSeries,
		}
	}

	if in.CRD.Spec.TimeRangeConfig != nil {
		sops.Min = manifests.Duration(manifests.OptionalToString(in.CRD.Spec.TimeRangeConfig.MinTime))
		sops.Max = manifests.Duration(manifests.OptionalToString(in.CRD.Spec.TimeRangeConfig.MaxTime))
	}

	return sops
}

func compactV1Alpha1ToOptions(in compactV1Alpha1TransformInput) manifestscompact.Options {
	labels := manifests.MergeLabels(in.CRD.GetLabels(), in.CRD.Spec.CommonFields.Labels)
	opts := commonToOpts(&in.CRD, 1, labels, in.CRD.GetAnnotations(), in.CRD.Spec.CommonFields, &in.CRD.Spec.StatefulSetFields, in.FeatureGate, in.CRD.Spec.Additional)
	// we always set nil for compactor since it should run as single pod
	opts.PodDisruptionConfig = nil
	opts.PodManagementPolicy = string(ptr.Deref(in.CRD.Spec.PodManagementPolicy, ""))

	downsamplingConfig := func() *manifestscompact.DownsamplingOptions {
		if in.CRD.Spec.DownsamplingConfig == nil {
			return nil
		}

		disable := in.CRD.Spec.DownsamplingConfig.Disable != nil && *in.CRD.Spec.DownsamplingConfig.Disable

		return &manifestscompact.DownsamplingOptions{
			Disable:     disable,
			Concurrency: in.CRD.Spec.DownsamplingConfig.Concurrency,
		}
	}

	compaction := func() *manifestscompact.CompactionOptions {
		if in.CRD.Spec.CompactConfig == nil {
			return nil
		}
		return &manifestscompact.CompactionOptions{
			CompactConcurrency:           in.CRD.Spec.CompactConfig.CompactConcurrency,
			CompactCleanupInterval:       ptr.To(manifests.Duration(*in.CRD.Spec.CompactConfig.CleanupInterval)),
			ConsistencyDelay:             ptr.To(manifests.Duration(*in.CRD.Spec.CompactConfig.ConsistencyDelay)),
			CompactBlockFetchConcurrency: in.CRD.Spec.CompactConfig.BlockFetchConcurrency,
		}
	}
	blockDiscovery := func() *manifestscompact.BlockConfigOptions {
		if in.CRD.Spec.BlockConfig == nil && in.CRD.Spec.BlockViewerGlobalSync == nil {
			return nil
		}
		opts := &manifestscompact.BlockConfigOptions{}

		if in.CRD.Spec.BlockConfig != nil {
			opts.BlockDiscoveryStrategy = ptr.To(string(in.CRD.Spec.BlockConfig.BlockDiscoveryStrategy))
			opts.BlockFilesConcurrency = in.CRD.Spec.BlockConfig.BlockFilesConcurrency
			opts.BlockMetaFetchConcurrency = in.CRD.Spec.BlockConfig.BlockMetaFetchConcurrency
		}

		if in.CRD.Spec.BlockViewerGlobalSync != nil {
			opts.BlockViewerGlobalSyncInterval = ptr.To(manifests.Duration(*in.CRD.Spec.BlockViewerGlobalSync.BlockViewerGlobalSyncInterval))
			opts.BlockViewerGlobalSyncTimeout = ptr.To(manifests.Duration(*in.CRD.Spec.BlockViewerGlobalSync.BlockViewerGlobalSyncTimeout))
		}
		return opts
	}
	debugConfig := func() *manifestscompact.DebugConfigOptions {
		if in.CRD.Spec.DebugConfig == nil {
			return nil
		}

		var mcl int32
		if in.CRD.Spec.DebugConfig.MaxCompactionLevel != nil {
			mcl = *in.CRD.Spec.DebugConfig.MaxCompactionLevel
		}
		return &manifestscompact.DebugConfigOptions{
			AcceptMalformedIndex: in.CRD.Spec.DebugConfig.AcceptMalformedIndex != nil && *in.CRD.Spec.DebugConfig.AcceptMalformedIndex,
			MaxCompactionLevel:   mcl,
			HaltOnError:          in.CRD.Spec.DebugConfig.HaltOnError != nil && *in.CRD.Spec.DebugConfig.HaltOnError,
		}
	}

	cops := manifestscompact.Options{
		Options: opts,
		RetentionOptions: &manifestscompact.RetentionOptions{
			Raw:         ptr.To(manifests.Duration(in.CRD.Spec.RetentionConfig.Raw)),
			FiveMinutes: ptr.To(manifests.Duration(in.CRD.Spec.RetentionConfig.FiveMinutes)),
			OneHour:     ptr.To(manifests.Duration(in.CRD.Spec.RetentionConfig.OneHour)),
		},
		BlockConfig:  blockDiscovery(),
		Compaction:   compaction(),
		Downsampling: downsamplingConfig(),
		DebugConfig:  debugConfig(),
		StorageConfig: manifests.StorageConfig{
			StorageSize:      in.CRD.Spec.StorageConfiguration.Size.ToResourceQuantity(),
			StorageClassName: in.CRD.Spec.StorageConfiguration.StorageClass,
		},
		ObjStoreSecret: in.CRD.Spec.ObjectStorageConfig.ToSecretKeySelector(),
	}

	if in.CRD.Spec.TimeRangeConfig != nil {
		cops.Min = ptr.To(manifests.Duration(manifests.OptionalToString(in.CRD.Spec.TimeRangeConfig.MinTime)))
		cops.Max = ptr.To(manifests.Duration(manifests.OptionalToString(in.CRD.Spec.TimeRangeConfig.MaxTime)))
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
