package controllers_metrics

import (
	"fmt"

	"github.com/prometheus/client_golang/prometheus"
	"github.com/prometheus/client_golang/prometheus/promauto"
)

type Metrics interface {
	IncReconciliationsTotal()
	IncReconciliationsFailedTotal()
	IncClientErrorsTotal()
}

type BaseMetrics struct {
	ReconciliationsTotal       prometheus.Counter
	ReconciliationsFailedTotal prometheus.Counter
	ClientErrorsTotal          prometheus.Counter
}

func NewBaseMetrics(reg prometheus.Registerer, component string) *BaseMetrics {
	return &BaseMetrics{
		ReconciliationsTotal: promauto.With(reg).NewCounter(prometheus.CounterOpts{
			Name: fmt.Sprintf("thanos_operator_%s_reconciliations_total", component),
			Help: fmt.Sprintf("Total number of reconciliations for %s resources", component),
		}),
		ReconciliationsFailedTotal: promauto.With(reg).NewCounter(prometheus.CounterOpts{
			Name: fmt.Sprintf("thanos_operator_%s_reconciliations_failed_total", component),
			Help: fmt.Sprintf("Total number of failed reconciliations for %s resources", component),
		}),
		ClientErrorsTotal: promauto.With(reg).NewCounter(prometheus.CounterOpts{
			Name: fmt.Sprintf("thanos_operator_%s_client_errors_total", component),
			Help: fmt.Sprintf("Total number of errors encountered during kube client calls of %s resources", component),
		}),
	}
}
