package gbox

import (
	"github.com/jensneuse/graphql-go-tools/pkg/graphql"
	"github.com/prometheus/client_golang/prometheus"
	"github.com/prometheus/client_golang/prometheus/promauto"
	"go.uber.org/zap"
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

		cachingLabels := []string{"operation_name", "status"}
		metrics.cachingCount = promauto.NewCounterVec(prometheus.CounterOpts{
			Namespace: ns,
			Subsystem: sub,
			Name:      "caching_total",
			Help:      "Counter of graphql query operations caching statues.",
		}, cachingLabels)
	})
}

type Metrics struct {
	once              sync.Once
	operationInFlight *prometheus.GaugeVec
	operationCount    *prometheus.CounterVec
	operationDuration *prometheus.HistogramVec
	cachingCount      *prometheus.CounterVec
}

type requestMetrics interface {
	addMetricsBeginRequest(*graphql.Request)
	addMetricsEndRequest(*graphql.Request, time.Duration)
}

type cachingMetrics interface {
	addMetricsCaching(*graphql.Request, CachingStatus)
}

func (h *Handler) addMetricsBeginRequest(request *graphql.Request) {
	labels, err := h.metricsOperationLabels(request)

	if err != nil {
		h.logger.Warn("fail to get metrics operation labels", zap.Error(err))

		return
	}

	h.metrics.operationCount.With(labels).Inc()
	h.metrics.operationInFlight.With(labels).Inc()
}

func (h *Handler) addMetricsEndRequest(request *graphql.Request, d time.Duration) {
	labels, err := h.metricsOperationLabels(request)

	if err != nil {
		h.logger.Warn("fail to get metrics operation labels", zap.Error(err))

		return
	}

	h.metrics.operationInFlight.With(labels).Dec()
	h.metrics.operationDuration.With(labels).Observe(d.Seconds())
}

func (h *Handler) addMetricsCaching(request *graphql.Request, status CachingStatus) {
	labels, err := h.metricsCachingLabels(request, status)

	if err != nil {
		h.logger.Warn("fail to get metrics caching labels", zap.Error(err))

		return
	}

	h.metrics.cachingCount.With(labels).Inc()
}

func (h *Handler) metricsCachingLabels(request *graphql.Request, status CachingStatus) (map[string]string, error) {
	if !request.IsNormalized() {
		if result, _ := request.Normalize(h.schema); !result.Successful {
			return nil, result.Errors
		}
	}

	labels := map[string]string{
		"operation_name": request.OperationName,
		"status":         string(status),
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
