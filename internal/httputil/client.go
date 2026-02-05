package httputil

import (
	"net"
	"net/http"
	"time"
)

type ClientConfig struct {
	Timeout             time.Duration
	DialTimeout         time.Duration
	TLSHandshakeTimeout time.Duration
	ResponseHeaderTimeout time.Duration
	IdleConnTimeout     time.Duration
	MaxIdleConns        int
	MaxIdleConnsPerHost int
}

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

func DefaultClient() *http.Client {
	return NewClient(DefaultConfig())
}
