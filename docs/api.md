# API Reference

## Packages
- [monitoring.thanos.io/v1alpha1](#monitoringthanosiov1alpha1)


## monitoring.thanos.io/v1alpha1

Package v1alpha1 contains API Schema definitions for the  v1alpha1 API group

### Resource Types
- [ThanosCompact](#thanoscompact)
- [ThanosCompactList](#thanoscompactlist)
- [ThanosQuery](#thanosquery)
- [ThanosQueryList](#thanosquerylist)
- [ThanosReceive](#thanosreceive)
- [ThanosReceiveList](#thanosreceivelist)
- [ThanosRuler](#thanosruler)
- [ThanosRulerList](#thanosrulerlist)
- [ThanosStore](#thanosstore)
- [ThanosStoreList](#thanosstorelist)



#### Additional



Additional holds additional configuration for the Thanos components.



_Appears in:_
- [IngesterSpec](#ingesterspec)
- [QueryFrontendSpec](#queryfrontendspec)
- [RouterSpec](#routerspec)
- [ThanosCompactSpec](#thanoscompactspec)
- [ThanosQuerySpec](#thanosqueryspec)
- [ThanosRulerSpec](#thanosrulerspec)
- [ThanosStoreSpec](#thanosstorespec)

| Field | Description | Default | Validation |
| --- | --- | --- | --- |
| `additionalArgs` _string array_ | Additional arguments to pass to the Thanos components. |  | Optional: \{\} <br /> |
| `additionalContainers` _[Container](https://kubernetes.io/docs/reference/generated/kubernetes-api/v1.31/#container-v1-core) array_ | Additional containers to add to the Thanos components. |  | Optional: \{\} <br /> |
| `additionalVolumes` _[Volume](https://kubernetes.io/docs/reference/generated/kubernetes-api/v1.31/#volume-v1-core) array_ | Additional volumes to add to the Thanos components. |  | Optional: \{\} <br /> |
| `additionalVolumeMounts` _[VolumeMount](https://kubernetes.io/docs/reference/generated/kubernetes-api/v1.31/#volumemount-v1-core) array_ | Additional volume mounts to add to the Thanos component container in a Deployment or StatefulSet<br />controlled by the operator. |  | Optional: \{\} <br /> |
| `additionalPorts` _[ContainerPort](https://kubernetes.io/docs/reference/generated/kubernetes-api/v1.31/#containerport-v1-core) array_ | Additional ports to expose on the Thanos component container in a Deployment or StatefulSet<br />controlled by the operator. |  | Optional: \{\} <br /> |
| `additionalEnv` _[EnvVar](https://kubernetes.io/docs/reference/generated/kubernetes-api/v1.31/#envvar-v1-core) array_ | Additional environment variables to add to the Thanos component container in a Deployment or StatefulSet<br />controlled by the operator. |  | Optional: \{\} <br /> |
| `additionalServicePorts` _[ServicePort](https://kubernetes.io/docs/reference/generated/kubernetes-api/v1.31/#serviceport-v1-core) array_ | AdditionalServicePorts are additional ports to expose on the Service for the Thanos component. |  | Optional: \{\} <br /> |


#### BlockConfig



BlockConfig defines settings for block handling.



_Appears in:_
- [ThanosCompactSpec](#thanoscompactspec)
- [ThanosStoreSpec](#thanosstorespec)

| Field | Description | Default | Validation |
| --- | --- | --- | --- |
| `blockDiscoveryStrategy` _[BlockDiscoveryStrategy](#blockdiscoverystrategy)_ | BlockDiscoveryStrategy is the discovery strategy to use for block discovery in storage. | concurrent | Enum: [concurrent recursive] <br /> |
| `blockFilesConcurrency` _integer_ | BlockFilesConcurrency is the number of goroutines to use when to use when<br />fetching/uploading block files from object storage.<br />Only used for Compactor, no-op for store gateway | 1 | Optional: \{\} <br /> |
| `blockMetaFetchConcurrency` _integer_ | BlockMetaFetchConcurrency is the number of goroutines to use when fetching block metadata from object storage. | 32 | Optional: \{\} <br /> |


#### BlockDiscoveryStrategy

_Underlying type:_ _string_

BlockDiscoveryStrategy represents the strategy to use for block discovery.



_Appears in:_
- [BlockConfig](#blockconfig)

| Field | Description |
| --- | --- |
| `concurrent` | BlockDiscoveryStrategyConcurrent means stores will concurrently issue one call<br />per directory to discover active blocks storage.<br /> |
| `recursive` | BlockDiscoveryStrategyRecursive means stores iterate through all objects in storage<br />recursively traversing into each directory.<br />This avoids N+1 calls at the expense of having slower bucket iterations.<br /> |


#### BlockViewerGlobalSyncConfig



BlockViewerGlobalSyncConfig is the configuration for syncing the blocks between local and remote view for /global Block Viewer UI.



_Appears in:_
- [ThanosCompactSpec](#thanoscompactspec)

| Field | Description | Default | Validation |
| --- | --- | --- | --- |
| `blockViewerGlobalSync` _[Duration](#duration)_ | BlockViewerGlobalSyncInterval for syncing the blocks between local and remote view for /global Block Viewer UI. | 1m | Optional: \{\} <br />Pattern: `^-?(0\|(([0-9]+)y)?(([0-9]+)w)?(([0-9]+)d)?(([0-9]+)h)?(([0-9]+)m)?(([0-9]+)s)?(([0-9]+)ms)?)$` <br /> |
| `blockViewerGlobalSyncTimeout` _[Duration](#duration)_ | BlockViewerGlobalSyncTimeout is the maximum time for syncing the blocks<br />between local and remote view for /global Block Viewer UI. | 5m | Optional: \{\} <br />Pattern: `^-?(0\|(([0-9]+)y)?(([0-9]+)w)?(([0-9]+)d)?(([0-9]+)h)?(([0-9]+)m)?(([0-9]+)s)?(([0-9]+)ms)?)$` <br /> |


#### CacheConfig



CacheConfig is the configuration for the cache.
If both InMemoryCacheConfig and ExternalCacheConfig are specified, the operator will prefer the ExternalCacheConfig.



_Appears in:_
- [QueryFrontendSpec](#queryfrontendspec)
- [ThanosStoreSpec](#thanosstorespec)

| Field | Description | Default | Validation |
| --- | --- | --- | --- |
| `inMemoryCacheConfig` _[InMemoryCacheConfig](#inmemorycacheconfig)_ | InMemoryCacheConfig is the configuration for the in-memory cache. |  | Optional: \{\} <br /> |
| `externalCacheConfig` _[SecretKeySelector](https://kubernetes.io/docs/reference/generated/kubernetes-api/v1.31/#secretkeyselector-v1-core)_ | ExternalCacheConfig is the configuration for the external cache. |  | Optional: \{\} <br /> |


#### CommonFields



CommonFields are the options available to all Thanos components.
These fields reflect runtime changes to managed StatefulSet and Deployment resources.



_Appears in:_
- [IngesterHashringSpec](#ingesterhashringspec)
- [QueryFrontendSpec](#queryfrontendspec)
- [RouterSpec](#routerspec)
- [ThanosCompactSpec](#thanoscompactspec)
- [ThanosQuerySpec](#thanosqueryspec)
- [ThanosRulerSpec](#thanosrulerspec)
- [ThanosStoreSpec](#thanosstorespec)

| Field | Description | Default | Validation |
| --- | --- | --- | --- |
| `version` _string_ | Version of Thanos to be deployed.<br />If not specified, the operator assumes the latest upstream version of<br />Thanos available at the time when the version of the operator was released. |  | Optional: \{\} <br /> |
| `image` _string_ | Container image to use for the Thanos components. |  | Optional: \{\} <br /> |
| `imagePullPolicy` _[PullPolicy](https://kubernetes.io/docs/reference/generated/kubernetes-api/v1.31/#pullpolicy-v1-core)_ | Image pull policy for the Thanos containers.<br />See https://kubernetes.io/docs/concepts/containers/images/#image-pull-policy for more details. | IfNotPresent | Enum: [Always Never IfNotPresent] <br />Optional: \{\} <br /> |
| `imagePullSecrets` _[LocalObjectReference](https://kubernetes.io/docs/reference/generated/kubernetes-api/v1.31/#localobjectreference-v1-core) array_ | An optional list of references to Secrets in the same namespace<br />to use for pulling images from registries.<br />See http://kubernetes.io/docs/user-guide/images#specifying-imagepullsecrets-on-a-pod |  | Optional: \{\} <br /> |
| `resourceRequirements` _[ResourceRequirements](https://kubernetes.io/docs/reference/generated/kubernetes-api/v1.31/#resourcerequirements-v1-core)_ | ResourceRequirements for the Thanos component container. |  | Optional: \{\} <br /> |
| `logLevel` _string_ | Log level for Thanos. |  | Enum: [debug info warn error] <br />Optional: \{\} <br /> |
| `logFormat` _string_ | Log format for Thanos. | logfmt | Enum: [logfmt json] <br />Optional: \{\} <br /> |


#### CompactConfig







_Appears in:_
- [ThanosCompactSpec](#thanoscompactspec)

| Field | Description | Default | Validation |
| --- | --- | --- | --- |
| `compactConcurrency` _integer_ | CompactConcurrency is the number of goroutines to use when compacting blocks. | 1 | Optional: \{\} <br /> |
| `blockFetchConcurrency` _integer_ | BlockFetchConcurrency is the number of goroutines to use when fetching blocks from object storage. | 1 | Optional: \{\} <br /> |
| `cleanupInterval` _[Duration](#duration)_ | CleanupInterval configures how often we should clean up partially uploaded blocks and blocks<br />that are marked for deletion.<br />Cleaning happens at the end of an iteration.<br />Setting this to 0s disables the cleanup. | 5m | Optional: \{\} <br />Pattern: `^-?(0\|(([0-9]+)y)?(([0-9]+)w)?(([0-9]+)d)?(([0-9]+)h)?(([0-9]+)m)?(([0-9]+)s)?(([0-9]+)ms)?)$` <br /> |
| `blockConsistencyDelay` _[Duration](#duration)_ | ConsistencyDelay is the minimum age of fresh (non-compacted) blocks before they are being processed.<br />Malformed blocks older than the maximum of consistency-delay and 48h0m0s will be removed. | 30m | Optional: \{\} <br />Pattern: `^-?(0\|(([0-9]+)y)?(([0-9]+)w)?(([0-9]+)d)?(([0-9]+)h)?(([0-9]+)m)?(([0-9]+)s)?(([0-9]+)ms)?)$` <br /> |


#### DebugConfig







_Appears in:_
- [ThanosCompactSpec](#thanoscompactspec)

| Field | Description | Default | Validation |
| --- | --- | --- | --- |
| `acceptMalformedIndex` _boolean_ | AcceptMalformedIndex allows compact to accept blocks with malformed index. | false | Optional: \{\} <br /> |
| `maxCompactionLevel` _integer_ | MaxCompactionLevel is the maximum compaction level to use when compacting blocks. | 5 | Optional: \{\} <br /> |
| `haltOnError` _boolean_ | HaltOnError halts the compact process on critical compaction error. | false | Optional: \{\} <br /> |


#### DownsamplingConfig



DownsamplingConfig defines the downsampling configuration for the compact component.



_Appears in:_
- [ThanosCompactSpec](#thanoscompactspec)

| Field | Description | Default | Validation |
| --- | --- | --- | --- |
| `downsamplingEnabled` _boolean_ | Disable downsampling. | false |  |
| `downsamplingConcurrency` _integer_ | Concurrency is the number of goroutines to use when downsampling blocks. | 1 | Optional: \{\} <br /> |


#### Duration

_Underlying type:_ _string_

Duration is a valid time duration that can be parsed by Prometheus model.ParseDuration() function.
Supported units: y, w, d, h, m, s, ms
Examples: `30s`, `1m`, `1h20m15s`, `15d`

_Validation:_
- Pattern: `^-?(0|(([0-9]+)y)?(([0-9]+)w)?(([0-9]+)d)?(([0-9]+)h)?(([0-9]+)m)?(([0-9]+)s)?(([0-9]+)ms)?)$`

_Appears in:_
- [BlockViewerGlobalSyncConfig](#blockviewerglobalsyncconfig)
- [CompactConfig](#compactconfig)
- [IndexHeaderConfig](#indexheaderconfig)
- [IngesterHashringSpec](#ingesterhashringspec)
- [QueryFrontendSpec](#queryfrontendspec)
- [RetentionResolutionConfig](#retentionresolutionconfig)
- [TSDBConfig](#tsdbconfig)
- [ThanosCompactSpec](#thanoscompactspec)
- [ThanosRulerSpec](#thanosrulerspec)
- [ThanosStoreSpec](#thanosstorespec)



#### ExternalLabelShardingConfig



ExternalLabelShardingConfig defines the sharding configuration based on explicit external labels and their values.
The keys are the external labels to shard on and the values are the values (as regular expressions) to shard on.
Each value will be a configured and deployed as a separate compact component.
For example, if the 'label' is set to `tenant_id` with values `tenant-a` and `!tenant-a`
two compact components will be deployed.
The resulting compact StatefulSets will have an appropriate --selection.relabel-config flag set to the value of the external label sharding.
And named such that:


		The first compact component will have the name {ThanosCompact.Name}-{shardName}-0 with the flag
	    --selector.relabel-config=
	       - source_labels:
	         - tenant_id
	         regex: 'tenant-a'
	         action: keep


		The second compact component will have the name {ThanosCompact.Name}-{shardName}-1 with the flag
	    --selector.relabel-config=
	       - source_labels:
	         - tenant_id
	         regex: '!tenant-a'
	         action: keep



_Appears in:_
- [ShardingConfig](#shardingconfig)

| Field | Description | Default | Validation |
| --- | --- | --- | --- |
| `shardName` _string_ | ShardName is the name of the shard.<br />ShardName is used to identify the shard in the compact component. |  | Required: \{\} <br /> |
| `label` _string_ | Label is the external label to shard on. |  | Required: \{\} <br /> |
| `values` _string array_ | Values are the values (as regular expressions) to shard on. |  |  |


#### ExternalLabels

_Underlying type:_ _object_

ExternalLabels are the labels to add to the metrics.
POD_NAME and POD_NAMESPACE are available via the downward API.
https://thanos.io/tip/thanos/storage.md/#external-labels

_Validation:_
- MinProperties: 1

_Appears in:_
- [IngesterHashringSpec](#ingesterhashringspec)
- [RouterSpec](#routerspec)
- [ThanosRulerSpec](#thanosrulerspec)



#### FeatureGates



FeatureGates holds the configuration for behaviour that is behind feature flags in the operator.



_Appears in:_
- [ThanosCompactSpec](#thanoscompactspec)
- [ThanosQuerySpec](#thanosqueryspec)
- [ThanosReceiveSpec](#thanosreceivespec)
- [ThanosRulerSpec](#thanosrulerspec)
- [ThanosStoreSpec](#thanosstorespec)

| Field | Description | Default | Validation |
| --- | --- | --- | --- |
| `serviceMonitor` _[ServiceMonitorConfig](#servicemonitorconfig)_ | ServiceMonitorConfig is the configuration for the ServiceMonitor.<br />This setting requires the feature gate for ServiceMonitor management to be enabled. | \{ enable:true \} | Optional: \{\} <br /> |
| `prometheusRuleEnabled` _boolean_ | PrometheusRuleEnabled enables the loading of PrometheusRules into the Thanos Ruler.<br />This setting is only applicable to ThanosRuler CRD, will be ignored for other components. | true | Optional: \{\} <br /> |


#### InMemoryCacheConfig



InMemoryCacheConfig is the configuration for the in-memory cache.



_Appears in:_
- [CacheConfig](#cacheconfig)

| Field | Description | Default | Validation |
| --- | --- | --- | --- |
| `maxSize` _[StorageSize](#storagesize)_ |  |  | Pattern: `^([+-]?[0-9.]+)([eEinumkKMGTP]*[-+]?[0-9]*)$` <br /> |
| `maxItemSize` _[StorageSize](#storagesize)_ |  |  | Pattern: `^([+-]?[0-9.]+)([eEinumkKMGTP]*[-+]?[0-9]*)$` <br /> |


#### IndexHeaderConfig



IndexHeaderConfig allows configuration of the Store Gateway index header.



_Appears in:_
- [ThanosStoreSpec](#thanosstorespec)

| Field | Description | Default | Validation |
| --- | --- | --- | --- |
| `enableLazyReader` _boolean_ | If true, Store Gateway will lazy memory map index-header only once the block is required by a query. | true | Optional: \{\} <br /> |
| `lazyReaderIdleTimeout` _[Duration](#duration)_ | If index-header lazy reader is enabled and this idle timeout setting is > 0, memory map-ed index-headers will be automatically released after 'idle timeout' inactivity | 5m | Optional: \{\} <br />Pattern: `^-?(0\|(([0-9]+)y)?(([0-9]+)w)?(([0-9]+)d)?(([0-9]+)h)?(([0-9]+)m)?(([0-9]+)s)?(([0-9]+)ms)?)$` <br /> |
| `lazyDownloadStrategy` _string_ | Strategy of how to download index headers lazily.<br />If eager, always download index header during initial load. If lazy, download index header during query time. | eager | Enum: [eager lazy] <br />Optional: \{\} <br /> |


#### IngesterHashringSpec



IngesterHashringSpec represents the configuration for a hashring to be used by the Thanos Receive StatefulSet.



_Appears in:_
- [IngesterSpec](#ingesterspec)

| Field | Description | Default | Validation |
| --- | --- | --- | --- |
| `version` _string_ | Version of Thanos to be deployed.<br />If not specified, the operator assumes the latest upstream version of<br />Thanos available at the time when the version of the operator was released. |  | Optional: \{\} <br /> |
| `image` _string_ | Container image to use for the Thanos components. |  | Optional: \{\} <br /> |
| `imagePullPolicy` _[PullPolicy](https://kubernetes.io/docs/reference/generated/kubernetes-api/v1.31/#pullpolicy-v1-core)_ | Image pull policy for the Thanos containers.<br />See https://kubernetes.io/docs/concepts/containers/images/#image-pull-policy for more details. | IfNotPresent | Enum: [Always Never IfNotPresent] <br />Optional: \{\} <br /> |
| `imagePullSecrets` _[LocalObjectReference](https://kubernetes.io/docs/reference/generated/kubernetes-api/v1.31/#localobjectreference-v1-core) array_ | An optional list of references to Secrets in the same namespace<br />to use for pulling images from registries.<br />See http://kubernetes.io/docs/user-guide/images#specifying-imagepullsecrets-on-a-pod |  | Optional: \{\} <br /> |
| `resourceRequirements` _[ResourceRequirements](https://kubernetes.io/docs/reference/generated/kubernetes-api/v1.31/#resourcerequirements-v1-core)_ | ResourceRequirements for the Thanos component container. |  | Optional: \{\} <br /> |
| `logLevel` _string_ | Log level for Thanos. |  | Enum: [debug info warn error] <br />Optional: \{\} <br /> |
| `logFormat` _string_ | Log format for Thanos. | logfmt | Enum: [logfmt json] <br />Optional: \{\} <br /> |
| `name` _string_ | Name is the name of the hashring.<br />Name will be used to generate the names for the resources created for the hashring. |  | MaxLength: 253 <br />MinLength: 1 <br />Pattern: `^$\|^[a-z0-9]([-a-z0-9]*[a-z0-9])?(\.[a-z0-9]([-a-z0-9]*[a-z0-9])?)*$` <br />Required: \{\} <br /> |
| `labels` _object (keys:string, values:string)_ | Labels are additional labels to add to the ingester components.<br />Labels set here will overwrite the labels inherited from the ThanosReceive object if they have the same key. |  | Optional: \{\} <br /> |
| `externalLabels` _[ExternalLabels](#externallabels)_ | ExternalLabels to add to the ingesters tsdb blocks. | \{ replica:$(POD_NAME) \} | MinProperties: 1 <br />Required: \{\} <br /> |
| `replicas` _integer_ | Replicas is the number of replicas/members of the hashring to add to the Thanos Receive StatefulSet. | 1 | Minimum: 1 <br />Required: \{\} <br /> |
| `tsdbConfig` _[TSDBConfig](#tsdbconfig)_ | TSDB configuration for the ingestor. |  | Required: \{\} <br /> |
| `objectStorageConfig` _[ObjectStorageConfig](#objectstorageconfig)_ | ObjectStorageConfig is the secret that contains the object storage configuration for the hashring. |  | Optional: \{\} <br /> |
| `storageSize` _[StorageSize](#storagesize)_ | StorageSize is the size of the storage to be used by the Thanos Receive StatefulSet. |  | Pattern: `^([+-]?[0-9.]+)([eEinumkKMGTP]*[-+]?[0-9]*)$` <br />Required: \{\} <br /> |
| `tenancyConfig` _[TenancyConfig](#tenancyconfig)_ | TenancyConfig is the configuration for the tenancy options. |  | Optional: \{\} <br /> |
| `asyncForwardWorkerCount` _integer_ | AsyncForwardWorkerCount is the number of concurrent workers processing forwarding of remote-write requests. | 5 | Optional: \{\} <br /> |
| `storeLimitsOptions` _[StoreLimitsOptions](#storelimitsoptions)_ | StoreLimitsOptions is the configuration for the store API limits options. |  | Optional: \{\} <br /> |
| `tooFarInFutureTimeWindow` _[Duration](#duration)_ | TooFarInFutureTimeWindow is the allowed time window for ingesting samples too far in the future.<br />0s means disabled. | 0s | Optional: \{\} <br />Pattern: `^-?(0\|(([0-9]+)y)?(([0-9]+)w)?(([0-9]+)d)?(([0-9]+)h)?(([0-9]+)m)?(([0-9]+)s)?(([0-9]+)ms)?)$` <br /> |


#### IngesterSpec



IngesterSpec represents the configuration for the ingestor



_Appears in:_
- [ThanosReceiveSpec](#thanosreceivespec)

| Field | Description | Default | Validation |
| --- | --- | --- | --- |
| `defaultObjectStorageConfig` _[ObjectStorageConfig](#objectstorageconfig)_ | DefaultObjectStorageConfig is the secret that contains the object storage configuration for the ingest components.<br />Can be overridden by the ObjectStorageConfig in the IngesterHashringSpec per hashring. |  | Required: \{\} <br /> |
| `hashrings` _[IngesterHashringSpec](#ingesterhashringspec) array_ | Hashrings is a list of hashrings to route to. |  | MaxItems: 100 <br />Required: \{\} <br /> |
| `additionalArgs` _string array_ | Additional arguments to pass to the Thanos components. |  | Optional: \{\} <br /> |
| `additionalContainers` _[Container](https://kubernetes.io/docs/reference/generated/kubernetes-api/v1.31/#container-v1-core) array_ | Additional containers to add to the Thanos components. |  | Optional: \{\} <br /> |
| `additionalVolumes` _[Volume](https://kubernetes.io/docs/reference/generated/kubernetes-api/v1.31/#volume-v1-core) array_ | Additional volumes to add to the Thanos components. |  | Optional: \{\} <br /> |
| `additionalVolumeMounts` _[VolumeMount](https://kubernetes.io/docs/reference/generated/kubernetes-api/v1.31/#volumemount-v1-core) array_ | Additional volume mounts to add to the Thanos component container in a Deployment or StatefulSet<br />controlled by the operator. |  | Optional: \{\} <br /> |
| `additionalPorts` _[ContainerPort](https://kubernetes.io/docs/reference/generated/kubernetes-api/v1.31/#containerport-v1-core) array_ | Additional ports to expose on the Thanos component container in a Deployment or StatefulSet<br />controlled by the operator. |  | Optional: \{\} <br /> |
| `additionalEnv` _[EnvVar](https://kubernetes.io/docs/reference/generated/kubernetes-api/v1.31/#envvar-v1-core) array_ | Additional environment variables to add to the Thanos component container in a Deployment or StatefulSet<br />controlled by the operator. |  | Optional: \{\} <br /> |
| `additionalServicePorts` _[ServicePort](https://kubernetes.io/docs/reference/generated/kubernetes-api/v1.31/#serviceport-v1-core) array_ | AdditionalServicePorts are additional ports to expose on the Service for the Thanos component. |  | Optional: \{\} <br /> |


#### ObjectStorageConfig

_Underlying type:_ _[SecretKeySelector](https://kubernetes.io/docs/reference/generated/kubernetes-api/v1.31/#secretkeyselector-v1-core)_

ObjectStorageConfig is the secret that contains the object storage configuration.
The secret needs to be in the same namespace as the ReceiveHashring object.
See https://thanos.io/tip/thanos/storage.md/#supported-clients for relevant documentation.



_Appears in:_
- [IngesterHashringSpec](#ingesterhashringspec)
- [IngesterSpec](#ingesterspec)
- [ThanosCompactSpec](#thanoscompactspec)
- [ThanosRulerSpec](#thanosrulerspec)
- [ThanosStoreSpec](#thanosstorespec)



#### QueryFrontendSpec



QueryFrontendSpec defines the desired state of ThanosQueryFrontend



_Appears in:_
- [ThanosQuerySpec](#thanosqueryspec)

| Field | Description | Default | Validation |
| --- | --- | --- | --- |
| `version` _string_ | Version of Thanos to be deployed.<br />If not specified, the operator assumes the latest upstream version of<br />Thanos available at the time when the version of the operator was released. |  | Optional: \{\} <br /> |
| `image` _string_ | Container image to use for the Thanos components. |  | Optional: \{\} <br /> |
| `imagePullPolicy` _[PullPolicy](https://kubernetes.io/docs/reference/generated/kubernetes-api/v1.31/#pullpolicy-v1-core)_ | Image pull policy for the Thanos containers.<br />See https://kubernetes.io/docs/concepts/containers/images/#image-pull-policy for more details. | IfNotPresent | Enum: [Always Never IfNotPresent] <br />Optional: \{\} <br /> |
| `imagePullSecrets` _[LocalObjectReference](https://kubernetes.io/docs/reference/generated/kubernetes-api/v1.31/#localobjectreference-v1-core) array_ | An optional list of references to Secrets in the same namespace<br />to use for pulling images from registries.<br />See http://kubernetes.io/docs/user-guide/images#specifying-imagepullsecrets-on-a-pod |  | Optional: \{\} <br /> |
| `resourceRequirements` _[ResourceRequirements](https://kubernetes.io/docs/reference/generated/kubernetes-api/v1.31/#resourcerequirements-v1-core)_ | ResourceRequirements for the Thanos component container. |  | Optional: \{\} <br /> |
| `logLevel` _string_ | Log level for Thanos. |  | Enum: [debug info warn error] <br />Optional: \{\} <br /> |
| `logFormat` _string_ | Log format for Thanos. | logfmt | Enum: [logfmt json] <br />Optional: \{\} <br /> |
| `replicas` _integer_ |  | 1 | Minimum: 1 <br /> |
| `compressResponses` _boolean_ | CompressResponses enables response compression | true |  |
| `queryLabelSelector` _[LabelSelector](https://kubernetes.io/docs/reference/generated/kubernetes-api/v1.31/#labelselector-v1-meta)_ | By default, the operator will add the first discoverable Query API to the<br />Query Frontend, if they have query labels. You can optionally choose to override default<br />Query selector labels, to select a subset of QueryAPIs to query. | \{ matchLabels:map[operator.thanos.io/query-api:true] \} | Optional: \{\} <br /> |
| `logQueriesLongerThan` _[Duration](#duration)_ | LogQueriesLongerThan sets the duration threshold for logging long queries |  | Optional: \{\} <br />Pattern: `^-?(0\|(([0-9]+)y)?(([0-9]+)w)?(([0-9]+)d)?(([0-9]+)h)?(([0-9]+)m)?(([0-9]+)s)?(([0-9]+)ms)?)$` <br /> |
| `queryRangeResponseCacheConfig` _[CacheConfig](#cacheconfig)_ | QueryRangeResponseCacheConfig holds the configuration for the query range response cache |  | Optional: \{\} <br /> |
| `queryRangeSplitInterval` _[Duration](#duration)_ | QueryRangeSplitInterval sets the split interval for query range |  | Optional: \{\} <br />Pattern: `^-?(0\|(([0-9]+)y)?(([0-9]+)w)?(([0-9]+)d)?(([0-9]+)h)?(([0-9]+)m)?(([0-9]+)s)?(([0-9]+)ms)?)$` <br /> |
| `labelsSplitInterval` _[Duration](#duration)_ | LabelsSplitInterval sets the split interval for labels |  | Optional: \{\} <br />Pattern: `^-?(0\|(([0-9]+)y)?(([0-9]+)w)?(([0-9]+)d)?(([0-9]+)h)?(([0-9]+)m)?(([0-9]+)s)?(([0-9]+)ms)?)$` <br /> |
| `queryRangeMaxRetries` _integer_ | QueryRangeMaxRetries sets the maximum number of retries for query range requests | 5 | Minimum: 0 <br /> |
| `labelsMaxRetries` _integer_ | LabelsMaxRetries sets the maximum number of retries for label requests | 5 | Minimum: 0 <br /> |
| `labelsDefaultTimeRange` _[Duration](#duration)_ | LabelsDefaultTimeRange sets the default time range for label queries |  | Optional: \{\} <br />Pattern: `^-?(0\|(([0-9]+)y)?(([0-9]+)w)?(([0-9]+)d)?(([0-9]+)h)?(([0-9]+)m)?(([0-9]+)s)?(([0-9]+)ms)?)$` <br /> |
| `additionalArgs` _string array_ | Additional arguments to pass to the Thanos components. |  | Optional: \{\} <br /> |
| `additionalContainers` _[Container](https://kubernetes.io/docs/reference/generated/kubernetes-api/v1.31/#container-v1-core) array_ | Additional containers to add to the Thanos components. |  | Optional: \{\} <br /> |
| `additionalVolumes` _[Volume](https://kubernetes.io/docs/reference/generated/kubernetes-api/v1.31/#volume-v1-core) array_ | Additional volumes to add to the Thanos components. |  | Optional: \{\} <br /> |
| `additionalVolumeMounts` _[VolumeMount](https://kubernetes.io/docs/reference/generated/kubernetes-api/v1.31/#volumemount-v1-core) array_ | Additional volume mounts to add to the Thanos component container in a Deployment or StatefulSet<br />controlled by the operator. |  | Optional: \{\} <br /> |
| `additionalPorts` _[ContainerPort](https://kubernetes.io/docs/reference/generated/kubernetes-api/v1.31/#containerport-v1-core) array_ | Additional ports to expose on the Thanos component container in a Deployment or StatefulSet<br />controlled by the operator. |  | Optional: \{\} <br /> |
| `additionalEnv` _[EnvVar](https://kubernetes.io/docs/reference/generated/kubernetes-api/v1.31/#envvar-v1-core) array_ | Additional environment variables to add to the Thanos component container in a Deployment or StatefulSet<br />controlled by the operator. |  | Optional: \{\} <br /> |
| `additionalServicePorts` _[ServicePort](https://kubernetes.io/docs/reference/generated/kubernetes-api/v1.31/#serviceport-v1-core) array_ | AdditionalServicePorts are additional ports to expose on the Service for the Thanos component. |  | Optional: \{\} <br /> |


#### RetentionResolutionConfig



RetentionResolutionConfig defines the retention configuration for the compact component.



_Appears in:_
- [ThanosCompactSpec](#thanoscompactspec)

| Field | Description | Default | Validation |
| --- | --- | --- | --- |
| `raw` _[Duration](#duration)_ | Raw is the retention configuration for the raw samples.<br />This configures how long to retain raw samples in the storage.<br />The default value is 0d, which means samples are retained indefinitely. | 0d | Pattern: `^-?(0\|(([0-9]+)y)?(([0-9]+)w)?(([0-9]+)d)?(([0-9]+)h)?(([0-9]+)m)?(([0-9]+)s)?(([0-9]+)ms)?)$` <br />Required: \{\} <br /> |
| `fiveMinutes` _[Duration](#duration)_ | FiveMinutes is the retention configuration for samples of resolution 1 (5 minutes).<br />This configures how long to retain samples of resolution 1 (5 minutes) in storage.<br />The default value is 0d, which means these samples are retained indefinitely. | 0d | Pattern: `^-?(0\|(([0-9]+)y)?(([0-9]+)w)?(([0-9]+)d)?(([0-9]+)h)?(([0-9]+)m)?(([0-9]+)s)?(([0-9]+)ms)?)$` <br />Required: \{\} <br /> |
| `oneHour` _[Duration](#duration)_ | OneHour is the retention configuration for samples of resolution 2 (1 hour).<br />This configures how long to retain samples of resolution 2 (1 hour) in storage.<br />The default value is 0d, which means these samples are retained indefinitely. | 0d | Pattern: `^-?(0\|(([0-9]+)y)?(([0-9]+)w)?(([0-9]+)d)?(([0-9]+)h)?(([0-9]+)m)?(([0-9]+)s)?(([0-9]+)ms)?)$` <br />Required: \{\} <br /> |


#### RouterSpec



RouterSpec represents the configuration for the router



_Appears in:_
- [ThanosReceiveSpec](#thanosreceivespec)

| Field | Description | Default | Validation |
| --- | --- | --- | --- |
| `version` _string_ | Version of Thanos to be deployed.<br />If not specified, the operator assumes the latest upstream version of<br />Thanos available at the time when the version of the operator was released. |  | Optional: \{\} <br /> |
| `image` _string_ | Container image to use for the Thanos components. |  | Optional: \{\} <br /> |
| `imagePullPolicy` _[PullPolicy](https://kubernetes.io/docs/reference/generated/kubernetes-api/v1.31/#pullpolicy-v1-core)_ | Image pull policy for the Thanos containers.<br />See https://kubernetes.io/docs/concepts/containers/images/#image-pull-policy for more details. | IfNotPresent | Enum: [Always Never IfNotPresent] <br />Optional: \{\} <br /> |
| `imagePullSecrets` _[LocalObjectReference](https://kubernetes.io/docs/reference/generated/kubernetes-api/v1.31/#localobjectreference-v1-core) array_ | An optional list of references to Secrets in the same namespace<br />to use for pulling images from registries.<br />See http://kubernetes.io/docs/user-guide/images#specifying-imagepullsecrets-on-a-pod |  | Optional: \{\} <br /> |
| `resourceRequirements` _[ResourceRequirements](https://kubernetes.io/docs/reference/generated/kubernetes-api/v1.31/#resourcerequirements-v1-core)_ | ResourceRequirements for the Thanos component container. |  | Optional: \{\} <br /> |
| `logLevel` _string_ | Log level for Thanos. |  | Enum: [debug info warn error] <br />Optional: \{\} <br /> |
| `logFormat` _string_ | Log format for Thanos. | logfmt | Enum: [logfmt json] <br />Optional: \{\} <br /> |
| `labels` _object (keys:string, values:string)_ | Labels are additional labels to add to the router components.<br />Labels set here will overwrite the labels inherited from the ThanosReceive object if they have the same key. |  | Optional: \{\} <br /> |
| `replicas` _integer_ | Replicas is the number of router replicas. | 1 | Minimum: 1 <br />Required: \{\} <br /> |
| `replicationFactor` _integer_ | ReplicationFactor is the replication factor for the router. | 1 | Enum: [1 3 5] <br />Required: \{\} <br /> |
| `externalLabels` _[ExternalLabels](#externallabels)_ | ExternalLabels set and forwarded by the router to the ingesters. | \{ receive:true \} | MinProperties: 1 <br />Required: \{\} <br /> |
| `additionalArgs` _string array_ | Additional arguments to pass to the Thanos components. |  | Optional: \{\} <br /> |
| `additionalContainers` _[Container](https://kubernetes.io/docs/reference/generated/kubernetes-api/v1.31/#container-v1-core) array_ | Additional containers to add to the Thanos components. |  | Optional: \{\} <br /> |
| `additionalVolumes` _[Volume](https://kubernetes.io/docs/reference/generated/kubernetes-api/v1.31/#volume-v1-core) array_ | Additional volumes to add to the Thanos components. |  | Optional: \{\} <br /> |
| `additionalVolumeMounts` _[VolumeMount](https://kubernetes.io/docs/reference/generated/kubernetes-api/v1.31/#volumemount-v1-core) array_ | Additional volume mounts to add to the Thanos component container in a Deployment or StatefulSet<br />controlled by the operator. |  | Optional: \{\} <br /> |
| `additionalPorts` _[ContainerPort](https://kubernetes.io/docs/reference/generated/kubernetes-api/v1.31/#containerport-v1-core) array_ | Additional ports to expose on the Thanos component container in a Deployment or StatefulSet<br />controlled by the operator. |  | Optional: \{\} <br /> |
| `additionalEnv` _[EnvVar](https://kubernetes.io/docs/reference/generated/kubernetes-api/v1.31/#envvar-v1-core) array_ | Additional environment variables to add to the Thanos component container in a Deployment or StatefulSet<br />controlled by the operator. |  | Optional: \{\} <br /> |
| `additionalServicePorts` _[ServicePort](https://kubernetes.io/docs/reference/generated/kubernetes-api/v1.31/#serviceport-v1-core) array_ | AdditionalServicePorts are additional ports to expose on the Service for the Thanos component. |  | Optional: \{\} <br /> |


#### ServiceMonitorConfig



ServiceMonitorConfig is the configuration for the ServiceMonitor.



_Appears in:_
- [FeatureGates](#featuregates)

| Field | Description | Default | Validation |
| --- | --- | --- | --- |
| `enable` _boolean_ | Enable the management of ServiceMonitors for the Thanos component.<br />If not specified, the operator will default to true. |  | Optional: \{\} <br /> |
| `labels` _object (keys:string, values:string)_ | Labels to add to the ServiceMonitor. |  | Optional: \{\} <br /> |


#### ShardingConfig



ShardingConfig defines the sharding configuration for the compact component.



_Appears in:_
- [ThanosCompactSpec](#thanoscompactspec)

| Field | Description | Default | Validation |
| --- | --- | --- | --- |
| `externalLabelSharding` _[ExternalLabelShardingConfig](#externallabelshardingconfig) array_ | ExternalLabelSharding is the sharding configuration based on explicit external labels and their values. |  | Optional: \{\} <br /> |


#### ShardingStrategy



ShardingStrategy controls the automatic deployment of multiple store gateways sharded by block ID
by hashmoding __block_id label value.



_Appears in:_
- [ThanosStoreSpec](#thanosstorespec)

| Field | Description | Default | Validation |
| --- | --- | --- | --- |
| `type` _[ShardingStrategyType](#shardingstrategytype)_ | Type here is the type of sharding strategy. | block | Enum: [block] <br />Required: \{\} <br /> |
| `shards` _integer_ | Shards is the number of shards to split the data into. | 1 | Minimum: 1 <br /> |


#### ShardingStrategyType

_Underlying type:_ _string_





_Appears in:_
- [ShardingStrategy](#shardingstrategy)

| Field | Description |
| --- | --- |
| `block` | Block is the block modulo sharding strategy for sharding Stores according to block ids.<br /> |


#### StorageSize

_Underlying type:_ _string_

StorageSize is the size of the PV storage to be used by a Thanos component.

_Validation:_
- Pattern: `^([+-]?[0-9.]+)([eEinumkKMGTP]*[-+]?[0-9]*)$`

_Appears in:_
- [InMemoryCacheConfig](#inmemorycacheconfig)
- [IngesterHashringSpec](#ingesterhashringspec)
- [ThanosCompactSpec](#thanoscompactspec)
- [ThanosStoreSpec](#thanosstorespec)



#### StoreLimitsOptions



StoreLimitsOptions is the configuration for the store API limits options.



_Appears in:_
- [IngesterHashringSpec](#ingesterhashringspec)
- [ThanosStoreSpec](#thanosstorespec)

| Field | Description | Default | Validation |
| --- | --- | --- | --- |
| `storeLimitsRequestSamples` _integer_ | StoreLimitsRequestSamples is the maximum samples allowed for a single StoreAPI Series request.<br />0 means no limit. | 0 |  |
| `storeLimitsRequestSeries` _integer_ | StoreLimitsRequestSeries is the maximum series allowed for a single StoreAPI Series request.<br />0 means no limit. | 0 |  |


#### TSDBConfig



TSDBConfig specifies configuration for any particular Thanos TSDB.
NOTE: Some of these options will not exist for all components, in which case, even if specified can be ignored.



_Appears in:_
- [IngesterHashringSpec](#ingesterhashringspec)

| Field | Description | Default | Validation |
| --- | --- | --- | --- |
| `retention` _[Duration](#duration)_ | Retention is the duration for which a particular TSDB will retain data. | 2h | Pattern: `^-?(0\|(([0-9]+)y)?(([0-9]+)w)?(([0-9]+)d)?(([0-9]+)h)?(([0-9]+)m)?(([0-9]+)s)?(([0-9]+)ms)?)$` <br />Required: \{\} <br /> |


#### TelemetryQuantiles



TelemetryQuantiles is the configuration for the request telemetry quantiles.
Float usage is discouraged by controller-runtime, so we use string instead.



_Appears in:_
- [ThanosQuerySpec](#thanosqueryspec)

| Field | Description | Default | Validation |
| --- | --- | --- | --- |
| `duration` _string array_ | Duration is the quantiles for exporting metrics about the request duration. |  | Optional: \{\} <br /> |
| `samples` _string array_ | Samples is the quantiles for exporting metrics about the samples count. |  | Optional: \{\} <br /> |
| `series` _string array_ | Series is the quantiles for exporting metrics about the series count. |  | Optional: \{\} <br /> |


#### TenancyConfig



TenancyConfig is the configuration for the tenancy options.



_Appears in:_
- [IngesterHashringSpec](#ingesterhashringspec)

| Field | Description | Default | Validation |
| --- | --- | --- | --- |
| `tenants` _string array_ | Tenants is a list of tenants that should be matched by the hashring.<br />An empty list matches all tenants. |  | Optional: \{\} <br /> |
| `tenantMatcherType` _string_ | TenantMatcherType is the type of tenant matching to use. | exact | Enum: [exact glob] <br /> |
| `tenantHeader` _string_ | TenantHeader is the HTTP header to determine tenant for write requests. | THANOS-TENANT |  |
| `tenantCertificateField` _string_ | TenantCertificateField is the TLS client's certificate field to determine tenant for write requests. |  | Enum: [organization organizationalUnit commonName] <br />Optional: \{\} <br /> |
| `defaultTenantID` _string_ | DefaultTenantID is the default tenant ID to use when none is provided via a header. | default-tenant |  |
| `splitTenantLabelName` _string_ | SplitTenantLabelName is the label name through which the request will be split into multiple tenants. |  | Optional: \{\} <br /> |
| `tenantLabelName` _string_ | TenantLabelName is the label name through which the tenant will be announced. | tenant_id |  |


#### ThanosCompact



ThanosCompact is the Schema for the thanoscompacts API



_Appears in:_
- [ThanosCompactList](#thanoscompactlist)

| Field | Description | Default | Validation |
| --- | --- | --- | --- |
| `apiVersion` _string_ | `monitoring.thanos.io/v1alpha1` | | |
| `kind` _string_ | `ThanosCompact` | | |
| `kind` _string_ | Kind is a string value representing the REST resource this object represents.<br />Servers may infer this from the endpoint the client submits requests to.<br />Cannot be updated.<br />In CamelCase.<br />More info: https://git.k8s.io/community/contributors/devel/sig-architecture/api-conventions.md#types-kinds |  |  |
| `apiVersion` _string_ | APIVersion defines the versioned schema of this representation of an object.<br />Servers should convert recognized schemas to the latest internal value, and<br />may reject unrecognized values.<br />More info: https://git.k8s.io/community/contributors/devel/sig-architecture/api-conventions.md#resources |  |  |
| `metadata` _[ObjectMeta](https://kubernetes.io/docs/reference/generated/kubernetes-api/v1.31/#objectmeta-v1-meta)_ | Refer to Kubernetes API documentation for fields of `metadata`. |  |  |
| `spec` _[ThanosCompactSpec](#thanoscompactspec)_ |  |  |  |
| `status` _[ThanosCompactStatus](#thanoscompactstatus)_ |  |  |  |


#### ThanosCompactList



ThanosCompactList contains a list of ThanosCompact





| Field | Description | Default | Validation |
| --- | --- | --- | --- |
| `apiVersion` _string_ | `monitoring.thanos.io/v1alpha1` | | |
| `kind` _string_ | `ThanosCompactList` | | |
| `kind` _string_ | Kind is a string value representing the REST resource this object represents.<br />Servers may infer this from the endpoint the client submits requests to.<br />Cannot be updated.<br />In CamelCase.<br />More info: https://git.k8s.io/community/contributors/devel/sig-architecture/api-conventions.md#types-kinds |  |  |
| `apiVersion` _string_ | APIVersion defines the versioned schema of this representation of an object.<br />Servers should convert recognized schemas to the latest internal value, and<br />may reject unrecognized values.<br />More info: https://git.k8s.io/community/contributors/devel/sig-architecture/api-conventions.md#resources |  |  |
| `metadata` _[ListMeta](https://kubernetes.io/docs/reference/generated/kubernetes-api/v1.31/#listmeta-v1-meta)_ | Refer to Kubernetes API documentation for fields of `metadata`. |  |  |
| `items` _[ThanosCompact](#thanoscompact) array_ |  |  |  |


#### ThanosCompactSpec



ThanosCompactSpec defines the desired state of ThanosCompact



_Appears in:_
- [ThanosCompact](#thanoscompact)

| Field | Description | Default | Validation |
| --- | --- | --- | --- |
| `version` _string_ | Version of Thanos to be deployed.<br />If not specified, the operator assumes the latest upstream version of<br />Thanos available at the time when the version of the operator was released. |  | Optional: \{\} <br /> |
| `image` _string_ | Container image to use for the Thanos components. |  | Optional: \{\} <br /> |
| `imagePullPolicy` _[PullPolicy](https://kubernetes.io/docs/reference/generated/kubernetes-api/v1.31/#pullpolicy-v1-core)_ | Image pull policy for the Thanos containers.<br />See https://kubernetes.io/docs/concepts/containers/images/#image-pull-policy for more details. | IfNotPresent | Enum: [Always Never IfNotPresent] <br />Optional: \{\} <br /> |
| `imagePullSecrets` _[LocalObjectReference](https://kubernetes.io/docs/reference/generated/kubernetes-api/v1.31/#localobjectreference-v1-core) array_ | An optional list of references to Secrets in the same namespace<br />to use for pulling images from registries.<br />See http://kubernetes.io/docs/user-guide/images#specifying-imagepullsecrets-on-a-pod |  | Optional: \{\} <br /> |
| `resourceRequirements` _[ResourceRequirements](https://kubernetes.io/docs/reference/generated/kubernetes-api/v1.31/#resourcerequirements-v1-core)_ | ResourceRequirements for the Thanos component container. |  | Optional: \{\} <br /> |
| `logLevel` _string_ | Log level for Thanos. |  | Enum: [debug info warn error] <br />Optional: \{\} <br /> |
| `logFormat` _string_ | Log format for Thanos. | logfmt | Enum: [logfmt json] <br />Optional: \{\} <br /> |
| `labels` _object (keys:string, values:string)_ | Labels are additional labels to add to the Compact component. |  | Optional: \{\} <br /> |
| `objectStorageConfig` _[ObjectStorageConfig](#objectstorageconfig)_ | ObjectStorageConfig is the object storage configuration for the compact component. |  | Required: \{\} <br /> |
| `storageSize` _[StorageSize](#storagesize)_ | StorageSize is the size of the storage to be used by the Thanos Compact StatefulSets. |  | Pattern: `^([+-]?[0-9.]+)([eEinumkKMGTP]*[-+]?[0-9]*)$` <br />Required: \{\} <br /> |
| `retentionConfig` _[RetentionResolutionConfig](#retentionresolutionconfig)_ | RetentionConfig is the retention configuration for the compact component. |  | Required: \{\} <br /> |
| `blockConfig` _[BlockConfig](#blockconfig)_ | BlockConfig defines settings for block handling. |  | Optional: \{\} <br /> |
| `blockViewerGlobalSync` _[BlockViewerGlobalSyncConfig](#blockviewerglobalsyncconfig)_ | BlockViewerGlobalSync is the configuration for syncing the blocks between local and remote view for /global Block Viewer UI. |  | Optional: \{\} <br /> |
| `shardingConfig` _[ShardingConfig](#shardingconfig)_ | ShardingConfig is the sharding configuration for the compact component. |  | Optional: \{\} <br /> |
| `compactConfig` _[CompactConfig](#compactconfig)_ | CompactConfig is the configuration for the compact component. |  | Optional: \{\} <br /> |
| `downsamplingConfig` _[DownsamplingConfig](#downsamplingconfig)_ | DownsamplingConfig is the downsampling configuration for the compact component. |  | Optional: \{\} <br /> |
| `debugConfig` _[DebugConfig](#debugconfig)_ | DebugConfig is the debug configuration for the compact component. |  | Optional: \{\} <br /> |
| `minTime` _[Duration](#duration)_ | Minimum time range to serve. Any data earlier than this lower time range will be ignored.<br />If not set, will be set as zero value, so most recent blocks will be served. |  | Optional: \{\} <br />Pattern: `^-?(0\|(([0-9]+)y)?(([0-9]+)w)?(([0-9]+)d)?(([0-9]+)h)?(([0-9]+)m)?(([0-9]+)s)?(([0-9]+)ms)?)$` <br /> |
| `maxTime` _[Duration](#duration)_ | Maximum time range to serve. Any data after this upper time range will be ignored.<br />If not set, will be set as max value, so all blocks will be served. |  | Optional: \{\} <br />Pattern: `^-?(0\|(([0-9]+)y)?(([0-9]+)w)?(([0-9]+)d)?(([0-9]+)h)?(([0-9]+)m)?(([0-9]+)s)?(([0-9]+)ms)?)$` <br /> |
| `paused` _boolean_ | When a resource is paused, no actions except for deletion<br />will be performed on the underlying objects. |  | Optional: \{\} <br /> |
| `featureGates` _[FeatureGates](#featuregates)_ | FeatureGates are feature gates for the compact component. | \{ serviceMonitor:map[enable:true] \} | Optional: \{\} <br /> |
| `additionalArgs` _string array_ | Additional arguments to pass to the Thanos components. |  | Optional: \{\} <br /> |
| `additionalContainers` _[Container](https://kubernetes.io/docs/reference/generated/kubernetes-api/v1.31/#container-v1-core) array_ | Additional containers to add to the Thanos components. |  | Optional: \{\} <br /> |
| `additionalVolumes` _[Volume](https://kubernetes.io/docs/reference/generated/kubernetes-api/v1.31/#volume-v1-core) array_ | Additional volumes to add to the Thanos components. |  | Optional: \{\} <br /> |
| `additionalVolumeMounts` _[VolumeMount](https://kubernetes.io/docs/reference/generated/kubernetes-api/v1.31/#volumemount-v1-core) array_ | Additional volume mounts to add to the Thanos component container in a Deployment or StatefulSet<br />controlled by the operator. |  | Optional: \{\} <br /> |
| `additionalPorts` _[ContainerPort](https://kubernetes.io/docs/reference/generated/kubernetes-api/v1.31/#containerport-v1-core) array_ | Additional ports to expose on the Thanos component container in a Deployment or StatefulSet<br />controlled by the operator. |  | Optional: \{\} <br /> |
| `additionalEnv` _[EnvVar](https://kubernetes.io/docs/reference/generated/kubernetes-api/v1.31/#envvar-v1-core) array_ | Additional environment variables to add to the Thanos component container in a Deployment or StatefulSet<br />controlled by the operator. |  | Optional: \{\} <br /> |
| `additionalServicePorts` _[ServicePort](https://kubernetes.io/docs/reference/generated/kubernetes-api/v1.31/#serviceport-v1-core) array_ | AdditionalServicePorts are additional ports to expose on the Service for the Thanos component. |  | Optional: \{\} <br /> |


#### ThanosCompactStatus



ThanosCompactStatus defines the observed state of ThanosCompact



_Appears in:_
- [ThanosCompact](#thanoscompact)

| Field | Description | Default | Validation |
| --- | --- | --- | --- |
| `conditions` _[Condition](https://kubernetes.io/docs/reference/generated/kubernetes-api/v1.31/#condition-v1-meta) array_ | Conditions represent the latest available observations of the state of the hashring. |  |  |


#### ThanosQuery



ThanosQuery is the Schema for the thanosqueries API



_Appears in:_
- [ThanosQueryList](#thanosquerylist)

| Field | Description | Default | Validation |
| --- | --- | --- | --- |
| `apiVersion` _string_ | `monitoring.thanos.io/v1alpha1` | | |
| `kind` _string_ | `ThanosQuery` | | |
| `kind` _string_ | Kind is a string value representing the REST resource this object represents.<br />Servers may infer this from the endpoint the client submits requests to.<br />Cannot be updated.<br />In CamelCase.<br />More info: https://git.k8s.io/community/contributors/devel/sig-architecture/api-conventions.md#types-kinds |  |  |
| `apiVersion` _string_ | APIVersion defines the versioned schema of this representation of an object.<br />Servers should convert recognized schemas to the latest internal value, and<br />may reject unrecognized values.<br />More info: https://git.k8s.io/community/contributors/devel/sig-architecture/api-conventions.md#resources |  |  |
| `metadata` _[ObjectMeta](https://kubernetes.io/docs/reference/generated/kubernetes-api/v1.31/#objectmeta-v1-meta)_ | Refer to Kubernetes API documentation for fields of `metadata`. |  |  |
| `spec` _[ThanosQuerySpec](#thanosqueryspec)_ |  |  |  |
| `status` _[ThanosQueryStatus](#thanosquerystatus)_ |  |  |  |


#### ThanosQueryList



ThanosQueryList contains a list of ThanosQuery





| Field | Description | Default | Validation |
| --- | --- | --- | --- |
| `apiVersion` _string_ | `monitoring.thanos.io/v1alpha1` | | |
| `kind` _string_ | `ThanosQueryList` | | |
| `kind` _string_ | Kind is a string value representing the REST resource this object represents.<br />Servers may infer this from the endpoint the client submits requests to.<br />Cannot be updated.<br />In CamelCase.<br />More info: https://git.k8s.io/community/contributors/devel/sig-architecture/api-conventions.md#types-kinds |  |  |
| `apiVersion` _string_ | APIVersion defines the versioned schema of this representation of an object.<br />Servers should convert recognized schemas to the latest internal value, and<br />may reject unrecognized values.<br />More info: https://git.k8s.io/community/contributors/devel/sig-architecture/api-conventions.md#resources |  |  |
| `metadata` _[ListMeta](https://kubernetes.io/docs/reference/generated/kubernetes-api/v1.31/#listmeta-v1-meta)_ | Refer to Kubernetes API documentation for fields of `metadata`. |  |  |
| `items` _[ThanosQuery](#thanosquery) array_ |  |  |  |


#### ThanosQuerySpec



ThanosQuerySpec defines the desired state of ThanosQuery



_Appears in:_
- [ThanosQuery](#thanosquery)

| Field | Description | Default | Validation |
| --- | --- | --- | --- |
| `version` _string_ | Version of Thanos to be deployed.<br />If not specified, the operator assumes the latest upstream version of<br />Thanos available at the time when the version of the operator was released. |  | Optional: \{\} <br /> |
| `image` _string_ | Container image to use for the Thanos components. |  | Optional: \{\} <br /> |
| `imagePullPolicy` _[PullPolicy](https://kubernetes.io/docs/reference/generated/kubernetes-api/v1.31/#pullpolicy-v1-core)_ | Image pull policy for the Thanos containers.<br />See https://kubernetes.io/docs/concepts/containers/images/#image-pull-policy for more details. | IfNotPresent | Enum: [Always Never IfNotPresent] <br />Optional: \{\} <br /> |
| `imagePullSecrets` _[LocalObjectReference](https://kubernetes.io/docs/reference/generated/kubernetes-api/v1.31/#localobjectreference-v1-core) array_ | An optional list of references to Secrets in the same namespace<br />to use for pulling images from registries.<br />See http://kubernetes.io/docs/user-guide/images#specifying-imagepullsecrets-on-a-pod |  | Optional: \{\} <br /> |
| `resourceRequirements` _[ResourceRequirements](https://kubernetes.io/docs/reference/generated/kubernetes-api/v1.31/#resourcerequirements-v1-core)_ | ResourceRequirements for the Thanos component container. |  | Optional: \{\} <br /> |
| `logLevel` _string_ | Log level for Thanos. |  | Enum: [debug info warn error] <br />Optional: \{\} <br /> |
| `logFormat` _string_ | Log format for Thanos. | logfmt | Enum: [logfmt json] <br />Optional: \{\} <br /> |
| `replicas` _integer_ | Replicas is the number of querier replicas. | 1 | Minimum: 1 <br />Required: \{\} <br /> |
| `labels` _object (keys:string, values:string)_ | Labels are additional labels to add to the Querier component. |  | Optional: \{\} <br /> |
| `replicaLabels` _string array_ | ReplicaLabels are labels to treat as a replica indicator along which data is deduplicated.<br />Data can still be queried without deduplication using 'dedup=false' parameter.<br />Data includes time series, recording rules, and alerting rules.<br />Refer to https://thanos.io/tip/components/query.md/#deduplication-replica-labels | [replica] | Optional: \{\} <br /> |
| `customStoreLabelSelector` _[LabelSelector](https://kubernetes.io/docs/reference/generated/kubernetes-api/v1.31/#labelselector-v1-meta)_ | StoreLabelSelector enables adding additional labels to build a custom label selector<br />for discoverable StoreAPIs. Values provided here will be appended to the default which are<br />\{"operator.thanos.io/store-api": "true", "app.kubernetes.io/part-of": "thanos"\}. |  | Optional: \{\} <br /> |
| `telemetryQuantiles` _[TelemetryQuantiles](#telemetryquantiles)_ | TelemetryQuantiles is the configuration for the request telemetry quantiles. |  | Optional: \{\} <br /> |
| `webConfig` _[WebConfig](#webconfig)_ | WebConfig is the configuration for the Query UI and API web options. |  | Optional: \{\} <br /> |
| `grpcProxyStrategy` _string_ | GRPCProxyStrategy is the strategy to use when proxying Series requests to leaf nodes. | eager | Enum: [eager lazy] <br /> |
| `queryFrontend` _[QueryFrontendSpec](#queryfrontendspec)_ | QueryFrontend is the configuration for the Query Frontend<br />If you specify this, the operator will create a Query Frontend in front of your query deployment. |  | Optional: \{\} <br /> |
| `paused` _boolean_ | When a resource is paused, no actions except for deletion<br />will be performed on the underlying objects. |  | Optional: \{\} <br /> |
| `featureGates` _[FeatureGates](#featuregates)_ | FeatureGates are feature gates for the compact component. | \{ serviceMonitor:map[enable:true] \} | Optional: \{\} <br /> |
| `additionalArgs` _string array_ | Additional arguments to pass to the Thanos components. |  | Optional: \{\} <br /> |
| `additionalContainers` _[Container](https://kubernetes.io/docs/reference/generated/kubernetes-api/v1.31/#container-v1-core) array_ | Additional containers to add to the Thanos components. |  | Optional: \{\} <br /> |
| `additionalVolumes` _[Volume](https://kubernetes.io/docs/reference/generated/kubernetes-api/v1.31/#volume-v1-core) array_ | Additional volumes to add to the Thanos components. |  | Optional: \{\} <br /> |
| `additionalVolumeMounts` _[VolumeMount](https://kubernetes.io/docs/reference/generated/kubernetes-api/v1.31/#volumemount-v1-core) array_ | Additional volume mounts to add to the Thanos component container in a Deployment or StatefulSet<br />controlled by the operator. |  | Optional: \{\} <br /> |
| `additionalPorts` _[ContainerPort](https://kubernetes.io/docs/reference/generated/kubernetes-api/v1.31/#containerport-v1-core) array_ | Additional ports to expose on the Thanos component container in a Deployment or StatefulSet<br />controlled by the operator. |  | Optional: \{\} <br /> |
| `additionalEnv` _[EnvVar](https://kubernetes.io/docs/reference/generated/kubernetes-api/v1.31/#envvar-v1-core) array_ | Additional environment variables to add to the Thanos component container in a Deployment or StatefulSet<br />controlled by the operator. |  | Optional: \{\} <br /> |
| `additionalServicePorts` _[ServicePort](https://kubernetes.io/docs/reference/generated/kubernetes-api/v1.31/#serviceport-v1-core) array_ | AdditionalServicePorts are additional ports to expose on the Service for the Thanos component. |  | Optional: \{\} <br /> |


#### ThanosQueryStatus



ThanosQueryStatus defines the observed state of ThanosQuery



_Appears in:_
- [ThanosQuery](#thanosquery)

| Field | Description | Default | Validation |
| --- | --- | --- | --- |
| `conditions` _[Condition](https://kubernetes.io/docs/reference/generated/kubernetes-api/v1.31/#condition-v1-meta) array_ | Conditions represent the latest available observations of the state of the Querier. |  |  |


#### ThanosReceive



ThanosReceive is the Schema for the thanosreceives API



_Appears in:_
- [ThanosReceiveList](#thanosreceivelist)

| Field | Description | Default | Validation |
| --- | --- | --- | --- |
| `apiVersion` _string_ | `monitoring.thanos.io/v1alpha1` | | |
| `kind` _string_ | `ThanosReceive` | | |
| `kind` _string_ | Kind is a string value representing the REST resource this object represents.<br />Servers may infer this from the endpoint the client submits requests to.<br />Cannot be updated.<br />In CamelCase.<br />More info: https://git.k8s.io/community/contributors/devel/sig-architecture/api-conventions.md#types-kinds |  |  |
| `apiVersion` _string_ | APIVersion defines the versioned schema of this representation of an object.<br />Servers should convert recognized schemas to the latest internal value, and<br />may reject unrecognized values.<br />More info: https://git.k8s.io/community/contributors/devel/sig-architecture/api-conventions.md#resources |  |  |
| `metadata` _[ObjectMeta](https://kubernetes.io/docs/reference/generated/kubernetes-api/v1.31/#objectmeta-v1-meta)_ | Refer to Kubernetes API documentation for fields of `metadata`. |  |  |
| `spec` _[ThanosReceiveSpec](#thanosreceivespec)_ | Spec defines the desired state of ThanosReceive |  |  |
| `status` _[ThanosReceiveStatus](#thanosreceivestatus)_ | Status defines the observed state of ThanosReceive |  |  |


#### ThanosReceiveList



ThanosReceiveList contains a list of ThanosReceive





| Field | Description | Default | Validation |
| --- | --- | --- | --- |
| `apiVersion` _string_ | `monitoring.thanos.io/v1alpha1` | | |
| `kind` _string_ | `ThanosReceiveList` | | |
| `kind` _string_ | Kind is a string value representing the REST resource this object represents.<br />Servers may infer this from the endpoint the client submits requests to.<br />Cannot be updated.<br />In CamelCase.<br />More info: https://git.k8s.io/community/contributors/devel/sig-architecture/api-conventions.md#types-kinds |  |  |
| `apiVersion` _string_ | APIVersion defines the versioned schema of this representation of an object.<br />Servers should convert recognized schemas to the latest internal value, and<br />may reject unrecognized values.<br />More info: https://git.k8s.io/community/contributors/devel/sig-architecture/api-conventions.md#resources |  |  |
| `metadata` _[ListMeta](https://kubernetes.io/docs/reference/generated/kubernetes-api/v1.31/#listmeta-v1-meta)_ | Refer to Kubernetes API documentation for fields of `metadata`. |  |  |
| `items` _[ThanosReceive](#thanosreceive) array_ |  |  |  |


#### ThanosReceiveSpec



ThanosReceiveSpec defines the desired state of ThanosReceive



_Appears in:_
- [ThanosReceive](#thanosreceive)

| Field | Description | Default | Validation |
| --- | --- | --- | --- |
| `routerSpec` _[RouterSpec](#routerspec)_ | Router is the configuration for the router. |  | Required: \{\} <br /> |
| `ingesterSpec` _[IngesterSpec](#ingesterspec)_ | Ingester is the configuration for the ingestor. |  | Required: \{\} <br /> |
| `paused` _boolean_ | When a resource is paused, no actions except for deletion<br />will be performed on the underlying objects. |  | Optional: \{\} <br /> |
| `featureGates` _[FeatureGates](#featuregates)_ | FeatureGates are feature gates for the compact component. | \{ serviceMonitor:map[enable:true] \} | Optional: \{\} <br /> |


#### ThanosReceiveStatus



ThanosReceiveStatus defines the observed state of ThanosReceive



_Appears in:_
- [ThanosReceive](#thanosreceive)

| Field | Description | Default | Validation |
| --- | --- | --- | --- |
| `conditions` _[Condition](https://kubernetes.io/docs/reference/generated/kubernetes-api/v1.31/#condition-v1-meta) array_ | Conditions represent the latest available observations of the state of the hashring. |  |  |


#### ThanosRuler



ThanosRuler is the Schema for the thanosrulers API



_Appears in:_
- [ThanosRulerList](#thanosrulerlist)

| Field | Description | Default | Validation |
| --- | --- | --- | --- |
| `apiVersion` _string_ | `monitoring.thanos.io/v1alpha1` | | |
| `kind` _string_ | `ThanosRuler` | | |
| `kind` _string_ | Kind is a string value representing the REST resource this object represents.<br />Servers may infer this from the endpoint the client submits requests to.<br />Cannot be updated.<br />In CamelCase.<br />More info: https://git.k8s.io/community/contributors/devel/sig-architecture/api-conventions.md#types-kinds |  |  |
| `apiVersion` _string_ | APIVersion defines the versioned schema of this representation of an object.<br />Servers should convert recognized schemas to the latest internal value, and<br />may reject unrecognized values.<br />More info: https://git.k8s.io/community/contributors/devel/sig-architecture/api-conventions.md#resources |  |  |
| `metadata` _[ObjectMeta](https://kubernetes.io/docs/reference/generated/kubernetes-api/v1.31/#objectmeta-v1-meta)_ | Refer to Kubernetes API documentation for fields of `metadata`. |  |  |
| `spec` _[ThanosRulerSpec](#thanosrulerspec)_ |  |  |  |
| `status` _[ThanosRulerStatus](#thanosrulerstatus)_ |  |  |  |


#### ThanosRulerList



ThanosRulerList contains a list of ThanosRuler





| Field | Description | Default | Validation |
| --- | --- | --- | --- |
| `apiVersion` _string_ | `monitoring.thanos.io/v1alpha1` | | |
| `kind` _string_ | `ThanosRulerList` | | |
| `kind` _string_ | Kind is a string value representing the REST resource this object represents.<br />Servers may infer this from the endpoint the client submits requests to.<br />Cannot be updated.<br />In CamelCase.<br />More info: https://git.k8s.io/community/contributors/devel/sig-architecture/api-conventions.md#types-kinds |  |  |
| `apiVersion` _string_ | APIVersion defines the versioned schema of this representation of an object.<br />Servers should convert recognized schemas to the latest internal value, and<br />may reject unrecognized values.<br />More info: https://git.k8s.io/community/contributors/devel/sig-architecture/api-conventions.md#resources |  |  |
| `metadata` _[ListMeta](https://kubernetes.io/docs/reference/generated/kubernetes-api/v1.31/#listmeta-v1-meta)_ | Refer to Kubernetes API documentation for fields of `metadata`. |  |  |
| `items` _[ThanosRuler](#thanosruler) array_ |  |  |  |


#### ThanosRulerSpec



ThanosRulerSpec defines the desired state of ThanosRuler



_Appears in:_
- [ThanosRuler](#thanosruler)

| Field | Description | Default | Validation |
| --- | --- | --- | --- |
| `version` _string_ | Version of Thanos to be deployed.<br />If not specified, the operator assumes the latest upstream version of<br />Thanos available at the time when the version of the operator was released. |  | Optional: \{\} <br /> |
| `image` _string_ | Container image to use for the Thanos components. |  | Optional: \{\} <br /> |
| `imagePullPolicy` _[PullPolicy](https://kubernetes.io/docs/reference/generated/kubernetes-api/v1.31/#pullpolicy-v1-core)_ | Image pull policy for the Thanos containers.<br />See https://kubernetes.io/docs/concepts/containers/images/#image-pull-policy for more details. | IfNotPresent | Enum: [Always Never IfNotPresent] <br />Optional: \{\} <br /> |
| `imagePullSecrets` _[LocalObjectReference](https://kubernetes.io/docs/reference/generated/kubernetes-api/v1.31/#localobjectreference-v1-core) array_ | An optional list of references to Secrets in the same namespace<br />to use for pulling images from registries.<br />See http://kubernetes.io/docs/user-guide/images#specifying-imagepullsecrets-on-a-pod |  | Optional: \{\} <br /> |
| `resourceRequirements` _[ResourceRequirements](https://kubernetes.io/docs/reference/generated/kubernetes-api/v1.31/#resourcerequirements-v1-core)_ | ResourceRequirements for the Thanos component container. |  | Optional: \{\} <br /> |
| `logLevel` _string_ | Log level for Thanos. |  | Enum: [debug info warn error] <br />Optional: \{\} <br /> |
| `logFormat` _string_ | Log format for Thanos. | logfmt | Enum: [logfmt json] <br />Optional: \{\} <br /> |
| `labels` _object (keys:string, values:string)_ | Labels are additional labels to add to the Ruler component. |  | Optional: \{\} <br /> |
| `replicas` _integer_ | Replicas is the number of Ruler replicas. | 1 | Minimum: 1 <br />Required: \{\} <br /> |
| `queryLabelSelector` _[LabelSelector](https://kubernetes.io/docs/reference/generated/kubernetes-api/v1.31/#labelselector-v1-meta)_ | QueryLabelSelector is the label selector to discover Queriers.<br />It enables adding additional labels to build a custom label selector for discoverable QueryAPIs.<br />Values provided here will be appended to the default which are:<br />\{"operator.thanos.io/query-api": "true", "app.kubernetes.io/part-of": "thanos"\}. |  | Optional: \{\} <br /> |
| `defaultObjectStorageConfig` _[ObjectStorageConfig](#objectstorageconfig)_ | ObjectStorageConfig is the secret that contains the object storage configuration for Ruler to upload blocks. |  | Required: \{\} <br /> |
| `ruleConfigSelector` _[LabelSelector](https://kubernetes.io/docs/reference/generated/kubernetes-api/v1.31/#labelselector-v1-meta)_ | RuleConfigSelector is the label selector to discover ConfigMaps with rule files.<br />It enables adding additional labels to build a custom label selector for discoverable rule files.<br />Values provided here will be appended to the default which is:<br />\{"operator.thanos.io/rule-file": "true"\}. |  |  |
| `alertmanagerURL` _string_ | AlertmanagerURL is the URL of the Alertmanager to which the Ruler will send alerts.<br />The scheme should not be empty e.g http might be used. The scheme may be prefixed with<br />'dns+' or 'dnssrv+' to detect Alertmanager IPs through respective DNS lookups. |  | Pattern: `^((dns\+)?(dnssrv\+)?(http\|https):\/\/)[a-zA-Z0-9\-\.]+\.[a-zA-Z]\{2,\}(:[0-9]\{1,5\})?$` <br />Required: \{\} <br /> |
| `externalLabels` _[ExternalLabels](#externallabels)_ | ExternalLabels set on Ruler TSDB, for query time deduplication. | \{ rule_replica:$(NAME) \} | MinProperties: 1 <br />Required: \{\} <br /> |
| `evaluationInterval` _[Duration](#duration)_ | EvaluationInterval is the default interval at which rules are evaluated. | 1m | Pattern: `^-?(0\|(([0-9]+)y)?(([0-9]+)w)?(([0-9]+)d)?(([0-9]+)h)?(([0-9]+)m)?(([0-9]+)s)?(([0-9]+)ms)?)$` <br /> |
| `alertLabelDrop` _string array_ | Labels to drop before Ruler sends alerts to alertmanager. |  | Optional: \{\} <br /> |
| `retention` _[Duration](#duration)_ | Retention is the duration for which the Thanos Rule StatefulSet will retain data. | 2h | Pattern: `^-?(0\|(([0-9]+)y)?(([0-9]+)w)?(([0-9]+)d)?(([0-9]+)h)?(([0-9]+)m)?(([0-9]+)s)?(([0-9]+)ms)?)$` <br />Required: \{\} <br /> |
| `storageSize` _string_ | StorageSize is the size of the storage to be used by the Thanos Ruler StatefulSet. |  | Pattern: `^([+-]?[0-9.]+)([eEinumkKMGTP]*[-+]?[0-9]*)$` <br />Required: \{\} <br /> |
| `paused` _boolean_ | When a resource is paused, no actions except for deletion<br />will be performed on the underlying objects. |  | Optional: \{\} <br /> |
| `featureGates` _[FeatureGates](#featuregates)_ | FeatureGates are feature gates for the rule component. | \{ prometheusRuleEnabled:true serviceMonitor:map[enable:true] \} | Optional: \{\} <br /> |
| `prometheusRuleSelector` _[LabelSelector](https://kubernetes.io/docs/reference/generated/kubernetes-api/v1.31/#labelselector-v1-meta)_ | PrometheusRuleSelector is the label selector to discover PrometheusRule CRDs.<br />Once detected, these rules are made into configmaps and added to the Ruler. | \{ matchLabels:map[operator.thanos.io/prometheus-rule:true] \} | Required: \{\} <br /> |
| `additionalArgs` _string array_ | Additional arguments to pass to the Thanos components. |  | Optional: \{\} <br /> |
| `additionalContainers` _[Container](https://kubernetes.io/docs/reference/generated/kubernetes-api/v1.31/#container-v1-core) array_ | Additional containers to add to the Thanos components. |  | Optional: \{\} <br /> |
| `additionalVolumes` _[Volume](https://kubernetes.io/docs/reference/generated/kubernetes-api/v1.31/#volume-v1-core) array_ | Additional volumes to add to the Thanos components. |  | Optional: \{\} <br /> |
| `additionalVolumeMounts` _[VolumeMount](https://kubernetes.io/docs/reference/generated/kubernetes-api/v1.31/#volumemount-v1-core) array_ | Additional volume mounts to add to the Thanos component container in a Deployment or StatefulSet<br />controlled by the operator. |  | Optional: \{\} <br /> |
| `additionalPorts` _[ContainerPort](https://kubernetes.io/docs/reference/generated/kubernetes-api/v1.31/#containerport-v1-core) array_ | Additional ports to expose on the Thanos component container in a Deployment or StatefulSet<br />controlled by the operator. |  | Optional: \{\} <br /> |
| `additionalEnv` _[EnvVar](https://kubernetes.io/docs/reference/generated/kubernetes-api/v1.31/#envvar-v1-core) array_ | Additional environment variables to add to the Thanos component container in a Deployment or StatefulSet<br />controlled by the operator. |  | Optional: \{\} <br /> |
| `additionalServicePorts` _[ServicePort](https://kubernetes.io/docs/reference/generated/kubernetes-api/v1.31/#serviceport-v1-core) array_ | AdditionalServicePorts are additional ports to expose on the Service for the Thanos component. |  | Optional: \{\} <br /> |


#### ThanosRulerStatus



ThanosRulerStatus defines the observed state of ThanosRuler



_Appears in:_
- [ThanosRuler](#thanosruler)

| Field | Description | Default | Validation |
| --- | --- | --- | --- |
| `conditions` _[Condition](https://kubernetes.io/docs/reference/generated/kubernetes-api/v1.31/#condition-v1-meta) array_ | Conditions represent the latest available observations of the state of the Ruler. |  |  |


#### ThanosStore



ThanosStore is the Schema for the thanosstores API



_Appears in:_
- [ThanosStoreList](#thanosstorelist)

| Field | Description | Default | Validation |
| --- | --- | --- | --- |
| `apiVersion` _string_ | `monitoring.thanos.io/v1alpha1` | | |
| `kind` _string_ | `ThanosStore` | | |
| `kind` _string_ | Kind is a string value representing the REST resource this object represents.<br />Servers may infer this from the endpoint the client submits requests to.<br />Cannot be updated.<br />In CamelCase.<br />More info: https://git.k8s.io/community/contributors/devel/sig-architecture/api-conventions.md#types-kinds |  |  |
| `apiVersion` _string_ | APIVersion defines the versioned schema of this representation of an object.<br />Servers should convert recognized schemas to the latest internal value, and<br />may reject unrecognized values.<br />More info: https://git.k8s.io/community/contributors/devel/sig-architecture/api-conventions.md#resources |  |  |
| `metadata` _[ObjectMeta](https://kubernetes.io/docs/reference/generated/kubernetes-api/v1.31/#objectmeta-v1-meta)_ | Refer to Kubernetes API documentation for fields of `metadata`. |  |  |
| `spec` _[ThanosStoreSpec](#thanosstorespec)_ |  |  |  |
| `status` _[ThanosStoreStatus](#thanosstorestatus)_ |  |  |  |


#### ThanosStoreList



ThanosStoreList contains a list of ThanosStore





| Field | Description | Default | Validation |
| --- | --- | --- | --- |
| `apiVersion` _string_ | `monitoring.thanos.io/v1alpha1` | | |
| `kind` _string_ | `ThanosStoreList` | | |
| `kind` _string_ | Kind is a string value representing the REST resource this object represents.<br />Servers may infer this from the endpoint the client submits requests to.<br />Cannot be updated.<br />In CamelCase.<br />More info: https://git.k8s.io/community/contributors/devel/sig-architecture/api-conventions.md#types-kinds |  |  |
| `apiVersion` _string_ | APIVersion defines the versioned schema of this representation of an object.<br />Servers should convert recognized schemas to the latest internal value, and<br />may reject unrecognized values.<br />More info: https://git.k8s.io/community/contributors/devel/sig-architecture/api-conventions.md#resources |  |  |
| `metadata` _[ListMeta](https://kubernetes.io/docs/reference/generated/kubernetes-api/v1.31/#listmeta-v1-meta)_ | Refer to Kubernetes API documentation for fields of `metadata`. |  |  |
| `items` _[ThanosStore](#thanosstore) array_ |  |  |  |


#### ThanosStoreSpec



ThanosStoreSpec defines the desired state of ThanosStore



_Appears in:_
- [ThanosStore](#thanosstore)

| Field | Description | Default | Validation |
| --- | --- | --- | --- |
| `version` _string_ | Version of Thanos to be deployed.<br />If not specified, the operator assumes the latest upstream version of<br />Thanos available at the time when the version of the operator was released. |  | Optional: \{\} <br /> |
| `image` _string_ | Container image to use for the Thanos components. |  | Optional: \{\} <br /> |
| `imagePullPolicy` _[PullPolicy](https://kubernetes.io/docs/reference/generated/kubernetes-api/v1.31/#pullpolicy-v1-core)_ | Image pull policy for the Thanos containers.<br />See https://kubernetes.io/docs/concepts/containers/images/#image-pull-policy for more details. | IfNotPresent | Enum: [Always Never IfNotPresent] <br />Optional: \{\} <br /> |
| `imagePullSecrets` _[LocalObjectReference](https://kubernetes.io/docs/reference/generated/kubernetes-api/v1.31/#localobjectreference-v1-core) array_ | An optional list of references to Secrets in the same namespace<br />to use for pulling images from registries.<br />See http://kubernetes.io/docs/user-guide/images#specifying-imagepullsecrets-on-a-pod |  | Optional: \{\} <br /> |
| `resourceRequirements` _[ResourceRequirements](https://kubernetes.io/docs/reference/generated/kubernetes-api/v1.31/#resourcerequirements-v1-core)_ | ResourceRequirements for the Thanos component container. |  | Optional: \{\} <br /> |
| `logLevel` _string_ | Log level for Thanos. |  | Enum: [debug info warn error] <br />Optional: \{\} <br /> |
| `logFormat` _string_ | Log format for Thanos. | logfmt | Enum: [logfmt json] <br />Optional: \{\} <br /> |
| `replicas` _integer_ | Replicas is the number of store or store shard replicas. | 1 | Minimum: 1 <br />Required: \{\} <br /> |
| `labels` _object (keys:string, values:string)_ | Labels are additional labels to add to the Store component. |  | Optional: \{\} <br /> |
| `objectStorageConfig` _[ObjectStorageConfig](#objectstorageconfig)_ | ObjectStorageConfig is the secret that contains the object storage configuration for Store Gateways. |  | Required: \{\} <br /> |
| `storageSize` _[StorageSize](#storagesize)_ | StorageSize is the size of the storage to be used by the Thanos Store StatefulSets. |  | Pattern: `^([+-]?[0-9.]+)([eEinumkKMGTP]*[-+]?[0-9]*)$` <br />Required: \{\} <br /> |
| `ignoreDeletionMarksDelay` _[Duration](#duration)_ | Duration after which the blocks marked for deletion will be filtered out while fetching blocks.<br />The idea of ignore-deletion-marks-delay is to ignore blocks that are marked for deletion with some delay.<br />This ensures store can still serve blocks that are meant to be deleted but do not have a replacement yet.<br />If delete-delay duration is provided to compactor or bucket verify component, it will upload deletion-mark.json<br />file to mark after what duration the block should be deleted rather than deleting the block straight away. | 24h | Pattern: `^-?(0\|(([0-9]+)y)?(([0-9]+)w)?(([0-9]+)d)?(([0-9]+)h)?(([0-9]+)m)?(([0-9]+)s)?(([0-9]+)ms)?)$` <br /> |
| `indexCacheConfig` _[CacheConfig](#cacheconfig)_ | IndexCacheConfig allows configuration of the index cache.<br />See format details: https://thanos.io/tip/components/store.md/#index-cache |  | Optional: \{\} <br /> |
| `cachingBucketConfig` _[CacheConfig](#cacheconfig)_ | CachingBucketConfig allows configuration of the caching bucket.<br />See format details: https://thanos.io/tip/components/store.md/#caching-bucket |  | Optional: \{\} <br /> |
| `shardingStrategy` _[ShardingStrategy](#shardingstrategy)_ | ShardingStrategy defines the sharding strategy for the Store Gateways across object storage blocks. |  | Required: \{\} <br /> |
| `minTime` _[Duration](#duration)_ | Minimum time range to serve. Any data earlier than this lower time range will be ignored.<br />If not set, will be set as zero value, so most recent blocks will be served. |  | Optional: \{\} <br />Pattern: `^-?(0\|(([0-9]+)y)?(([0-9]+)w)?(([0-9]+)d)?(([0-9]+)h)?(([0-9]+)m)?(([0-9]+)s)?(([0-9]+)ms)?)$` <br /> |
| `maxTime` _[Duration](#duration)_ | Maximum time range to serve. Any data after this upper time range will be ignored.<br />If not set, will be set as max value, so all blocks will be served. |  | Optional: \{\} <br />Pattern: `^-?(0\|(([0-9]+)y)?(([0-9]+)w)?(([0-9]+)d)?(([0-9]+)h)?(([0-9]+)m)?(([0-9]+)s)?(([0-9]+)ms)?)$` <br /> |
| `storeLimitsOptions` _[StoreLimitsOptions](#storelimitsoptions)_ | StoreLimitsOptions allows configuration of the store API limits. |  | Optional: \{\} <br /> |
| `indexHeaderConfig` _[IndexHeaderConfig](#indexheaderconfig)_ | IndexHeaderConfig allows configuration of the Store Gateway index header. |  | Optional: \{\} <br /> |
| `blockConfig` _[BlockConfig](#blockconfig)_ | BlockConfig defines settings for block handling. |  | Optional: \{\} <br /> |
| `paused` _boolean_ | When a resource is paused, no actions except for deletion<br />will be performed on the underlying objects. |  | Optional: \{\} <br /> |
| `featureGates` _[FeatureGates](#featuregates)_ | FeatureGates are feature gates for the compact component. | \{ serviceMonitor:map[enable:true] \} | Optional: \{\} <br /> |
| `additionalArgs` _string array_ | Additional arguments to pass to the Thanos components. |  | Optional: \{\} <br /> |
| `additionalContainers` _[Container](https://kubernetes.io/docs/reference/generated/kubernetes-api/v1.31/#container-v1-core) array_ | Additional containers to add to the Thanos components. |  | Optional: \{\} <br /> |
| `additionalVolumes` _[Volume](https://kubernetes.io/docs/reference/generated/kubernetes-api/v1.31/#volume-v1-core) array_ | Additional volumes to add to the Thanos components. |  | Optional: \{\} <br /> |
| `additionalVolumeMounts` _[VolumeMount](https://kubernetes.io/docs/reference/generated/kubernetes-api/v1.31/#volumemount-v1-core) array_ | Additional volume mounts to add to the Thanos component container in a Deployment or StatefulSet<br />controlled by the operator. |  | Optional: \{\} <br /> |
| `additionalPorts` _[ContainerPort](https://kubernetes.io/docs/reference/generated/kubernetes-api/v1.31/#containerport-v1-core) array_ | Additional ports to expose on the Thanos component container in a Deployment or StatefulSet<br />controlled by the operator. |  | Optional: \{\} <br /> |
| `additionalEnv` _[EnvVar](https://kubernetes.io/docs/reference/generated/kubernetes-api/v1.31/#envvar-v1-core) array_ | Additional environment variables to add to the Thanos component container in a Deployment or StatefulSet<br />controlled by the operator. |  | Optional: \{\} <br /> |
| `additionalServicePorts` _[ServicePort](https://kubernetes.io/docs/reference/generated/kubernetes-api/v1.31/#serviceport-v1-core) array_ | AdditionalServicePorts are additional ports to expose on the Service for the Thanos component. |  | Optional: \{\} <br /> |


#### ThanosStoreStatus



ThanosStoreStatus defines the observed state of ThanosStore



_Appears in:_
- [ThanosStore](#thanosstore)

| Field | Description | Default | Validation |
| --- | --- | --- | --- |
| `conditions` _[Condition](https://kubernetes.io/docs/reference/generated/kubernetes-api/v1.31/#condition-v1-meta) array_ | Conditions represent the latest available observations of the state of the Querier. |  |  |


#### WebConfig



WebConfig is the configuration for the Query UI and API web options.



_Appears in:_
- [ThanosQuerySpec](#thanosqueryspec)

| Field | Description | Default | Validation |
| --- | --- | --- | --- |
| `routePrefix` _string_ | RoutePrefix is the prefix for API and UI endpoints.<br />This allows thanos UI to be served on a sub-path.<br />Defaults to the value of --web.external-prefix.<br />This option is analogous to --web.route-prefix of Prometheus. |  | Optional: \{\} <br /> |
| `externalPrefix` _string_ | ExternalPrefix is the static prefix for all HTML links and redirect URLs in the UI query web interface.<br />Actual endpoints are still served on / or the web.route-prefix.<br />This allows thanos UI to be served behind a reverse proxy that strips a URL sub-path. |  | Optional: \{\} <br /> |
| `prefixHeader` _string_ | PrefixHeader is the name of HTTP request header used for dynamic prefixing of UI links and redirects.<br />This option is ignored if web.external-prefix argument is set.<br />Security risk: enable this option only if a reverse proxy in front of thanos is resetting the header.<br />This allows thanos UI to be served on a sub-path. |  | Optional: \{\} <br /> |
| `disableCORS` _boolean_ | DisableCORS is the flag to disable CORS headers to be set by Thanos.<br />By default Thanos sets CORS headers to be allowed by all. | false |  |


