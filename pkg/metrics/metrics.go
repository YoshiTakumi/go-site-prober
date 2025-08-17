package metrics

import "github.com/prometheus/client_golang/prometheus"

var (
	ProberDuration = prometheus.NewHistogramVec(
		prometheus.HistogramOpts{
			Namespace: "site_prober",
			Name:      "request_duration_seconds",
			Help:      "Duration of probe HTTP requests in seconds.",
			Buckets:   prometheus.DefBuckets,
		},
		[]string{"target", "code"},
	)

	ProberUp = prometheus.NewGaugeVec(
		prometheus.GaugeOpts{
			Namespace: "site_prober",
			Name:      "up",
			Help:      "Whether the last probe for a target succeeded (1) or failed (0).",
		},
		[]string{"target"},
	)

	ConsecutiveFailures = prometheus.NewGaugeVec(
		prometheus.GaugeOpts{
			Namespace: "site_prober",
			Name:      "consecutive_failures",
			Help:      "Number of consecutive failures for a target.",
		},
		[]string{"target"},
	)
)

func MustRegister() {
	prometheus.MustRegister(ProberDuration, ProberUp, ConsecutiveFailures)
}
