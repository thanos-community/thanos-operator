package manifests

import (
	monitoringv1 "github.com/prometheus-operator/prometheus-operator/pkg/apis/monitoring/v1"
	"github.com/thanos-community/thanos-operator/api/v1alpha1"

	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/utils/ptr"
)

type ServiceMonitorConfig struct {
	Enabled   bool
	Namespace string
	Interval  *Duration
	Labels    map[string]string
}

type ServiceMonitorOptions struct {
	// Port is the name of the port on the target service to scrape.
	// Defaults to "http" if not specified.
	Port *string
	// Interval at which metrics should be scraped.
	// Defaults to 30s if not specified.
	Interval *Duration
	// Path is the path on the target service to scrape for metrics.
	// Defaults to "/metrics" if not specified.
	Path *string
}

func BuildServiceMonitor(name, namespace string, objectMetaLabels, selectorLabels map[string]string, opts ServiceMonitorOptions) *monitoringv1.ServiceMonitor {
	opts = opts.applyDefaults()

	return &monitoringv1.ServiceMonitor{
		TypeMeta: metav1.TypeMeta{
			Kind:       "ServiceMonitor",
			APIVersion: monitoringv1.SchemeGroupVersion.String(),
		},
		ObjectMeta: metav1.ObjectMeta{
			Name:      name,
			Namespace: namespace,
			Labels:    objectMetaLabels,
		},
		Spec: monitoringv1.ServiceMonitorSpec{
			Selector: metav1.LabelSelector{
				MatchLabels: selectorLabels,
			},
			NamespaceSelector: monitoringv1.NamespaceSelector{
				MatchNames: []string{namespace},
			},
			Endpoints: []monitoringv1.Endpoint{
				{
					Interval: monitoringv1.Duration(*opts.Interval),
					Port:     *opts.Port,
					Path:     *opts.Path,
				},
			},
		},
	}
}

func (opts ServiceMonitorOptions) applyDefaults() ServiceMonitorOptions {
	if opts.Port == nil {
		opts.Port = ptr.To("http")
	}
	if opts.Interval == nil {
		opts.Interval = ptr.To(Duration("30s"))
	}
	if opts.Path == nil {
		opts.Path = ptr.To("/metrics")
	}
	return opts
}

func HasServiceMonitorEnabled(in *v1alpha1.FeatureGates) bool {
	return in != nil && in.ServiceMonitorConfig != nil && *in.ServiceMonitorConfig.Enable
}

func HasPrometheusRuleEnabled(in *v1alpha1.FeatureGates) bool {
	return in != nil && in.PrometheusRuleEnabled != nil && *in.PrometheusRuleEnabled
}
