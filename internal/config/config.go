package config

import (
	"os"
	"strconv"
	"time"
)

type Config struct {
	Addr             string
	LogLevel         string
	RedisURL         string
	DatabaseURL      string
	OpenAIAPIKey     string
	OpenAIBaseURL    string
	AnthropicAPIKey  string
	OllamaBaseURL    string
	DefaultProvider  string
	OTLPEndpoint     string
	AWSRegion        string
	EncryptionKey    string
	AdminAuthEnabled bool

	// Horizontal scaling features
	UseDistributedCircuitBreaker bool

	// Graceful shutdown
	ShutdownTimeout time.Duration
	DrainTimeout    time.Duration
}

func Load() (*Config, error) {
	cfg := &Config{
		Addr:                         getEnv("ADDR", ":8080"),
		LogLevel:                     getEnv("LOG_LEVEL", "info"),
		RedisURL:                     getEnv("REDIS_URL", ""),
		DatabaseURL:                  getEnv("DATABASE_URL", ""),
		OpenAIAPIKey:                 getEnv("OPENAI_API_KEY", ""),
		OpenAIBaseURL:                getEnv("OPENAI_BASE_URL", "https://api.openai.com/v1"),
		AnthropicAPIKey:              getEnv("ANTHROPIC_API_KEY", ""),
		OllamaBaseURL:                getEnv("OLLAMA_BASE_URL", "http://localhost:11434"),
		DefaultProvider:              getEnv("DEFAULT_PROVIDER", "ollama"),
		OTLPEndpoint:                 getEnv("OTLP_ENDPOINT", ""),
		AWSRegion:                    getEnv("AWS_REGION", ""),
		EncryptionKey:                getEnv("ENCRYPTION_KEY", ""),
		AdminAuthEnabled:             getEnv("ADMIN_AUTH_ENABLED", "false") == "true",
		UseDistributedCircuitBreaker: getEnv("USE_DISTRIBUTED_CB", "false") == "true",
		ShutdownTimeout:              getDurationEnv("SHUTDOWN_TIMEOUT", 30*time.Second),
		DrainTimeout:                 getDurationEnv("DRAIN_TIMEOUT", 15*time.Second),
	}

	return cfg, nil
}

func getEnv(key, defaultValue string) string {
	if value := os.Getenv(key); value != "" {
		return value
	}
	return defaultValue
}

func getDurationEnv(key string, defaultValue time.Duration) time.Duration {
	if value := os.Getenv(key); value != "" {
		if seconds, err := strconv.Atoi(value); err == nil {
			return time.Duration(seconds) * time.Second
		}
	}
	return defaultValue
}
