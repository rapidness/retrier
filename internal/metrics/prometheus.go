package metrics

import (
	"github.com/prometheus/client_golang/prometheus"
)

// Metrics holds all Prometheus metrics for the retry middleware.
// Uses a custom registry to avoid duplicate registration in tests.
type Metrics struct {
	registry          *prometheus.Registry
	retryTotal        prometheus.Counter
	retryByRule       *prometheus.CounterVec
	retrySuccess      prometheus.Counter
	retryExhausted    prometheus.Counter
	retryDelaySeconds prometheus.Histogram
	requestDuration   prometheus.Histogram
	activeRequests    prometheus.Gauge
	logEntriesWritten prometheus.Counter
}

// New creates and registers Prometheus metrics on a custom registry.
func New() *Metrics {
	const subsystem = "retry_middleware"
	reg := prometheus.NewRegistry()

	m := &Metrics{
		registry: reg,
		retryTotal: prometheus.NewCounter(prometheus.CounterOpts{
			Subsystem: subsystem,
			Name:      "retry_total",
			Help:      "Total number of retry attempts",
		}),
		retryByRule: prometheus.NewCounterVec(prometheus.CounterOpts{
			Subsystem: subsystem,
			Name:      "retry_by_rule",
			Help:      "Number of retries triggered by each rule",
		}, []string{"rule"}),
		retrySuccess: prometheus.NewCounter(prometheus.CounterOpts{
			Subsystem: subsystem,
			Name:      "retry_success_total",
			Help:      "Number of retries that eventually succeeded",
		}),
		retryExhausted: prometheus.NewCounter(prometheus.CounterOpts{
			Subsystem: subsystem,
			Name:      "retry_exhausted_total",
			Help:      "Number of retries that exhausted all attempts",
		}),
		retryDelaySeconds: prometheus.NewHistogram(prometheus.HistogramOpts{
			Subsystem: subsystem,
			Name:      "retry_delay_seconds",
			Help:      "Actual backoff delay distribution",
			Buckets:   prometheus.ExponentialBuckets(0.01, 2, 10),
		}),
		requestDuration: prometheus.NewHistogram(prometheus.HistogramOpts{
			Subsystem: subsystem,
			Name:      "request_duration_seconds",
			Help:      "End-to-end request duration",
			Buckets:   prometheus.ExponentialBuckets(0.001, 2, 12),
		}),
		activeRequests: prometheus.NewGauge(prometheus.GaugeOpts{
			Subsystem: subsystem,
			Name:      "active_requests",
			Help:      "Number of requests currently in flight",
		}),
		logEntriesWritten: prometheus.NewCounter(prometheus.CounterOpts{
			Subsystem: subsystem,
			Name:      "log_entries_written_total",
			Help:      "Number of log entries written",
		}),
	}

	reg.MustRegister(
		m.retryTotal,
		m.retryByRule,
		m.retrySuccess,
		m.retryExhausted,
		m.retryDelaySeconds,
		m.requestDuration,
		m.activeRequests,
		m.logEntriesWritten,
	)

	return m
}

// Registry returns the custom Prometheus registry for this metrics instance.
func (m *Metrics) Registry() *prometheus.Registry {
	return m.registry
}

// RetryTotalInc increments the total retry counter.
func (m *Metrics) RetryTotalInc() {
	m.retryTotal.Inc()
}

// RetryByRuleInc increments the per-rule retry counter.
func (m *Metrics) RetryByRuleInc(rule string) {
	m.retryByRule.WithLabelValues(rule).Inc()
}

// RetrySuccessInc increments the retry success counter.
func (m *Metrics) RetrySuccessInc() {
	m.retrySuccess.Inc()
}

// RetryExhaustedInc increments the retry exhausted counter.
func (m *Metrics) RetryExhaustedInc() {
	m.retryExhausted.Inc()
}

// RetryDelayObserve records a retry delay.
func (m *Metrics) RetryDelayObserve(seconds float64) {
	m.retryDelaySeconds.Observe(seconds)
}

// RequestDurationObserve records a request duration.
func (m *Metrics) RequestDurationObserve(seconds float64) {
	m.requestDuration.Observe(seconds)
}

// ActiveRequestsInc increments the active requests gauge.
func (m *Metrics) ActiveRequestsInc() {
	m.activeRequests.Inc()
}

// ActiveRequestsDec decrements the active requests gauge.
func (m *Metrics) ActiveRequestsDec() {
	m.activeRequests.Dec()
}

// LogEntriesWrittenInc increments the log entries counter.
func (m *Metrics) LogEntriesWrittenInc() {
	m.logEntriesWritten.Inc()
}
