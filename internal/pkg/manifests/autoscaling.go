package manifests

import (
	autoscalingv2 "k8s.io/api/autoscaling/v2"
	corev1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
)

// HorizontalPodAutoscalerOptions defines options for creating a HorizontalPodAutoscaler.
type HorizontalPodAutoscalerOptions struct {
	Name                                string
	Namespace                           string
	Labels                              map[string]string
	Annotations                         map[string]string
	TargetRef                           autoscalingv2.CrossVersionObjectReference
	MinReplicas                         *int32
	MaxReplicas                         int32
	TargetCPUUtilizationPercentage      *int32
	TargetMemoryUtilizationPercentage   *int32
	ScaleDownStabilizationWindowSeconds *int32
	ScaleUpStabilizationWindowSeconds   *int32
	ScaleUpPods                         *int32
	ScaleDownPods                       *int32
}

// BuildHorizontalPodAutoscaler creates a HorizontalPodAutoscaler resource.
func BuildHorizontalPodAutoscaler(opts HorizontalPodAutoscalerOptions) *autoscalingv2.HorizontalPodAutoscaler {
	hpa := &autoscalingv2.HorizontalPodAutoscaler{
		TypeMeta: metav1.TypeMeta{
			Kind:       "HorizontalPodAutoscaler",
			APIVersion: autoscalingv2.SchemeGroupVersion.String(),
		},
		ObjectMeta: metav1.ObjectMeta{
			Name:        opts.Name,
			Namespace:   opts.Namespace,
			Labels:      opts.Labels,
			Annotations: opts.Annotations,
		},
		Spec: autoscalingv2.HorizontalPodAutoscalerSpec{
			ScaleTargetRef: opts.TargetRef,
			MinReplicas:    opts.MinReplicas,
			MaxReplicas:    opts.MaxReplicas,
		},
	}

	var metrics []autoscalingv2.MetricSpec
	if opts.TargetCPUUtilizationPercentage != nil {
		metrics = append(metrics, autoscalingv2.MetricSpec{
			Type: autoscalingv2.ResourceMetricSourceType,
			Resource: &autoscalingv2.ResourceMetricSource{
				Name: corev1.ResourceCPU,
				Target: autoscalingv2.MetricTarget{
					Type:               autoscalingv2.UtilizationMetricType,
					AverageUtilization: opts.TargetCPUUtilizationPercentage,
				},
			},
		})
	}

	if opts.TargetMemoryUtilizationPercentage != nil {
		metrics = append(metrics, autoscalingv2.MetricSpec{
			Type: autoscalingv2.ResourceMetricSourceType,
			Resource: &autoscalingv2.ResourceMetricSource{
				Name: corev1.ResourceMemory,
				Target: autoscalingv2.MetricTarget{
					Type:               autoscalingv2.UtilizationMetricType,
					AverageUtilization: opts.TargetMemoryUtilizationPercentage,
				},
			},
		})
	}

	if len(metrics) > 0 {
		hpa.Spec.Metrics = metrics
	}

	// Add/remove 2 pods every 15 seconds by default.
	// For topology-aware setups, users should set scaleUpPods/scaleDownPods to match zone count.
	hpa.Spec.Behavior = &autoscalingv2.HorizontalPodAutoscalerBehavior{}

	scaleDownPods := int32(2)
	if opts.ScaleDownPods != nil {
		scaleDownPods = *opts.ScaleDownPods
	}
	scaleDownStabilization := int32(300) // Default 5 minutes
	if opts.ScaleDownStabilizationWindowSeconds != nil {
		scaleDownStabilization = *opts.ScaleDownStabilizationWindowSeconds
	}
	hpa.Spec.Behavior.ScaleDown = &autoscalingv2.HPAScalingRules{
		StabilizationWindowSeconds: &scaleDownStabilization,
		Policies: []autoscalingv2.HPAScalingPolicy{
			{
				Type:          autoscalingv2.PodsScalingPolicy,
				Value:         scaleDownPods,
				PeriodSeconds: 15,
			},
		},
	}

	scaleUpPods := int32(2)
	if opts.ScaleUpPods != nil {
		scaleUpPods = *opts.ScaleUpPods
	}
	// By default, scale up immediately.
	scaleUpStabilization := int32(0)
	if opts.ScaleUpStabilizationWindowSeconds != nil {
		scaleUpStabilization = *opts.ScaleUpStabilizationWindowSeconds
	}
	hpa.Spec.Behavior.ScaleUp = &autoscalingv2.HPAScalingRules{
		StabilizationWindowSeconds: &scaleUpStabilization,
		Policies: []autoscalingv2.HPAScalingPolicy{
			{
				Type:          autoscalingv2.PodsScalingPolicy,
				Value:         scaleUpPods,
				PeriodSeconds: 15,
			},
		},
	}

	return hpa
}
