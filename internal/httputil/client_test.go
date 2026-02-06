package httputil

import (
	"testing"
	"time"
)

func TestDefaultConfig(t *testing.T) {
	cfg := DefaultConfig()

	tests := []struct {
		name     string
		got      time.Duration
		expected time.Duration
	}{
		{"Timeout", cfg.Timeout, 120 * time.Second},
		{"DialTimeout", cfg.DialTimeout, 10 * time.Second},
		{"TLSHandshakeTimeout", cfg.TLSHandshakeTimeout, 10 * time.Second},
		{"ResponseHeaderTimeout", cfg.ResponseHeaderTimeout, 30 * time.Second},
		{"IdleConnTimeout", cfg.IdleConnTimeout, 90 * time.Second},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if tt.got != tt.expected {
				t.Errorf("%s = %v, want %v", tt.name, tt.got, tt.expected)
			}
		})
	}

	if cfg.MaxIdleConns != 100 {
		t.Errorf("MaxIdleConns = %d, want 100", cfg.MaxIdleConns)
	}

	if cfg.MaxIdleConnsPerHost != 10 {
		t.Errorf("MaxIdleConnsPerHost = %d, want 10", cfg.MaxIdleConnsPerHost)
	}
}

func TestNewClient(t *testing.T) {
	cfg := ClientConfig{
		Timeout:               60 * time.Second,
		DialTimeout:           5 * time.Second,
		TLSHandshakeTimeout:   5 * time.Second,
		ResponseHeaderTimeout: 15 * time.Second,
		IdleConnTimeout:       45 * time.Second,
		MaxIdleConns:          50,
		MaxIdleConnsPerHost:   5,
	}

	client := NewClient(cfg)

	if client == nil {
		t.Fatal("NewClient() returned nil")
	}

	if client.Timeout != cfg.Timeout {
		t.Errorf("client.Timeout = %v, want %v", client.Timeout, cfg.Timeout)
	}

	if client.Transport == nil {
		t.Error("client.Transport should not be nil")
	}
}

func TestDefaultClient(t *testing.T) {
	client := DefaultClient()

	if client == nil {
		t.Fatal("DefaultClient() returned nil")
	}

	expectedTimeout := 120 * time.Second
	if client.Timeout != expectedTimeout {
		t.Errorf("DefaultClient().Timeout = %v, want %v", client.Timeout, expectedTimeout)
	}
}

func TestNewClient_CustomConfig(t *testing.T) {
	tests := []struct {
		name    string
		timeout time.Duration
	}{
		{"short timeout", 5 * time.Second},
		{"medium timeout", 60 * time.Second},
		{"long timeout", 300 * time.Second},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			cfg := DefaultConfig()
			cfg.Timeout = tt.timeout

			client := NewClient(cfg)
			if client.Timeout != tt.timeout {
				t.Errorf("Timeout = %v, want %v", client.Timeout, tt.timeout)
			}
		})
	}
}

func TestClientConfig_ZeroValues(t *testing.T) {
	cfg := ClientConfig{} // All zero values

	client := NewClient(cfg)

	if client == nil {
		t.Fatal("NewClient() with zero config returned nil")
	}

	// Zero timeout means no timeout
	if client.Timeout != 0 {
		t.Errorf("Timeout = %v, want 0", client.Timeout)
	}
}
