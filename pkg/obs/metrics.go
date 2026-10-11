package obs

import (
	"net/http"

	"github.com/prometheus/client_golang/prometheus"
	"github.com/prometheus/client_golang/prometheus/collectors"
	"github.com/prometheus/client_golang/prometheus/promhttp"
)

// Metrics holds the Prometheus registry and standard collectors for an iotp service.
type Metrics struct {
	Registry *prometheus.Registry
}

// NewMetrics initializes a fresh Prometheus registry with standard process/Go collectors.
func NewMetrics() *Metrics {
	reg := prometheus.NewRegistry()
	reg.MustRegister(
		collectors.NewProcessCollector(collectors.ProcessCollectorOpts{}),
		collectors.NewGoCollector(),
	)
	return &Metrics{Registry: reg}
}

// Handler returns an HTTP handler for scraping registered Prometheus metrics.
func (m *Metrics) Handler() http.Handler {
	return promhttp.HandlerFor(m.Registry, promhttp.HandlerOpts{
		EnableOpenMetrics: true,
	})
}

// NewCounter registers and returns a new Counter metric with prefix iotp_<service>_<noun>_<unit>.
func (m *Metrics) NewCounter(subsystem, name, help string, labels []string) *prometheus.CounterVec {
	cv := prometheus.NewCounterVec(prometheus.CounterOpts{
		Namespace: "iotp",
		Subsystem: subsystem,
		Name:      name,
		Help:      help,
	}, labels)
	m.Registry.MustRegister(cv)
	return cv
}

// NewGauge registers and returns a new Gauge metric.
func (m *Metrics) NewGauge(subsystem, name, help string, labels []string) *prometheus.GaugeVec {
	gv := prometheus.NewGaugeVec(prometheus.GaugeOpts{
		Namespace: "iotp",
		Subsystem: subsystem,
		Name:      name,
		Help:      help,
	}, labels)
	m.Registry.MustRegister(gv)
	return gv
}

// NewHistogram registers and returns a new Histogram metric.
func (m *Metrics) NewHistogram(subsystem, name, help string, buckets []float64, labels []string) *prometheus.HistogramVec {
	hv := prometheus.NewHistogramVec(prometheus.HistogramOpts{
		Namespace: "iotp",
		Subsystem: subsystem,
		Name:      name,
		Help:      help,
		Buckets:   buckets,
	}, labels)
	m.Registry.MustRegister(hv)
	return hv
}
