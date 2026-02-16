package goraphdb

import (
	"context"
	"fmt"
	"net/http"
)

// ---------------------------------------------------------------------------
// Health & Readiness
// ---------------------------------------------------------------------------

// Health returns the health status of the GoraphDB node.
//
// The response includes:
//   - Status: "ok" (fully operational) or "readonly" (no leader/quorum lost)
//   - Role: "leader", "follower", or "standalone"
//   - Readable/Writable flags for routing decisions
func (c *Client) Health(ctx context.Context) (*Health, error) {
	var h Health
	if err := c.doJSON(ctx, http.MethodGet, "/api/health", nil, &h); err != nil {
		return nil, err
	}
	return &h, nil
}

// Ping checks whether the server is reachable and healthy.
// Returns nil if the server responds with a 200 status, or an error otherwise.
func (c *Client) Ping(ctx context.Context) error {
	_, err := c.Health(ctx)
	return err
}

// ---------------------------------------------------------------------------
// Database Statistics
// ---------------------------------------------------------------------------

// Stats returns database statistics including node count, edge count,
// shard count, and on-disk size.
func (c *Client) Stats(ctx context.Context) (*Stats, error) {
	var s Stats
	if err := c.doJSON(ctx, http.MethodGet, "/api/stats", nil, &s); err != nil {
		return nil, err
	}
	return &s, nil
}

// ---------------------------------------------------------------------------
// Metrics & Observability
// ---------------------------------------------------------------------------

// Metrics returns the database metrics as a JSON map.
// For Prometheus text format, use MetricsPrometheus().
func (c *Client) Metrics(ctx context.Context) (map[string]any, error) {
	var m map[string]any
	if err := c.doJSON(ctx, http.MethodGet, "/api/metrics", nil, &m); err != nil {
		return nil, err
	}
	return m, nil
}

// SlowQueries returns the most recent slow query log entries.
// The limit parameter controls how many entries are returned (server default: 50).
func (c *Client) SlowQueries(ctx context.Context, limit int) ([]SlowQuery, error) {
	path := fmt.Sprintf("/api/slow-queries?limit=%d", limit)
	var resp struct {
		Queries []SlowQuery `json:"queries"`
		Count   int         `json:"count"`
	}
	if err := c.doJSON(ctx, http.MethodGet, path, nil, &resp); err != nil {
		return nil, err
	}
	return resp.Queries, nil
}

// CacheStats returns query cache and prepared statement statistics.
func (c *Client) CacheStats(ctx context.Context) (*CacheStats, error) {
	var cs CacheStats
	if err := c.doJSON(ctx, http.MethodGet, "/api/cache/stats", nil, &cs); err != nil {
		return nil, err
	}
	return &cs, nil
}

// ---------------------------------------------------------------------------
// Cluster Information
// ---------------------------------------------------------------------------

// ClusterStatus returns the cluster state of the connected node.
//
// In standalone mode, returns mode="standalone".
// In cluster mode, returns node ID, role, leader ID, and replication state.
func (c *Client) ClusterStatus(ctx context.Context) (*ClusterStatus, error) {
	var cs ClusterStatus
	if err := c.doJSON(ctx, http.MethodGet, "/api/cluster", nil, &cs); err != nil {
		return nil, err
	}
	return &cs, nil
}

// ClusterNodes returns aggregated information about all nodes in the cluster.
// The connected node proxies requests to all peers and returns a combined response.
//
// In standalone mode, returns only the connected node's information.
func (c *Client) ClusterNodes(ctx context.Context) (*ClusterNodesResult, error) {
	var result ClusterNodesResult
	if err := c.doJSON(ctx, http.MethodGet, "/api/cluster/nodes", nil, &result); err != nil {
		return nil, err
	}
	return &result, nil
}
