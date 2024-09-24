package controller

import (
	"fmt"
	"github.com/thanos-community/thanos-operator/api/v1alpha1"
	"github.com/thanos-community/thanos-operator/internal/pkg/manifests"
	manifestquery "github.com/thanos-community/thanos-operator/internal/pkg/manifests/query"
	manifestqueryfrontend "github.com/thanos-community/thanos-operator/internal/pkg/manifests/queryfrontend"
	manifestruler "github.com/thanos-community/thanos-operator/internal/pkg/manifests/ruler"
	manifestsstore "github.com/thanos-community/thanos-operator/internal/pkg/manifests/store"
	"k8s.io/apimachinery/pkg/api/resource"
)

func queryAlphaV1ToOptions(in v1alpha1.ThanosQuery) manifestquery.Options {
	labels := manifests.MergeLabels(in.GetLabels(), in.Spec.Labels)
	opts := commonToOpts(queryV1AlphaV1NameFromSpec(in), in.GetNamespace(), in.Spec.Replicas, labels, in.Spec.CommonThanosFields, in.Spec.Additional)
	return manifestquery.Options{
		Options:       opts,
		ReplicaLabels: in.Spec.QuerierReplicaLabels,
		Timeout:       "15m",
		LookbackDelta: "5m",
		MaxConcurrent: 20,
	}
}

func queryV1AlphaV1NameFromSpec(in v1alpha1.ThanosQuery) string {
	return fmt.Sprintf("thanos-query-%s", in.GetName())
}

// queryAlphaV1ToQueryFrontEndOptions transforms a v1alpha1.ThanosQuery to a build Options
func queryAlphaV1ToQueryFrontEndOptions(in v1alpha1.ThanosQuery) manifestqueryfrontend.Options {
	labels := manifests.MergeLabels(in.GetLabels(), in.Spec.Labels)
	name := queryFrontendV1AlphaV1NameFromSpec(in)
	frontend := in.Spec.QueryFrontend
	opts := commonToOpts(name, in.GetNamespace(), frontend.Replicas, labels, frontend.CommonThanosFields, frontend.Additional)

	return manifestqueryfrontend.Options{
		Options:                opts,
		QueryService:           name,
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

func queryFrontendV1AlphaV1NameFromSpec(in v1alpha1.ThanosQuery) string {
	return fmt.Sprintf("thanos-query-frontend-%s", in.GetName())
}

func rulerAlphaV1ToOptions(in v1alpha1.ThanosRuler) manifestruler.Options {
	labels := manifests.MergeLabels(in.GetLabels(), nil)
	opts := commonToOpts(rulerV1AlphaV1NameFromSpec(in), in.GetNamespace(), in.Spec.Replicas, labels, in.Spec.CommonThanosFields, in.Spec.Additional)
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

func rulerV1AlphaV1NameFromSpec(in v1alpha1.ThanosRuler) string {
	return fmt.Sprintf("thanos-ruler-%s", in.GetName())
}

func storeAlphaV1ToOptions(in v1alpha1.ThanosStore) manifestsstore.Options {
	labels := manifests.MergeLabels(in.GetLabels(), in.Spec.Labels)
	opts := commonToOpts(storeV1AlphaV1NameFromSpec(in), in.GetNamespace(), in.Spec.ShardingStrategy.ShardReplicas, labels, in.Spec.CommonThanosFields, in.Spec.Additional)
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

func storeV1AlphaV1NameFromSpec(in v1alpha1.ThanosStore) string {
	return fmt.Sprintf("thanos-store-%s", in.GetName())
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
