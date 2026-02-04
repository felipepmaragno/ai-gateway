package secrets

import (
	"context"
	"encoding/json"
	"fmt"
	"sync"
	"time"

	"github.com/aws/aws-sdk-go-v2/aws"
	"github.com/aws/aws-sdk-go-v2/config"
	"github.com/aws/aws-sdk-go-v2/service/secretsmanager"
)

type SecretStore interface {
	GetSecret(ctx context.Context, name string) (string, error)
	GetSecretJSON(ctx context.Context, name string, v interface{}) error
}

type AWSSecretsManager struct {
	client *secretsmanager.Client
	cache  map[string]*cachedSecret
	mu     sync.RWMutex
	ttl    time.Duration
}

type cachedSecret struct {
	value     string
	expiresAt time.Time
}

func NewAWSSecretsManager(ctx context.Context, region string) (*AWSSecretsManager, error) {
	cfg, err := config.LoadDefaultConfig(ctx, config.WithRegion(region))
	if err != nil {
		return nil, fmt.Errorf("load aws config: %w", err)
	}

	return &AWSSecretsManager{
		client: secretsmanager.NewFromConfig(cfg),
		cache:  make(map[string]*cachedSecret),
		ttl:    5 * time.Minute,
	}, nil
}

func NewAWSSecretsManagerWithConfig(cfg aws.Config) *AWSSecretsManager {
	return &AWSSecretsManager{
		client: secretsmanager.NewFromConfig(cfg),
		cache:  make(map[string]*cachedSecret),
		ttl:    5 * time.Minute,
	}
}

func (s *AWSSecretsManager) GetSecret(ctx context.Context, name string) (string, error) {
	s.mu.RLock()
	if cached, ok := s.cache[name]; ok && time.Now().Before(cached.expiresAt) {
		s.mu.RUnlock()
		return cached.value, nil
	}
	s.mu.RUnlock()

	input := &secretsmanager.GetSecretValueInput{
		SecretId: aws.String(name),
	}

	result, err := s.client.GetSecretValue(ctx, input)
	if err != nil {
		return "", fmt.Errorf("get secret %s: %w", name, err)
	}

	value := ""
	if result.SecretString != nil {
		value = *result.SecretString
	}

	s.mu.Lock()
	s.cache[name] = &cachedSecret{
		value:     value,
		expiresAt: time.Now().Add(s.ttl),
	}
	s.mu.Unlock()

	return value, nil
}

func (s *AWSSecretsManager) GetSecretJSON(ctx context.Context, name string, v interface{}) error {
	secret, err := s.GetSecret(ctx, name)
	if err != nil {
		return err
	}

	return json.Unmarshal([]byte(secret), v)
}

func (s *AWSSecretsManager) SetCacheTTL(ttl time.Duration) {
	s.ttl = ttl
}

func (s *AWSSecretsManager) ClearCache() {
	s.mu.Lock()
	defer s.mu.Unlock()
	s.cache = make(map[string]*cachedSecret)
}

type InMemorySecretStore struct {
	mu      sync.RWMutex
	secrets map[string]string
}

func NewInMemorySecretStore() *InMemorySecretStore {
	return &InMemorySecretStore{
		secrets: make(map[string]string),
	}
}

func (s *InMemorySecretStore) GetSecret(ctx context.Context, name string) (string, error) {
	s.mu.RLock()
	defer s.mu.RUnlock()

	value, ok := s.secrets[name]
	if !ok {
		return "", fmt.Errorf("secret %s not found", name)
	}
	return value, nil
}

func (s *InMemorySecretStore) GetSecretJSON(ctx context.Context, name string, v interface{}) error {
	secret, err := s.GetSecret(ctx, name)
	if err != nil {
		return err
	}
	return json.Unmarshal([]byte(secret), v)
}

func (s *InMemorySecretStore) SetSecret(name, value string) {
	s.mu.Lock()
	defer s.mu.Unlock()
	s.secrets[name] = value
}

func (s *InMemorySecretStore) DeleteSecret(name string) {
	s.mu.Lock()
	defer s.mu.Unlock()
	delete(s.secrets, name)
}
