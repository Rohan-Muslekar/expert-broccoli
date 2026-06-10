package metrics

import (
	"github.com/prometheus/client_golang/prometheus"
	"github.com/prometheus/client_golang/prometheus/promauto"
)

var (
	EventsConsumed = promauto.NewCounter(prometheus.CounterOpts{
		Name: "feature_engine_events_consumed_total",
		Help: "Telemetry events read from Kafka",
	})

	FeaturesPublished = promauto.NewCounter(prometheus.CounterOpts{
		Name: "feature_engine_features_published_total",
		Help: "Feature vectors published to Kafka",
	})

	AlertsFired = promauto.NewCounterVec(prometheus.CounterOpts{
		Name: "feature_engine_alerts_fired_total",
		Help: "Alerts fired by rule name",
	}, []string{"rule"})

	ProcessingDuration = promauto.NewHistogram(prometheus.HistogramOpts{
		Name:    "feature_engine_processing_duration_seconds",
		Help:    "Per-event processing latency",
		Buckets: prometheus.DefBuckets,
	})

	ActivePlayers = promauto.NewGauge(prometheus.GaugeOpts{
		Name: "feature_engine_active_players",
		Help: "Current active player goroutines",
	})
)
