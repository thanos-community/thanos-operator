package controllers_metrics

import (
	"github.com/prometheus/client_golang/prometheus"
	"github.com/prometheus/client_golang/prometheus/promauto"
)

type BaseMetrics struct {
	ReconciliationsTotal       *prometheus.CounterVec
	ReconciliationsFailedTotal *prometheus.CounterVec
	ClientErrorsTotal          *prometheus.CounterVec
}

func NewBaseMetrics(reg prometheus.Registerer) *BaseMetrics {
	return &BaseMetrics{
		ReconciliationsTotal: promauto.With(reg).NewCounterVec(prometheus.CounterOpts{
			Name: "thanos_operator_reconciliations_total",
			Help: "Total number of reconciliations for Thanos resources",
		}, []string{"component"}),
		ReconciliationsFailedTotal: promauto.With(reg).NewCounterVec(prometheus.CounterOpts{
			Name: "thanos_operator_reconciliations_failed_total",
			Help: "Total number of failed reconciliations for Thanos resources",
		}, []string{"component"}),
		ClientErrorsTotal: promauto.With(reg).NewCounterVec(prometheus.CounterOpts{
			Name: "thanos_operator_client_errors_total",
			Help: "Total number of errors encountered during kube client calls of Thanos resources",
		}, []string{"component"}),
	}
}
