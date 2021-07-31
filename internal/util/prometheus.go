package util

import (
	"github.com/prometheus/client_golang/prometheus"
	"github.com/prometheus/client_golang/prometheus/promauto"
)

type ControllerFunctionalPrometheusMetrics struct {
	// ConfigCounter number of post /config to proxy successfully
	ConfigCounter *prometheus.CounterVec

	// ParseCounter number of ingress analysis failure
	ParseCounter *prometheus.CounterVec

	// ConfigureDurationHistogram duration of last successful confiuration sync
	ConfigureDurationHistogram prometheus.Histogram
}

type Success string

const (
	// ConfigSuccessTrue post-config to proxy successfully
	ConfigSuccessTrue Success = "true"
	// ConfigSuccessFalse post-config to proxy failed
	ConfigSuccessFalse Success = "false"
	// IngressParseTrue says that ingress parsed successful
	IngressParseTrue Success = "true"
	// IngressParseFalse ingress parsed failed
	IngressParseFalse Success = "false"
)

type ConfigType string

const (
	// ConfigProxy says post config to proxy
	ConfigProxy ConfigType = "post-config"
	// ConfigDeck says generate deck
	ConfigDeck ConfigType = "deck"
)

func ControllerMetricsInit() *ControllerFunctionalPrometheusMetrics {
	controllerMetrics := &ControllerFunctionalPrometheusMetrics{}

	reg := prometheus.NewRegistry()

	controllerMetrics.ConfigCounter =
		promauto.With(reg).NewCounterVec(
			prometheus.CounterOpts{
				Name: "send_configuration_count",
				Help: "number of post config proxy processed successfully.",
			},
			[]string{"success", "type"},
		)

	controllerMetrics.ParseCounter =
		promauto.With(reg).NewCounterVec(
			prometheus.CounterOpts{
				Name: "ingress_parse_count",
				Help: "number of ingress parse.",
			},
			[]string{"success"},
		)

	controllerMetrics.ConfigureDurationHistogram =
		promauto.With(reg).NewHistogram(
			prometheus.HistogramOpts{
				Name:    "proxy_configuration_duration_milliseconds",
				Help:    "duration of last successful configuration.",
				Buckets: prometheus.ExponentialBuckets(1, 10, 4),
			},
		)

	return controllerMetrics
}
