package controller

import (
	"testing"

	"github.com/stretchr/testify/assert"

	"github.com/thanos-community/thanos-operator/api/v1alpha1"
	"github.com/thanos-community/thanos-operator/internal/pkg/featuregate"

	corev1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
)

func TestCompactV1Alpha1ToOptions_Labels(t *testing.T) {
	compact := v1alpha1.ThanosCompact{
		ObjectMeta: metav1.ObjectMeta{
			Name:      "test-compact",
			Namespace: "test-ns",
			Labels: map[string]string{
				"compact-parent-label": "compact-parent-value",
				"compact-conflict-key": "parent-not-selected",
			},
		},
		Spec: v1alpha1.ThanosCompactSpec{
			CommonFields: v1alpha1.CommonFields{
				Labels: map[string]string{
					"compact-spec-label":   "compact-spec-value",
					"compact-conflict-key": "spec-selected",
				},
			},
			ObjectStorageConfig: v1alpha1.ObjectStorageConfig{
				LocalObjectReference: corev1.LocalObjectReference{Name: "test-secret"},
				Key:                  "test-key",
			},
			StorageConfiguration: v1alpha1.StorageConfiguration{
				Size: "1Gi",
			},
		},
	}

	opts := compactV1Alpha1ToOptions(compact, featuregate.Config{
		EnableServiceMonitor:          false,
		EnablePrometheusRuleDiscovery: false,
	})

	// Should have all three labels
	assert.Equal(t, "compact-parent-value", opts.Labels["compact-parent-label"])
	assert.Equal(t, "compact-spec-value", opts.Labels["compact-spec-label"])
	// Spec should win conflicts
	assert.Equal(t, "spec-selected", opts.Labels["compact-conflict-key"])
}

func TestQueryV1Alpha1ToOptions_Labels(t *testing.T) {
	query := v1alpha1.ThanosQuery{
		ObjectMeta: metav1.ObjectMeta{
			Name:      "test-query",
			Namespace: "test-ns",
			Labels: map[string]string{
				"query-parent-label": "query-parent-value",
				"query-conflict-key": "parent-not-selected",
			},
		},
		Spec: v1alpha1.ThanosQuerySpec{
			CommonFields: v1alpha1.CommonFields{
				Labels: map[string]string{
					"query-spec-label":   "query-spec-value",
					"query-conflict-key": "spec-selected",
				},
			},
		},
	}

	opts := queryV1Alpha1ToOptions(query, featuregate.Config{
		EnableServiceMonitor:          false,
		EnablePrometheusRuleDiscovery: false,
	})

	assert.Equal(t, "query-parent-value", opts.Labels["query-parent-label"])
	assert.Equal(t, "query-spec-value", opts.Labels["query-spec-label"])
	assert.Equal(t, "spec-selected", opts.Labels["query-conflict-key"])
}

func TestRulerV1Alpha1ToOptions_Labels(t *testing.T) {
	ruler := v1alpha1.ThanosRuler{
		ObjectMeta: metav1.ObjectMeta{
			Name:      "test-ruler",
			Namespace: "test-ns",
			Labels: map[string]string{
				"ruler-parent-label": "ruler-parent-value",
				"ruler-conflict-key": "parent-not-selected",
			},
		},
		Spec: v1alpha1.ThanosRulerSpec{
			CommonFields: v1alpha1.CommonFields{
				Labels: map[string]string{
					"ruler-spec-label":   "ruler-spec-value",
					"ruler-conflict-key": "spec-selected",
				},
			},
			ObjectStorageConfig: v1alpha1.ObjectStorageConfig{
				LocalObjectReference: corev1.LocalObjectReference{Name: "test-secret"},
				Key:                  "test-key",
			},
			StorageConfiguration: v1alpha1.StorageConfiguration{
				Size: "1Gi",
			},
		},
	}

	opts := rulerV1Alpha1ToOptions(ruler, featuregate.Config{
		EnableServiceMonitor:          false,
		EnablePrometheusRuleDiscovery: false,
	})

	assert.Equal(t, "ruler-parent-value", opts.Labels["ruler-parent-label"])
	assert.Equal(t, "ruler-spec-value", opts.Labels["ruler-spec-label"])
	assert.Equal(t, "spec-selected", opts.Labels["ruler-conflict-key"])
}

func TestStoreV1Alpha1ToOptions_Labels(t *testing.T) {
	store := v1alpha1.ThanosStore{
		ObjectMeta: metav1.ObjectMeta{
			Name:      "test-store",
			Namespace: "test-ns",
			Labels: map[string]string{
				"store-parent-label": "store-parent-value",
				"store-conflict-key": "parent-not-selected",
			},
		},
		Spec: v1alpha1.ThanosStoreSpec{
			CommonFields: v1alpha1.CommonFields{
				Labels: map[string]string{
					"store-spec-label":   "store-spec-value",
					"store-conflict-key": "spec-selected",
				},
			},
			ObjectStorageConfig: v1alpha1.ObjectStorageConfig{
				LocalObjectReference: corev1.LocalObjectReference{Name: "test-secret"},
				Key:                  "test-key",
			},
			StorageConfiguration: v1alpha1.StorageConfiguration{
				Size: "1Gi",
			},
		},
	}

	opts := storeV1Alpha1ToOptions(store, featuregate.Config{
		EnableServiceMonitor:          false,
		EnablePrometheusRuleDiscovery: false,
	})

	assert.Equal(t, "store-parent-value", opts.Labels["store-parent-label"])
	assert.Equal(t, "store-spec-value", opts.Labels["store-spec-label"])
	assert.Equal(t, "spec-selected", opts.Labels["store-conflict-key"])
}

func TestReceiveV1Alpha1ToRouterOptions_Labels(t *testing.T) {
	receive := v1alpha1.ThanosReceive{
		ObjectMeta: metav1.ObjectMeta{
			Name:      "test-receive",
			Namespace: "test-ns",
			Labels: map[string]string{
				"receive-label": "receive-value",
				"conflict-key":  "receive-not-selected",
			},
		},
		Spec: v1alpha1.ThanosReceiveSpec{
			Router: v1alpha1.RouterSpec{
				CommonFields: v1alpha1.CommonFields{
					Labels: map[string]string{
						"router-label": "router-value",
						"conflict-key": "router-selected",
					},
				},
			},
		},
	}

	opts := receiverV1Alpha1ToRouterOptions(receive, featuregate.Config{
		EnableServiceMonitor:          false,
		EnablePrometheusRuleDiscovery: false,
	})

	assert.Equal(t, "receive-value", opts.Labels["receive-label"])
	assert.Equal(t, "router-value", opts.Labels["router-label"])
	assert.Equal(t, "router-selected", opts.Labels["conflict-key"])
}

func TestReceiveV1AlphaToIngestOptions_Labels(t *testing.T) {
	receive := v1alpha1.ThanosReceive{
		ObjectMeta: metav1.ObjectMeta{
			Name:      "test-receive",
			Namespace: "test-ns",
			Labels: map[string]string{
				"receive-label": "receive-value",
				"conflict-key":  "receive-not-selected",
			},
		},
	}

	spec := v1alpha1.IngesterHashringSpec{
		CommonFields: v1alpha1.CommonFields{
			Labels: map[string]string{
				"ingester-label": "ingester-value",
				"conflict-key":   "ingester-selected",
			},
		},
		StorageConfiguration: v1alpha1.StorageConfiguration{
			Size: "100Mi",
		},
	}

	opts := receiverV1Alpha1ToIngesterOptions(receive, spec, featuregate.Config{
		EnableServiceMonitor:          false,
		EnablePrometheusRuleDiscovery: false,
	})

	assert.Equal(t, "receive-value", opts.Labels["receive-label"])
	assert.Equal(t, "ingester-value", opts.Labels["ingester-label"])
	assert.Equal(t, "ingester-selected", opts.Labels["conflict-key"])
}
