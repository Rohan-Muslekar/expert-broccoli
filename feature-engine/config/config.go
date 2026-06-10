package config

import (
	"os"
	"strconv"
	"strings"
)

type Config struct {
	KafkaBrokers     []string
	ConsumeTopic     string
	ProduceTopic     string
	AlertsTopic      string
	ConsumerGroup    string
	MetricsPort      int
	PlayerTimeoutSec int
}

func Load() Config {
	return Config{
		KafkaBrokers:     getEnvStringSlice("KAFKA_BROKERS", "localhost:9092"),
		ConsumeTopic:     getEnvString("CONSUME_TOPIC", "telemetry.raw"),
		ProduceTopic:     getEnvString("PRODUCE_TOPIC", "features.computed"),
		AlertsTopic:      getEnvString("ALERTS_TOPIC", "alerts.detections"),
		ConsumerGroup:    getEnvString("CONSUMER_GROUP", "feature-engine"),
		MetricsPort:      getEnvInt("METRICS_PORT", 9090),
		PlayerTimeoutSec: getEnvInt("PLAYER_TIMEOUT_SEC", 60),
	}
}

func getEnvString(key, fallback string) string {
	val := os.Getenv(key)
	if val == "" {
		return fallback
	}
	return val
}

func getEnvInt(key string, fallback int) int {
	val := os.Getenv(key)
	if val == "" {
		return fallback
	}
	n, err := strconv.Atoi(val)
	if err != nil {
		return fallback
	}
	return n
}

func getEnvStringSlice(key string, fallback string) []string {
	val := os.Getenv(key)
	if val == "" {
		val = fallback
	}
	parts := strings.Split(val, ",")
	result := make([]string, 0, len(parts))
	for _, p := range parts {
		trimmed := strings.TrimSpace(p)
		if trimmed != "" {
			result = append(result, trimmed)
		}
	}
	return result
}
