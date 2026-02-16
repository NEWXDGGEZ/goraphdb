package goraphdb

import (
	"context"
	"fmt"
	"net/http"
)

// ---------------------------------------------------------------------------
// Edge CRUD
// ---------------------------------------------------------------------------

// CreateEdge creates a directed, labeled edge between two nodes and returns
// the new edge's ID.
//
//	edgeID, err := client.CreateEdge(ctx, aliceID, bobID, "FOLLOWS", goraphdb.Props{"since": "2024"})
func (c *Client) CreateEdge(ctx context.Context, from, to NodeID, label string, props Props) (EdgeID, error) {
	body := map[string]any{
		"from":  from,
		"to":    to,
		"label": label,
		"props": props,
	}

	var resp struct {
		ID EdgeID `json:"id"`
	}
	if err := c.doJSON(ctx, http.MethodPost, "/api/edges", body, &resp); err != nil {
		return 0, err
	}
	return resp.ID, nil
}

// DeleteEdge removes an edge by its ID.
func (c *Client) DeleteEdge(ctx context.Context, id EdgeID) error {
	return c.doJSON(ctx, http.MethodDelete, fmt.Sprintf("/api/edges/%d", id), nil, nil)
}

// ---------------------------------------------------------------------------
// Edge Listing (cursor pagination)
// ---------------------------------------------------------------------------

// ListEdgesCursor returns a page of edges using cursor-based pagination.
// Pass cursor=0 for the first page. Use the returned NextCursor for subsequent pages.
//
//	page, err := client.ListEdgesCursor(ctx, 0, 50)
//	for page.HasMore {
//	    page, err = client.ListEdgesCursor(ctx, page.NextCursor, 50)
//	}
func (c *Client) ListEdgesCursor(ctx context.Context, cursor EdgeID, limit int) (*EdgePage, error) {
	path := fmt.Sprintf("/api/edges/cursor?cursor=%d&limit=%d", cursor, limit)
	var page EdgePage
	if err := c.doJSON(ctx, http.MethodGet, path, nil, &page); err != nil {
		return nil, err
	}
	return &page, nil
}
