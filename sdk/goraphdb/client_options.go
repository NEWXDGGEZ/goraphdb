package goraphdb

import (
	"net/http"
	"time"
)

// clientConfig holds all configurable settings for a Client.
type clientConfig struct {
	httpClient *http.Client
	timeout    time.Duration
	headers    http.Header
	userAgent  string

	// Retry policy.
	maxRetries    int
	retryBackoff  time.Duration
	retryMaxWait  time.Duration
	retryStatuses []int // HTTP status codes that trigger a retry.
}

// defaultConfig returns a sensible default configuration.
func defaultConfig() *clientConfig {
	return &clientConfig{
		httpClient: &http.Client{
			Timeout: 30 * time.Second,
			Transport: &http.Transport{
				MaxIdleConns:        100,
				MaxIdleConnsPerHost: 20,
				IdleConnTimeout:     90 * time.Second,
			},
		},
		timeout:       30 * time.Second,
		headers:       make(http.Header),
		userAgent:     "goraphdb-sdk/1.0",
		maxRetries:    0,
		retryBackoff:  100 * time.Millisecond,
		retryMaxWait:  5 * time.Second,
		retryStatuses: []int{502, 503, 504},
	}
}

// Option configures a Client.
type Option func(*clientConfig)

// WithHTTPClient sets a custom *http.Client for all HTTP requests.
// This allows full control over transport, TLS, timeouts, and proxies.
func WithHTTPClient(c *http.Client) Option {
	return func(cfg *clientConfig) {
		cfg.httpClient = c
	}
}

// WithTimeout sets the default timeout for individual requests.
// This is applied as a context deadline when the caller's context has
// no deadline of its own.
func WithTimeout(d time.Duration) Option {
	return func(cfg *clientConfig) {
		cfg.timeout = d
	}
}

// WithHeader adds a custom HTTP header that will be sent with every request.
// Can be called multiple times for different headers.
func WithHeader(key, value string) Option {
	return func(cfg *clientConfig) {
		cfg.headers.Set(key, value)
	}
}

// WithBasicAuth sets HTTP Basic Authentication credentials.
// This is a convenience wrapper for the Authorization header,
// ready for when GoraphDB adds authentication support.
func WithBasicAuth(username, password string) Option {
	return func(cfg *clientConfig) {
		cfg.headers.Set("Authorization", "Basic "+basicAuth(username, password))
	}
}

// WithBearerToken sets a Bearer token for authentication.
func WithBearerToken(token string) Option {
	return func(cfg *clientConfig) {
		cfg.headers.Set("Authorization", "Bearer "+token)
	}
}

// WithUserAgent sets a custom User-Agent header.
func WithUserAgent(ua string) Option {
	return func(cfg *clientConfig) {
		cfg.userAgent = ua
	}
}

// WithRetry configures automatic retry for transient failures.
// maxRetries is the number of retry attempts (0 = no retries, which is the default).
// backoff is the initial delay between retries (doubled on each attempt).
func WithRetry(maxRetries int, backoff time.Duration) Option {
	return func(cfg *clientConfig) {
		cfg.maxRetries = maxRetries
		cfg.retryBackoff = backoff
	}
}

// WithRetryStatuses sets which HTTP status codes trigger a retry.
// Default: [502, 503, 504].
func WithRetryStatuses(statuses ...int) Option {
	return func(cfg *clientConfig) {
		cfg.retryStatuses = statuses
	}
}
