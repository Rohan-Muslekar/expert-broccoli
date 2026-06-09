package config

import (
	"os"
	"strconv"
	"strings"
)

type Config struct {
	Port             int
	TickRate         int
	MaxPlayers       int
	BotCount         int
	BotCheatsEnabled bool
	KafkaEnabled     bool
	KafkaBrokers     []string
	RespawnDelaySec  int
	MetricsEnabled   bool
}

func Load() Config {
	return Config{
		Port:             getEnvInt("PORT", 8080),
		TickRate:         getEnvInt("TICK_RATE", 60),
		MaxPlayers:       getEnvInt("MAX_PLAYERS", 16),
		BotCount:         getEnvInt("BOT_COUNT", 8),
		BotCheatsEnabled: getEnvBool("BOT_CHEATS_ENABLED", false),
		KafkaEnabled:     getEnvBool("KAFKA_ENABLED", true),
		KafkaBrokers:     getEnvStringSlice("KAFKA_BROKERS", "localhost:9092"),
		RespawnDelaySec:  getEnvInt("RESPAWN_DELAY_SEC", 3),
		MetricsEnabled:   getEnvBool("METRICS_ENABLED", true),
	}
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

func getEnvBool(key string, fallback bool) bool {
	val := os.Getenv(key)
	if val == "" {
		return fallback
	}
	b, err := strconv.ParseBool(val)
	if err != nil {
		return fallback
	}
	return b
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
