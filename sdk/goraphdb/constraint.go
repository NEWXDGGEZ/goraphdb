package goraphdb

import (
	"context"
	"fmt"
	"net/http"
)

// ---------------------------------------------------------------------------
// Unique Constraint Management
// ---------------------------------------------------------------------------

// ListConstraints returns all registered unique constraints.
func (c *Client) ListConstraints(ctx context.Context) ([]Constraint, error) {
	var resp struct {
		Constraints []Constraint `json:"constraints"`
	}
	if err := c.doJSON(ctx, http.MethodGet, "/api/constraints", nil, &resp); err != nil {
		return nil, err
	}
	return resp.Constraints, nil
}

// CreateConstraint creates a unique constraint on the given label+property pair.
// This ensures no two nodes with the same label can have the same value for
// the specified property.
//
// Returns ErrConflict if the constraint already exists or would be violated
// by existing data.
func (c *Client) CreateConstraint(ctx context.Context, label, property string) error {
	body := map[string]string{
		"label":    label,
		"property": property,
	}
	return c.doJSON(ctx, http.MethodPost, "/api/constraints", body, nil)
}

// DropConstraint removes the unique constraint on the given label+property pair.
func (c *Client) DropConstraint(ctx context.Context, label, property string) error {
	path := fmt.Sprintf("/api/constraints?label=%s&property=%s", label, property)
	return c.doJSON(ctx, http.MethodDelete, path, nil, nil)
}
