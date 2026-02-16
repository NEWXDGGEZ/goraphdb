package goraphdb

import (
	"context"
	"fmt"
	"net/http"
)

// ---------------------------------------------------------------------------
// Node CRUD
// ---------------------------------------------------------------------------

// CreateNode creates a new node with the given properties and returns its ID.
//
//	id, err := client.CreateNode(ctx, goraphdb.Props{"name": "Alice", "age": 30})
func (c *Client) CreateNode(ctx context.Context, props Props) (NodeID, error) {
	body := map[string]any{"props": props}

	var resp struct {
		ID NodeID `json:"id"`
	}
	if err := c.doJSON(ctx, http.MethodPost, "/api/nodes", body, &resp); err != nil {
		return 0, err
	}
	return resp.ID, nil
}

// GetNode retrieves a node by its ID.
//
// Returns ErrNotFound if the node does not exist.
func (c *Client) GetNode(ctx context.Context, id NodeID) (*Node, error) {
	var node Node
	if err := c.doJSON(ctx, http.MethodGet, fmt.Sprintf("/api/nodes/%d", id), nil, &node); err != nil {
		return nil, err
	}
	return &node, nil
}

// UpdateNode updates the properties of an existing node.
// The provided props are merged with existing properties on the server.
//
// Returns ErrNotFound if the node does not exist.
func (c *Client) UpdateNode(ctx context.Context, id NodeID, props Props) error {
	body := map[string]any{"props": props}
	return c.doJSON(ctx, http.MethodPut, fmt.Sprintf("/api/nodes/%d", id), body, nil)
}

// DeleteNode removes a node by its ID.
//
// Returns ErrNotFound if the node does not exist.
func (c *Client) DeleteNode(ctx context.Context, id NodeID) error {
	return c.doJSON(ctx, http.MethodDelete, fmt.Sprintf("/api/nodes/%d", id), nil, nil)
}

// ---------------------------------------------------------------------------
// Node Listing (offset pagination)
// ---------------------------------------------------------------------------

// ListNodes returns a page of nodes using offset-based pagination.
//
//	result, err := client.ListNodes(ctx, 50, 0)
//	for _, n := range result.Nodes {
//	    fmt.Printf("Node %d: %v\n", n.ID, n.Props)
//	}
func (c *Client) ListNodes(ctx context.Context, limit, offset int) (*NodeListResult, error) {
	path := fmt.Sprintf("/api/nodes?limit=%d&offset=%d", limit, offset)
	var result NodeListResult
	if err := c.doJSON(ctx, http.MethodGet, path, nil, &result); err != nil {
		return nil, err
	}
	return &result, nil
}

// ---------------------------------------------------------------------------
// Node Listing (cursor pagination)
// ---------------------------------------------------------------------------

// ListNodesCursor returns a page of nodes using cursor-based pagination.
// Pass cursor=0 for the first page. Use the returned NextCursor for subsequent pages.
//
//	page, err := client.ListNodesCursor(ctx, 0, 50)
//	for page.HasMore {
//	    page, err = client.ListNodesCursor(ctx, page.NextCursor, 50)
//	}
func (c *Client) ListNodesCursor(ctx context.Context, cursor NodeID, limit int) (*NodePage, error) {
	path := fmt.Sprintf("/api/nodes/cursor?cursor=%d&limit=%d", cursor, limit)
	var page NodePage
	if err := c.doJSON(ctx, http.MethodGet, path, nil, &page); err != nil {
		return nil, err
	}
	return &page, nil
}

// ---------------------------------------------------------------------------
// Neighborhood
// ---------------------------------------------------------------------------

// GetNeighborhood returns a node, its direct neighbors, and all connecting edges.
// This is ideal for graph visualization of a node's local subgraph.
func (c *Client) GetNeighborhood(ctx context.Context, id NodeID) (*Neighborhood, error) {
	path := fmt.Sprintf("/api/nodes/%d/neighborhood", id)
	var result Neighborhood
	if err := c.doJSON(ctx, http.MethodGet, path, nil, &result); err != nil {
		return nil, err
	}
	return &result, nil
}
