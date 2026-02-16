package goraphdb

import (
	"context"
	"net/http"
)

// ---------------------------------------------------------------------------
// Prepared Statements
// ---------------------------------------------------------------------------

// PreparedStatement represents a pre-parsed Cypher query stored on the server.
//
// Prepared statements avoid repeated parsing overhead for queries that are
// executed many times with different parameters. The server caches the parsed
// AST and assigns a statement ID.
//
// Create with Client.Prepare(), execute with Execute():
//
//	stmt, err := client.Prepare(ctx, `MATCH (n {name: $name}) RETURN n`)
//	result, err := stmt.Execute(ctx, goraphdb.Props{"name": "Alice"})
//
// The statement is tied to the server instance that created it. In a clustered
// environment, the statement is only valid on the node that prepared it.
type PreparedStatement struct {
	client *Client
	stmtID string
	query  string
}

// ID returns the server-assigned statement identifier.
func (ps *PreparedStatement) ID() string { return ps.stmtID }

// Query returns the original Cypher query string.
func (ps *PreparedStatement) Query() string { return ps.query }

// Prepare creates a server-side prepared statement from a Cypher query.
//
// The server parses the query and caches the AST, returning a statement ID.
// Use Execute() on the returned PreparedStatement to run it with parameters.
//
//	stmt, err := client.Prepare(ctx, `MATCH (n {name: $name}) RETURN n`)
//	if err != nil {
//	    log.Fatal(err)
//	}
//	result, err := stmt.Execute(ctx, goraphdb.Props{"name": "Alice"})
func (c *Client) Prepare(ctx context.Context, cypher string) (*PreparedStatement, error) {
	body := map[string]string{"query": cypher}

	var resp struct {
		StmtID string `json:"stmt_id"`
		Query  string `json:"query"`
		Status string `json:"status"`
	}
	if err := c.doJSON(ctx, http.MethodPost, "/api/cypher/prepare", body, &resp); err != nil {
		return nil, err
	}

	return &PreparedStatement{
		client: c,
		stmtID: resp.StmtID,
		query:  resp.Query,
	}, nil
}

// Execute runs the prepared statement with the given parameters and returns
// the full result set.
//
// Parameters are matched to $param placeholders in the original query.
// Pass nil if the query has no parameters.
//
//	result, err := stmt.Execute(ctx, goraphdb.Props{"name": "Alice"})
//	for _, row := range result.Rows {
//	    fmt.Println(row)
//	}
func (ps *PreparedStatement) Execute(ctx context.Context, params Props) (*CypherResult, error) {
	body := map[string]any{
		"stmt_id": ps.stmtID,
	}
	if params != nil {
		body["params"] = params
	}

	var result CypherResult
	if err := ps.client.doJSON(ctx, http.MethodPost, "/api/cypher/execute", body, &result); err != nil {
		return nil, err
	}
	return &result, nil
}
