package config

import (
	"os"
	"testing"
)

func TestLoad_Defaults(t *testing.T) {
	// Clear all env vars
	envVars := []string{
		"ADDR", "LOG_LEVEL", "REDIS_URL", "DATABASE_URL",
		"OPENAI_API_KEY", "OPENAI_BASE_URL", "ANTHROPIC_API_KEY",
		"OLLAMA_BASE_URL", "DEFAULT_PROVIDER", "OTLP_ENDPOINT",
		"AWS_REGION", "ENCRYPTION_KEY", "ADMIN_AUTH_ENABLED",
	}
	for _, v := range envVars {
		os.Unsetenv(v)
	}

	cfg, err := Load()
	if err != nil {
		t.Fatalf("Load() error = %v", err)
	}

	tests := []struct {
		name     string
		got      string
		expected string
	}{
		{"Addr", cfg.Addr, ":8080"},
		{"LogLevel", cfg.LogLevel, "info"},
		{"RedisURL", cfg.RedisURL, ""},
		{"DatabaseURL", cfg.DatabaseURL, ""},
		{"OpenAIAPIKey", cfg.OpenAIAPIKey, ""},
		{"OpenAIBaseURL", cfg.OpenAIBaseURL, "https://api.openai.com/v1"},
		{"AnthropicAPIKey", cfg.AnthropicAPIKey, ""},
		{"OllamaBaseURL", cfg.OllamaBaseURL, "http://localhost:11434"},
		{"DefaultProvider", cfg.DefaultProvider, "ollama"},
		{"OTLPEndpoint", cfg.OTLPEndpoint, ""},
		{"AWSRegion", cfg.AWSRegion, ""},
		{"EncryptionKey", cfg.EncryptionKey, ""},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if tt.got != tt.expected {
				t.Errorf("%s = %q, want %q", tt.name, tt.got, tt.expected)
			}
		})
	}

	if cfg.AdminAuthEnabled {
		t.Error("AdminAuthEnabled should default to false")
	}
}

func TestLoad_FromEnv(t *testing.T) {
	// Set env vars
	os.Setenv("ADDR", ":9090")
	os.Setenv("LOG_LEVEL", "debug")
	os.Setenv("REDIS_URL", "redis://localhost:6379")
	os.Setenv("DATABASE_URL", "postgres://localhost/test")
	os.Setenv("OPENAI_API_KEY", "sk-test-key")
	os.Setenv("OPENAI_BASE_URL", "https://custom.openai.com")
	os.Setenv("ANTHROPIC_API_KEY", "anthropic-key")
	os.Setenv("OLLAMA_BASE_URL", "http://ollama:11434")
	os.Setenv("DEFAULT_PROVIDER", "openai")
	os.Setenv("OTLP_ENDPOINT", "http://jaeger:4317")
	os.Setenv("AWS_REGION", "us-east-1")
	os.Setenv("ENCRYPTION_KEY", "my-secret-key")
	os.Setenv("ADMIN_AUTH_ENABLED", "true")

	defer func() {
		os.Unsetenv("ADDR")
		os.Unsetenv("LOG_LEVEL")
		os.Unsetenv("REDIS_URL")
		os.Unsetenv("DATABASE_URL")
		os.Unsetenv("OPENAI_API_KEY")
		os.Unsetenv("OPENAI_BASE_URL")
		os.Unsetenv("ANTHROPIC_API_KEY")
		os.Unsetenv("OLLAMA_BASE_URL")
		os.Unsetenv("DEFAULT_PROVIDER")
		os.Unsetenv("OTLP_ENDPOINT")
		os.Unsetenv("AWS_REGION")
		os.Unsetenv("ENCRYPTION_KEY")
		os.Unsetenv("ADMIN_AUTH_ENABLED")
	}()

	cfg, err := Load()
	if err != nil {
		t.Fatalf("Load() error = %v", err)
	}

	tests := []struct {
		name     string
		got      string
		expected string
	}{
		{"Addr", cfg.Addr, ":9090"},
		{"LogLevel", cfg.LogLevel, "debug"},
		{"RedisURL", cfg.RedisURL, "redis://localhost:6379"},
		{"DatabaseURL", cfg.DatabaseURL, "postgres://localhost/test"},
		{"OpenAIAPIKey", cfg.OpenAIAPIKey, "sk-test-key"},
		{"OpenAIBaseURL", cfg.OpenAIBaseURL, "https://custom.openai.com"},
		{"AnthropicAPIKey", cfg.AnthropicAPIKey, "anthropic-key"},
		{"OllamaBaseURL", cfg.OllamaBaseURL, "http://ollama:11434"},
		{"DefaultProvider", cfg.DefaultProvider, "openai"},
		{"OTLPEndpoint", cfg.OTLPEndpoint, "http://jaeger:4317"},
		{"AWSRegion", cfg.AWSRegion, "us-east-1"},
		{"EncryptionKey", cfg.EncryptionKey, "my-secret-key"},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if tt.got != tt.expected {
				t.Errorf("%s = %q, want %q", tt.name, tt.got, tt.expected)
			}
		})
	}

	if !cfg.AdminAuthEnabled {
		t.Error("AdminAuthEnabled should be true when ADMIN_AUTH_ENABLED=true")
	}
}

func TestGetEnv(t *testing.T) {
	tests := []struct {
		name         string
		key          string
		envValue     string
		defaultValue string
		expected     string
	}{
		{"env set", "TEST_VAR", "custom", "default", "custom"},
		{"env not set", "TEST_VAR_UNSET", "", "default", "default"},
		{"env empty", "TEST_VAR_EMPTY", "", "default", "default"},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if tt.envValue != "" {
				os.Setenv(tt.key, tt.envValue)
				defer os.Unsetenv(tt.key)
			}

			got := getEnv(tt.key, tt.defaultValue)
			if got != tt.expected {
				t.Errorf("getEnv(%q, %q) = %q, want %q", tt.key, tt.defaultValue, got, tt.expected)
			}
		})
	}
}

func TestAdminAuthEnabled_FalseValues(t *testing.T) {
	falseValues := []string{"false", "0", "no", "FALSE", ""}

	for _, v := range falseValues {
		t.Run("value="+v, func(t *testing.T) {
			if v != "" {
				os.Setenv("ADMIN_AUTH_ENABLED", v)
				defer os.Unsetenv("ADMIN_AUTH_ENABLED")
			}

			cfg, _ := Load()
			if cfg.AdminAuthEnabled {
				t.Errorf("AdminAuthEnabled should be false for value %q", v)
			}
		})
	}
}
