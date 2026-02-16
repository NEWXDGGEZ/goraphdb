package goraphdb

import (
	"context"
	"fmt"
	"net/http"
)

// ---------------------------------------------------------------------------
// Index Management
// ---------------------------------------------------------------------------

// ListIndexes returns the names of all indexes in the database.
func (c *Client) ListIndexes(ctx context.Context) ([]string, error) {
	var resp struct {
		Indexes []string `json:"indexes"`
	}
	if err := c.doJSON(ctx, http.MethodGet, "/api/indexes", nil, &resp); err != nil {
		return nil, err
	}
	return resp.Indexes, nil
}

// CreateIndex creates a secondary index on the given property name.
// Nodes with this property will be indexed for fast lookup.
//
// Returns ErrConflict if the index already exists.
func (c *Client) CreateIndex(ctx context.Context, property string) error {
	body := map[string]string{"property": property}
	return c.doJSON(ctx, http.MethodPost, "/api/indexes", body, nil)
}

// DropIndex removes the index on the given property name.
func (c *Client) DropIndex(ctx context.Context, property string) error {
	return c.doJSON(ctx, http.MethodDelete, fmt.Sprintf("/api/indexes/%s", property), nil, nil)
}

// ReIndex rebuilds the index for the given property name.
// This scans all nodes and repopulates the index from scratch.
func (c *Client) ReIndex(ctx context.Context, property string) error {
	return c.doJSON(ctx, http.MethodPost, fmt.Sprintf("/api/indexes/%s/reindex", property), nil, nil)
}
