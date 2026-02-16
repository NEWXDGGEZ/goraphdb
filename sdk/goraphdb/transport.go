package goraphdb

import (
	"bytes"
	"context"
	"encoding/base64"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"strings"
	"time"
)

// ---------------------------------------------------------------------------
// Internal HTTP transport helpers
// ---------------------------------------------------------------------------

// doJSON performs an HTTP request with JSON body and decodes the JSON response
// into dst. If dst is nil, the response body is discarded.
func (c *Client) doJSON(ctx context.Context, method, path string, body, dst any) error {
	ctx, cancel := c.withTimeout(ctx)
	defer cancel()

	resp, err := c.doRawInner(ctx, method, path, body)
	if err != nil {
		return err
	}
	defer resp.Body.Close()

	if err := checkStatus(resp); err != nil {
		return err
	}

	if dst != nil {
		if err := json.NewDecoder(resp.Body).Decode(dst); err != nil {
			return fmt.Errorf("goraphdb: failed to decode response: %w", err)
		}
	}
	return nil
}

// doRaw performs an HTTP request with optional JSON body and returns the raw
// *http.Response. The caller is responsible for closing the response body.
// This applies the client's default timeout and does NOT check the HTTP
// status code — use checkStatus() for that.
func (c *Client) doRaw(ctx context.Context, method, path string, body any) (*http.Response, error) {
	ctx, cancel := c.withTimeout(ctx)
	// Attach cancel to the response so callers closing the body also cancel.
	resp, err := c.doRawInner(ctx, method, path, body)
	if err != nil {
		cancel()
		return nil, err
	}
	resp.Body = &cancelOnClose{ReadCloser: resp.Body, cancel: cancel}
	return resp, nil
}

// doRawInner is the core HTTP executor without timeout wrapping.
func (c *Client) doRawInner(ctx context.Context, method, path string, body any) (*http.Response, error) {
	url := c.baseURL + path

	var bodyReader io.Reader
	if body != nil {
		data, err := json.Marshal(body)
		if err != nil {
			return nil, fmt.Errorf("goraphdb: failed to marshal request body: %w", err)
		}
		bodyReader = bytes.NewReader(data)
	}

	req, err := http.NewRequestWithContext(ctx, method, url, bodyReader)
	if err != nil {
		return nil, fmt.Errorf("goraphdb: failed to create request: %w", err)
	}

	// Set standard headers.
	if body != nil {
		req.Header.Set("Content-Type", "application/json")
	}
	req.Header.Set("Accept", "application/json")
	req.Header.Set("User-Agent", c.cfg.userAgent)

	// Set custom headers.
	for key, values := range c.cfg.headers {
		for _, v := range values {
			req.Header.Set(key, v)
		}
	}

	// Execute with retry logic.
	var resp *http.Response
	maxAttempts := 1 + c.cfg.maxRetries
	backoff := c.cfg.retryBackoff

	for attempt := 0; attempt < maxAttempts; attempt++ {
		if attempt > 0 {
			// Rebuild the request body for retry.
			if body != nil {
				data, _ := json.Marshal(body)
				bodyReader = bytes.NewReader(data)
			}
			retryReq, err := http.NewRequestWithContext(ctx, method, url, bodyReader)
			if err != nil {
				return nil, fmt.Errorf("goraphdb: failed to create retry request: %w", err)
			}
			retryReq.Header = req.Header
			req = retryReq

			select {
			case <-ctx.Done():
				return nil, ctx.Err()
			case <-time.After(backoff):
			}
			backoff *= 2
			if backoff > c.cfg.retryMaxWait {
				backoff = c.cfg.retryMaxWait
			}
		}

		resp, err = c.cfg.httpClient.Do(req)
		if err != nil {
			if attempt < maxAttempts-1 {
				continue
			}
			return nil, fmt.Errorf("goraphdb: request failed: %w", err)
		}

		// Check if we should retry based on status code.
		if attempt < maxAttempts-1 && shouldRetry(resp.StatusCode, c.cfg.retryStatuses) {
			io.Copy(io.Discard, resp.Body)
			resp.Body.Close()
			continue
		}
		break
	}

	return resp, nil
}

// withTimeout wraps the context with the client's default timeout if the
// context does not already have a deadline. The returned cancel function
// must be called by the caller when the operation completes.
func (c *Client) withTimeout(ctx context.Context) (context.Context, context.CancelFunc) {
	if _, ok := ctx.Deadline(); !ok && c.cfg.timeout > 0 {
		return context.WithTimeout(ctx, c.cfg.timeout)
	}
	return ctx, func() {}
}

// checkStatus inspects the HTTP response and returns an *APIError for
// non-2xx status codes.
func checkStatus(resp *http.Response) error {
	if resp.StatusCode >= 200 && resp.StatusCode < 300 {
		return nil
	}

	// Try to extract the server error message.
	var errBody struct {
		Error string `json:"error"`
	}
	body, _ := io.ReadAll(io.LimitReader(resp.Body, 1<<16)) // 64KB limit
	if len(body) > 0 {
		_ = json.Unmarshal(body, &errBody)
	}

	msg := errBody.Error
	if msg == "" {
		msg = strings.TrimSpace(string(body))
	}
	if msg == "" {
		msg = resp.Status
	}

	return &APIError{
		StatusCode: resp.StatusCode,
		Message:    msg,
	}
}

// shouldRetry reports whether the given status code should be retried.
func shouldRetry(status int, retryStatuses []int) bool {
	for _, s := range retryStatuses {
		if status == s {
			return true
		}
	}
	return false
}

// basicAuth encodes username:password as base64 for HTTP Basic Auth.
func basicAuth(username, password string) string {
	return base64.StdEncoding.EncodeToString([]byte(username + ":" + password))
}

// cancelOnClose wraps an io.ReadCloser and calls a cancel function on Close.
// This ensures the context timeout is released when the response body is closed.
type cancelOnClose struct {
	io.ReadCloser
	cancel context.CancelFunc
}

func (c *cancelOnClose) Close() error {
	c.cancel()
	return c.ReadCloser.Close()
}
