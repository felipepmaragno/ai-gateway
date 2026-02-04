package config

import (
	"os"
)

type Config struct {
	Addr            string
	LogLevel        string
	RedisURL        string
	DatabaseURL     string
	OpenAIAPIKey    string
	OpenAIBaseURL   string
	OllamaBaseURL   string
	DefaultProvider string
}

func Load() (*Config, error) {
	cfg := &Config{
		Addr:            getEnv("ADDR", ":8080"),
		LogLevel:        getEnv("LOG_LEVEL", "info"),
		RedisURL:        getEnv("REDIS_URL", ""),
		DatabaseURL:     getEnv("DATABASE_URL", ""),
		OpenAIAPIKey:    getEnv("OPENAI_API_KEY", ""),
		OpenAIBaseURL:   getEnv("OPENAI_BASE_URL", "https://api.openai.com/v1"),
		OllamaBaseURL:   getEnv("OLLAMA_BASE_URL", "http://localhost:11434"),
		DefaultProvider: getEnv("DEFAULT_PROVIDER", "ollama"),
	}

	return cfg, nil
}

func getEnv(key, defaultValue string) string {
	if value := os.Getenv(key); value != "" {
		return value
	}
	return defaultValue
}
