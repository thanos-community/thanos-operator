package controllers_metrics

import (
	"github.com/prometheus/client_golang/prometheus"
)

type ThanosStoreMetrics struct {
	*BaseMetrics
}

func NewThanosStoreMetrics(reg prometheus.Registerer) ThanosStoreMetrics {
	return ThanosStoreMetrics{
		BaseMetrics: NewBaseMetrics(reg, "store"),
	}
}
