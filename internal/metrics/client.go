package metrics

import (
	"github.com/prometheus/client_golang/prometheus"
	"github.com/prometheus/client_golang/prometheus/promauto"
)

// Client is a prometheus metrics client
type Client interface {
	NewCounter(opts prometheus.CounterOpts) prometheus.Counter
	NewCounterVec(opts prometheus.CounterOpts, labels []string) *prometheus.CounterVec
	NewHistogram(opts prometheus.HistogramOpts) prometheus.Histogram
	NewHistogramVec(opts prometheus.HistogramOpts, labels []string) *prometheus.HistogramVec
}

// NoopMetrics is an empty metrics client that doesn't register to any metrics collector
type NoopMetrics struct {
}

// NewCounter will create an empty prometheus counter but will not register it
func (n *NoopMetrics) NewCounter(opts prometheus.CounterOpts) prometheus.Counter {
	return prometheus.NewCounter(opts)
}

// NewCounterVec will create an empty prometheus counter but will not register it
func (n *NoopMetrics) NewCounterVec(opts prometheus.CounterOpts, labels []string) *prometheus.CounterVec {
	return prometheus.NewCounterVec(opts, labels)
}

// NewHistogram returns a new histogram metric that's registered to the automatic prometheus collector
func (n *NoopMetrics) NewHistogram(opts prometheus.HistogramOpts) prometheus.Histogram {
	return prometheus.NewHistogram(opts)
}

// NewHistogramVec will create an empty prometheus histogram but will not register it
func (n *NoopMetrics) NewHistogramVec(opts prometheus.HistogramOpts, labels []string) *prometheus.HistogramVec {
	return prometheus.NewHistogramVec(opts, labels)
}

// PrometheusMetrics represents a prometheus metrics client
type PrometheusMetrics struct {
}

// NewCounter returns a new counter metric that's registered to the automatic prometheus collector
func (p *PrometheusMetrics) NewCounter(opts prometheus.CounterOpts) prometheus.Counter {
	return promauto.NewCounter(opts)
}

// NewCounterVec returns a new counter that can be partitioned with labels
func (p *PrometheusMetrics) NewCounterVec(opts prometheus.CounterOpts, labels []string) *prometheus.CounterVec {
	return promauto.NewCounterVec(opts, labels)
}

// NewHistogram returns a new histogram metric that's registered to the automatic prometheus collector
func (p *PrometheusMetrics) NewHistogram(opts prometheus.HistogramOpts) prometheus.Histogram {
	return promauto.NewHistogram(opts)
}

// NewHistogramVec returns a new counter that can be partitioned with labels
func (p *PrometheusMetrics) NewHistogramVec(opts prometheus.HistogramOpts, labels []string) *prometheus.HistogramVec {
	return promauto.NewHistogramVec(opts, labels)
}
