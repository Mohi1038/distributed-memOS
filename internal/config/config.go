package config

import (
	"os"
)

type Config struct {
	PostgresURL      string
	QdrantURL        string
	RedisURL         string
	NATSURL          string
	GRPCPort         string
	MetricsPort      string
	OpenAIAPIKey     string
	EmbeddingModel   string
	UseRealEmbedding bool
}

func Load() *Config {
	return &Config{
		PostgresURL:      getEnv("POSTGRES_URL", "postgres://app_user:app_secure_password@localhost:5432/memos_db?sslmode=disable"),
		QdrantURL:        getEnv("QDRANT_URL", "localhost"),
		RedisURL:         getEnv("REDIS_URL", "localhost:6379"),
		NATSURL:          getEnv("NATS_URL", "nats://localhost:4222"),
		GRPCPort:         getEnv("GRPC_PORT", "50051"),
		MetricsPort:      getEnv("METRICS_PORT", "9090"),
		OpenAIAPIKey:     getEnv("OPENAI_API_KEY", ""),
		EmbeddingModel:   getEnv("EMBEDDING_MODEL", "text-embedding-3-small"),
		UseRealEmbedding: getEnv("USE_REAL_EMBEDDING", "false") == "true",
	}
}

func getEnv(key, fallback string) string {
	if value, ok := os.LookupEnv(key); ok {
		return value
	}
	return fallback
}
