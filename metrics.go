package gbox

import (
	"github.com/jensneuse/graphql-go-tools/pkg/graphql"
	"github.com/prometheus/client_golang/prometheus"
	"github.com/prometheus/client_golang/prometheus/promauto"
	"sync"
	"time"
)

var metrics *Metrics = new(Metrics)

func init() {
	metrics.once.Do(func() {
		const ns, sub = "caddy", "http_gbox"
		operationLabels := []string{"operation_type", "operation_name"}
		metrics.operationInFlight = promauto.NewGaugeVec(prometheus.GaugeOpts{
			Namespace: ns,
			Subsystem: sub,
			Name:      "operations_in_flight",
			Help:      "Number of graphql operations currently handled by this server.",
		}, operationLabels)

		metrics.operationCount = promauto.NewCounterVec(prometheus.CounterOpts{
			Namespace: ns,
			Subsystem: sub,
			Name:      "operation_total",
			Help:      "Counter of graphql operations served.",
		}, operationLabels)

		metrics.operationDuration = promauto.NewHistogramVec(prometheus.HistogramOpts{
			Namespace: ns,
			Subsystem: sub,
			Name:      "operation_duration",
			Help:      "Histogram of GraphQL operations execution duration.",
			Buckets:   prometheus.DefBuckets,
		}, operationLabels)

		cacheLabels := []string{"operation_name"}
		metrics.cacheHits = promauto.NewCounterVec(prometheus.CounterOpts{
			Namespace: ns,
			Subsystem: sub,
			Name:      "cache_hits_total",
			Help:      "Counter of graphql query operations have cache status's hit.",
		}, cacheLabels)

		metrics.cacheMisses = promauto.NewCounterVec(prometheus.CounterOpts{
			Namespace: ns,
			Subsystem: sub,
			Name:      "cache_misses_total",
			Help:      "Counter of graphql query operations have cache status's miss.",
		}, cacheLabels)

		metrics.cachePasses = promauto.NewCounterVec(prometheus.CounterOpts{
			Namespace: ns,
			Subsystem: sub,
			Name:      "cache_passes_total",
			Help:      "Counter of graphql query operations have cache status's pass.",
		}, cacheLabels)
	})
}

type Metrics struct {
	once              sync.Once
	operationInFlight *prometheus.GaugeVec
	operationCount    *prometheus.CounterVec
	operationDuration *prometheus.HistogramVec
	cacheHits         *prometheus.CounterVec
	cacheMisses       *prometheus.CounterVec
	cachePasses       *prometheus.CounterVec
}

type requestMetrics interface {
	addMetricsBeginRequest(*graphql.Request) error
	addMetricsEndRequest(*graphql.Request, time.Duration) error
}

type cacheMetrics interface {
	addMetricsCacheHit(*graphql.Request) error
	addMetricsCacheMiss(*graphql.Request) error
	addMetricsCachePass(*graphql.Request) error
}

func (h *Handler) addMetricsBeginRequest(request *graphql.Request) error {
	labels, err := h.metricsOperationLabels(request)

	if err != nil {
		return err
	}

	h.metrics.operationCount.With(labels).Inc()
	h.metrics.operationInFlight.With(labels).Inc()

	return nil
}

func (h *Handler) addMetricsEndRequest(request *graphql.Request, d time.Duration) error {
	labels, err := h.metricsOperationLabels(request)

	if err != nil {
		return err
	}

	h.metrics.operationInFlight.With(labels).Dec()
	h.metrics.operationDuration.With(labels).Observe(d.Seconds())

	return nil
}

func (h *Handler) addMetricsCacheHit(request *graphql.Request) error {
	labels, err := h.metricsCacheLabels(request)

	if err != nil {
		return err
	}

	h.metrics.cacheHits.With(labels).Inc()

	return nil
}

func (h *Handler) addMetricsCacheMiss(request *graphql.Request) error {
	labels, err := h.metricsCacheLabels(request)

	if err != nil {
		return err
	}

	h.metrics.cacheMisses.With(labels).Inc()

	return nil
}

func (h *Handler) addMetricsCachePass(request *graphql.Request) error {
	labels, err := h.metricsCacheLabels(request)

	if err != nil {
		return err
	}

	h.metrics.cachePasses.With(labels).Inc()

	return nil
}

func (h *Handler) metricsCacheLabels(request *graphql.Request) (map[string]string, error) {
	if !request.IsNormalized() {
		if result, _ := request.Normalize(h.schema); !result.Successful {
			return nil, result.Errors
		}
	}

	labels := map[string]string{
		"operation_name": request.OperationName,
	}

	return labels, nil
}

func (h *Handler) metricsOperationLabels(request *graphql.Request) (map[string]string, error) {
	if !request.IsNormalized() {
		if result, _ := request.Normalize(h.schema); !result.Successful {
			return nil, result.Errors
		}
	}

	labels := map[string]string{
		"operation_name": request.OperationName,
	}

	operationType, _ := request.OperationType()

	switch operationType {
	case graphql.OperationTypeQuery:
		labels["operation_type"] = "query"
	case graphql.OperationTypeMutation:
		labels["operation_type"] = "mutation"
	case graphql.OperationTypeSubscription:
		labels["operation_type"] = "subscription"
	default:
		labels["operation_type"] = "unknown"
	}

	return labels, nil
}
