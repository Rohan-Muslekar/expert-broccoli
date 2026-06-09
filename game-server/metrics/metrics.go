package metrics

import (
	"github.com/prometheus/client_golang/prometheus"
	"github.com/prometheus/client_golang/prometheus/promauto"
)

var (
	TickDuration = promauto.NewHistogram(prometheus.HistogramOpts{
		Name:    "game_tick_duration_seconds",
		Help:    "Time to process one game tick",
		Buckets: prometheus.DefBuckets,
	})

	PlayersActive = promauto.NewGauge(prometheus.GaugeOpts{
		Name: "game_players_active",
		Help: "Current active players (humans + bots)",
	})

	WebSocketConns = promauto.NewGauge(prometheus.GaugeOpts{
		Name: "game_websocket_connections",
		Help: "Active WebSocket connections",
	})

	TelemetryPublished = promauto.NewCounterVec(prometheus.CounterOpts{
		Name: "telemetry_messages_published_total",
		Help: "Telemetry messages published to Kafka",
	}, []string{"topic"})

	KafkaPublishDuration = promauto.NewHistogram(prometheus.HistogramOpts{
		Name:    "kafka_publish_duration_seconds",
		Help:    "Time to publish a message to Kafka",
		Buckets: prometheus.DefBuckets,
	})

	KafkaPublishErrors = promauto.NewCounter(prometheus.CounterOpts{
		Name: "kafka_publish_errors_total",
		Help: "Total Kafka publish errors",
	})
)
