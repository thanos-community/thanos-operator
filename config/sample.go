package config

import (
	corev1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/utils/ptr"

	thanosv1alpha1 "github.com/thanos-community/thanos-operator/api/v1alpha1"
)

// SampleCR returns a sample CR for the given CRD.
func SampleCR(crd CRD) any {
	switch crd.Kind {
	case "ThanosQuery":
		return &thanosv1alpha1.ThanosQuery{
			TypeMeta: metav1.TypeMeta{
				APIVersion: thanosv1alpha1.GroupVersion.String(),
				Kind:       "ThanosQuery",
			},
			ObjectMeta: metav1.ObjectMeta{
				Name: "example-query",
			},
			Spec: thanosv1alpha1.ThanosQuerySpec{
				CommonFields: thanosv1alpha1.CommonFields{
					ImagePullPolicy: ptr.To(corev1.PullIfNotPresent),
					LogFormat:       ptr.To("logfmt"),
					Labels: map[string]string{
						"some-label": "xyz",
					},
				},
				Replicas: 1,
				ReplicaLabels: []string{
					"prometheus_replica",
					"replica",
					"rule_replica",
				},
				StoreLabelSelector: &metav1.LabelSelector{
					MatchLabels: map[string]string{
						"operator.thanos.io/store-api": "true",
					},
				},
				QueryFrontend: &thanosv1alpha1.QueryFrontendSpec{
					CommonFields: thanosv1alpha1.CommonFields{
						ImagePullPolicy: ptr.To(corev1.PullIfNotPresent),
						LogFormat:       ptr.To("logfmt"),
					},
					Replicas:             2,
					CompressResponses:    true,
					LogQueriesLongerThan: ptr.To(thanosv1alpha1.Duration("10s")),
					LabelsMaxRetries:     3,
					QueryRangeMaxRetries: 3,
					QueryLabelSelector: &metav1.LabelSelector{
						MatchLabels: map[string]string{
							"operator.thanos.io/query-api": "true",
						},
					},
				},
			},
		}

	case "ThanosStore":
		return &thanosv1alpha1.ThanosStore{
			TypeMeta: metav1.TypeMeta{
				APIVersion: thanosv1alpha1.GroupVersion.String(),
				Kind:       "ThanosStore",
			},
			ObjectMeta: metav1.ObjectMeta{
				Name: "example-store",
			},
			Spec: thanosv1alpha1.ThanosStoreSpec{
				CommonFields: thanosv1alpha1.CommonFields{
					ImagePullPolicy: ptr.To(corev1.PullIfNotPresent),
					LogFormat:       ptr.To("logfmt"),
					Labels:          map[string]string{"some-label": "xyz"},
				},
				ObjectStorageConfig: thanosv1alpha1.ObjectStorageConfig{
					LocalObjectReference: corev1.LocalObjectReference{
						Name: "thanos-object-storage",
					},
					Key: "thanos.yaml",
				},
				ShardingStrategy: thanosv1alpha1.ShardingStrategy{
					Type:   thanosv1alpha1.Block,
					Shards: 2,
				},
				StorageConfiguration: thanosv1alpha1.StorageConfiguration{
					Size: "1Gi",
				},
				IgnoreDeletionMarksDelay: "24h",
			},
		}

	case "ThanosReceive":
		return &thanosv1alpha1.ThanosReceive{
			TypeMeta: metav1.TypeMeta{
				APIVersion: thanosv1alpha1.GroupVersion.String(),
				Kind:       "ThanosReceive",
			},
			ObjectMeta: metav1.ObjectMeta{
				Name: "example-receive",
			},
			Spec: thanosv1alpha1.ThanosReceiveSpec{
				Ingester: thanosv1alpha1.IngesterSpec{
					DefaultObjectStorageConfig: thanosv1alpha1.ObjectStorageConfig{
						LocalObjectReference: corev1.LocalObjectReference{
							Name: "thanos-object-storage",
						},
						Key: "thanos.yaml",
					},
					Hashrings: []thanosv1alpha1.IngesterHashringSpec{
						{
							Name: "blue",
							StorageConfiguration: thanosv1alpha1.StorageConfiguration{
								Size: "100Mi",
							},
							TSDBConfig: thanosv1alpha1.TSDBConfig{
								Retention: "2h",
							},
							TenancyConfig: &thanosv1alpha1.TenancyConfig{
								TenantMatcherType: "exact",
							},
							Replicas: 1,
							ExternalLabels: map[string]string{
								"replica": "$(POD_NAME)",
							},
						},
						{
							Name: "green",
							StorageConfiguration: thanosv1alpha1.StorageConfiguration{
								Size: "100Mi",
							},
							TSDBConfig: thanosv1alpha1.TSDBConfig{
								Retention: "2h",
							},
							TenancyConfig: &thanosv1alpha1.TenancyConfig{
								TenantMatcherType: "exact",
							},
							Replicas: 3,
							ExternalLabels: map[string]string{
								"replica": "$(POD_NAME)",
							},
						},
					},
				},
				Router: thanosv1alpha1.RouterSpec{
					CommonFields: thanosv1alpha1.CommonFields{
						LogFormat:       ptr.To("logfmt"),
						ImagePullPolicy: ptr.To(corev1.PullIfNotPresent),
					},
					ExternalLabels: map[string]string{
						"receive": "true",
					},
					Replicas:          3,
					ReplicationFactor: 1,
				},
			},
		}

	case "ThanosCompact":
		return &thanosv1alpha1.ThanosCompact{
			TypeMeta: metav1.TypeMeta{
				APIVersion: thanosv1alpha1.GroupVersion.String(),
				Kind:       "ThanosCompact",
			},
			ObjectMeta: metav1.ObjectMeta{
				Name: "example-compact",
			},
			Spec: thanosv1alpha1.ThanosCompactSpec{
				StorageConfiguration: thanosv1alpha1.StorageConfiguration{
					Size: "100Mi",
				},
				ShardingConfig: []thanosv1alpha1.ShardingConfig{
					{
						ShardName: "example",
						ExternalLabelSharding: []thanosv1alpha1.ExternalLabelShardingConfig{
							{
								Label: "tenant_id",
								Value: "a",
							},
						},
					},
				},
				ObjectStorageConfig: thanosv1alpha1.ObjectStorageConfig{
					LocalObjectReference: corev1.LocalObjectReference{
						Name: "thanos-object-storage",
					},
					Key: "thanos.yaml",
				},
				RetentionConfig: thanosv1alpha1.RetentionResolutionConfig{
					Raw:         "30d",
					FiveMinutes: "30d",
					OneHour:     "30d",
				},
			},
		}

	case "ThanosRuler":
		return &thanosv1alpha1.ThanosRuler{
			TypeMeta: metav1.TypeMeta{
				APIVersion: thanosv1alpha1.GroupVersion.String(),
				Kind:       "ThanosRuler",
			},
			ObjectMeta: metav1.ObjectMeta{
				Name: "example-ruler",
			},
			Spec: thanosv1alpha1.ThanosRulerSpec{
				CommonFields: thanosv1alpha1.CommonFields{
					LogFormat:       ptr.To("logfmt"),
					ImagePullPolicy: ptr.To(corev1.PullIfNotPresent),
				},
				Replicas: 1,
				RuleConfigSelector: metav1.LabelSelector{
					MatchLabels: map[string]string{
						"operator.thanos.io/prometheus-rule": "true",
					},
				},
				QueryLabelSelector: &metav1.LabelSelector{
					MatchLabels: map[string]string{
						"operator.thanos.io/query-api": "true",
						"app.kubernetes.io/part-of":    "thanos",
					},
				},
				ObjectStorageConfig: thanosv1alpha1.ObjectStorageConfig{
					LocalObjectReference: corev1.LocalObjectReference{
						Name: "thanos-object-storage",
					},
					Key: "thanos.yaml",
				},
				AlertmanagerURL: "http://alertmanager.example.com:9093",
				ExternalLabels: map[string]string{
					"rule_replica": "$(NAME)",
				},
				EvaluationInterval: "1m",
				Retention:          "2h",
				StorageConfiguration: thanosv1alpha1.StorageConfiguration{
					Size: "1Gi",
				},
			},
		}

	default:
		return nil
	}
}
