// Package httputil provides a pre-configured HTTP client with proper timeouts.
// All LLM providers should use this client to ensure consistent timeout behavior.
package httputil

import (
	"net"
	"net/http"
	"time"
)

// ClientConfig defines timeout and connection pool settings for HTTP clients.
type ClientConfig struct {
	Timeout               time.Duration // Total request timeout
	DialTimeout           time.Duration // TCP connection timeout
	TLSHandshakeTimeout   time.Duration // TLS negotiation timeout
	ResponseHeaderTimeout time.Duration // Time to wait for response headers
	IdleConnTimeout       time.Duration // Keep-alive connection timeout
	MaxIdleConns          int           // Max idle connections across all hosts
	MaxIdleConnsPerHost   int           // Max idle connections per host
}

// DefaultConfig returns production-ready timeout settings.
func DefaultConfig() ClientConfig {
	return ClientConfig{
		Timeout:               120 * time.Second,
		DialTimeout:           10 * time.Second,
		TLSHandshakeTimeout:   10 * time.Second,
		ResponseHeaderTimeout: 30 * time.Second,
		IdleConnTimeout:       90 * time.Second,
		MaxIdleConns:          100,
		MaxIdleConnsPerHost:   10,
	}
}

// NewClient creates an HTTP client with the specified configuration.
func NewClient(cfg ClientConfig) *http.Client {
	transport := &http.Transport{
		DialContext: (&net.Dialer{
			Timeout:   cfg.DialTimeout,
			KeepAlive: 30 * time.Second,
		}).DialContext,
		TLSHandshakeTimeout:   cfg.TLSHandshakeTimeout,
		ResponseHeaderTimeout: cfg.ResponseHeaderTimeout,
		IdleConnTimeout:       cfg.IdleConnTimeout,
		MaxIdleConns:          cfg.MaxIdleConns,
		MaxIdleConnsPerHost:   cfg.MaxIdleConnsPerHost,
		ForceAttemptHTTP2:     true,
	}

	return &http.Client{
		Timeout:   cfg.Timeout,
		Transport: transport,
	}
}

// DefaultClient returns an HTTP client with production-ready settings.
func DefaultClient() *http.Client {
	return NewClient(DefaultConfig())
}
