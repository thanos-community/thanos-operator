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

func queryV1Alpha1ToOptions(in v1alpha1.ThanosQuery) manifestquery.Options {
	labels := manifests.MergeLabels(in.GetLabels(), in.Spec.Labels)
	name := queryV1Alpha1NameFromParent(in.GetName())
	opts := commonToOpts(name, in.GetNamespace(), in.Spec.Replicas, labels, in.Spec.CommonThanosFields, in.Spec.Additional)
	return manifestquery.Options{
		Options:       opts,
		ReplicaLabels: in.Spec.QuerierReplicaLabels,
		Timeout:       "15m",
		LookbackDelta: "5m",
		MaxConcurrent: 20,
	}
}

func queryV1Alpha1NameFromParent(resourceName string) string {
	return fmt.Sprintf("thanos-query-%s", resourceName)
}

// queryV1Alpha1ToQueryFrontEndOptions transforms a v1alpha1.ThanosQuery to a build Options
func queryV1Alpha1ToQueryFrontEndOptions(in v1alpha1.ThanosQuery) manifestqueryfrontend.Options {
	labels := manifests.MergeLabels(in.GetLabels(), in.Spec.Labels)
	name := queryFrontendV1Alpha1NameFromParent(in.GetName())
	frontend := in.Spec.QueryFrontend
	opts := commonToOpts(name, in.GetNamespace(), frontend.Replicas, labels, frontend.CommonThanosFields, frontend.Additional)

	return manifestqueryfrontend.Options{
		Options:                opts,
		QueryService:           queryV1Alpha1NameFromParent(in.GetName()),
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

func queryFrontendV1Alpha1NameFromParent(resourceName string) string {
	return fmt.Sprintf("thanos-query-frontend-%s", resourceName)
}

func rulerV1Alpha1ToOptions(in v1alpha1.ThanosRuler) manifestruler.Options {
	labels := manifests.MergeLabels(in.GetLabels(), nil)
	name := rulerV1Alpha1NameFromParent(in.GetName())
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

func rulerV1Alpha1NameFromParent(resourceName string) string {
	return fmt.Sprintf("thanos-ruler-%s", resourceName)
}

func storeV1Alpha1ToOptions(in v1alpha1.ThanosStore) manifestsstore.Options {
	labels := manifests.MergeLabels(in.GetLabels(), in.Spec.Labels)
	name := storeV1Alpha1NameFromParent(in.GetName())
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

func storeV1Alpha1NameFromParent(resourceName string) string {
	return fmt.Sprintf("thanos-store-%s", resourceName)
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
