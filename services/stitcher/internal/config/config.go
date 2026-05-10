package config

import (
	"os"
	"strconv"
	"time"
)

type Config struct {
	Environment   string
	KafkaBrokers  string
	ConsumerGroup string
	AtomicLibPath string
	LogLevel      string
	BuildTimeout  time.Duration
}

func Load() *Config {
	return &Config{
		Environment:   getEnvOrDefault("APP_ENV", "development"),
		KafkaBrokers:  getEnvOrDefault("KAFKA_BROKER", "localhost:9092"),
		ConsumerGroup: getEnvOrDefault("KAFKA_CONSUMER_GROUP", "archon-stitcher"),
		AtomicLibPath: getEnvOrDefault("ATOMIC_LIBRARY_PATH", "../../atomic-library"),
		LogLevel:      getEnvOrDefault("LOG_LEVEL", "info"),
		BuildTimeout:  time.Duration(getEnvAsIntOrDefault("BUILD_TIMEOUT_SEC", 300)) * time.Second,
	}
}

func getEnvOrDefault(key, fallback string) string {
	if value, exists := os.LookupEnv(key); exists {
		return value
	}
	return fallback
}

func getEnvAsIntOrDefault(key string, fallback int) int {
	valStr := getEnvOrDefault(key, "")
	if val, err := strconv.Atoi(valStr); err == nil {
		return val
	}
	return fallback
}