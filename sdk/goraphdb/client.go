package goraphdb

import (
	"strings"
)

// Client is the main entry point for interacting with a GoraphDB server.
//
// Create a client with New() and use the methods to perform operations:
//
//	client := goraphdb.New("http://localhost:8080")
//	id, err := client.CreateNode(ctx, goraphdb.Props{"name": "Alice"})
//
// Client is safe for concurrent use by multiple goroutines.
type Client struct {
	baseURL string
	cfg     *clientConfig
}

// New creates a new GoraphDB client connected to the given server address.
//
// The addr parameter should be the base URL of the GoraphDB server
// (e.g. "http://localhost:8080"). A trailing slash is automatically removed.
//
// Use Option functions to customize the client behavior:
//
//	client := goraphdb.New("http://localhost:8080",
//	    goraphdb.WithTimeout(10 * time.Second),
//	    goraphdb.WithRetry(3, 200 * time.Millisecond),
//	)
func New(addr string, opts ...Option) *Client {
	cfg := defaultConfig()
	for _, opt := range opts {
		opt(cfg)
	}

	// Normalize base URL.
	addr = strings.TrimRight(addr, "/")
	if !strings.HasPrefix(addr, "http://") && !strings.HasPrefix(addr, "https://") {
		addr = "http://" + addr
	}

	return &Client{
		baseURL: addr,
		cfg:     cfg,
	}
}

// BaseURL returns the base URL the client is connected to.
func (c *Client) BaseURL() string {
	return c.baseURL
}
