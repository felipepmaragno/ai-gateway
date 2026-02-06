package secrets

import (
	"context"
	"testing"
)

func TestNewInMemorySecretStore(t *testing.T) {
	store := NewInMemorySecretStore()
	if store == nil {
		t.Fatal("NewInMemorySecretStore() returned nil")
	}
}

func TestInMemorySecretStore_SetAndGet(t *testing.T) {
	store := NewInMemorySecretStore()
	ctx := context.Background()

	store.SetSecret("api-key", "sk-test-123")

	value, err := store.GetSecret(ctx, "api-key")
	if err != nil {
		t.Fatalf("GetSecret() error = %v", err)
	}
	if value != "sk-test-123" {
		t.Errorf("GetSecret() = %v, want sk-test-123", value)
	}
}

func TestInMemorySecretStore_GetNotFound(t *testing.T) {
	store := NewInMemorySecretStore()
	ctx := context.Background()

	_, err := store.GetSecret(ctx, "nonexistent")
	if err == nil {
		t.Error("GetSecret() should return error for nonexistent secret")
	}
}

func TestInMemorySecretStore_Delete(t *testing.T) {
	store := NewInMemorySecretStore()
	ctx := context.Background()

	store.SetSecret("api-key", "sk-test-123")
	store.DeleteSecret("api-key")

	_, err := store.GetSecret(ctx, "api-key")
	if err == nil {
		t.Error("GetSecret() should return error after delete")
	}
}

func TestInMemorySecretStore_GetSecretJSON(t *testing.T) {
	store := NewInMemorySecretStore()
	ctx := context.Background()

	store.SetSecret("config", `{"api_key": "sk-123", "enabled": true}`)

	var config struct {
		APIKey  string `json:"api_key"`
		Enabled bool   `json:"enabled"`
	}

	err := store.GetSecretJSON(ctx, "config", &config)
	if err != nil {
		t.Fatalf("GetSecretJSON() error = %v", err)
	}

	if config.APIKey != "sk-123" {
		t.Errorf("config.APIKey = %v, want sk-123", config.APIKey)
	}
	if !config.Enabled {
		t.Error("config.Enabled should be true")
	}
}

func TestInMemorySecretStore_GetSecretJSON_InvalidJSON(t *testing.T) {
	store := NewInMemorySecretStore()
	ctx := context.Background()

	store.SetSecret("invalid", "not json")

	var config struct{}
	err := store.GetSecretJSON(ctx, "invalid", &config)
	if err == nil {
		t.Error("GetSecretJSON() should return error for invalid JSON")
	}
}

func TestInMemorySecretStore_GetSecretJSON_NotFound(t *testing.T) {
	store := NewInMemorySecretStore()
	ctx := context.Background()

	var config struct{}
	err := store.GetSecretJSON(ctx, "nonexistent", &config)
	if err == nil {
		t.Error("GetSecretJSON() should return error for nonexistent secret")
	}
}

func TestInMemorySecretStore_Overwrite(t *testing.T) {
	store := NewInMemorySecretStore()
	ctx := context.Background()

	store.SetSecret("key", "value1")
	store.SetSecret("key", "value2")

	value, _ := store.GetSecret(ctx, "key")
	if value != "value2" {
		t.Errorf("GetSecret() = %v, want value2", value)
	}
}

func TestInMemorySecretStore_MultipleSecrets(t *testing.T) {
	store := NewInMemorySecretStore()
	ctx := context.Background()

	secrets := map[string]string{
		"openai":    "sk-openai",
		"anthropic": "sk-anthropic",
		"aws":       "sk-aws",
	}

	for name, value := range secrets {
		store.SetSecret(name, value)
	}

	for name, expected := range secrets {
		value, err := store.GetSecret(ctx, name)
		if err != nil {
			t.Errorf("GetSecret(%s) error = %v", name, err)
		}
		if value != expected {
			t.Errorf("GetSecret(%s) = %v, want %v", name, value, expected)
		}
	}
}
