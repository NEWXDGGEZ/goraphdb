package goraphdb

import (
	"context"
	"net/http"
)

// ---------------------------------------------------------------------------
// Cypher Query Execution
// ---------------------------------------------------------------------------

// Query executes a Cypher query and returns the full result set.
//
// This loads the entire result into memory. For large result sets,
// use QueryStream() instead to process rows one at a time.
//
//	result, err := client.Query(ctx, `MATCH (n:Person) RETURN n.name, n.age LIMIT 100`)
//	for _, row := range result.Rows {
//	    fmt.Println(row["n.name"])
//	}
func (c *Client) Query(ctx context.Context, cypher string) (*CypherResult, error) {
	body := map[string]string{"query": cypher}
	var result CypherResult
	if err := c.doJSON(ctx, http.MethodPost, "/api/cypher", body, &result); err != nil {
		return nil, err
	}
	return &result, nil
}

// QueryStream executes a Cypher query and returns a streaming RowIterator.
//
// The iterator reads NDJSON rows from the server on demand, making it suitable
// for very large result sets that should not be loaded into memory all at once.
//
// The caller MUST close the iterator when done to release the HTTP connection:
//
//	iter, err := client.QueryStream(ctx, `MATCH (n) RETURN n.name`)
//	if err != nil {
//	    log.Fatal(err)
//	}
//	defer iter.Close()
//
//	for iter.Next() {
//	    row := iter.Row()
//	    fmt.Println(row["n.name"])
//	}
//	if err := iter.Err(); err != nil {
//	    log.Fatal(err)
//	}
//
// Optional params are merged into a single parameter map. If provided,
// the query uses parameterized execution on the server.
func (c *Client) QueryStream(ctx context.Context, cypher string, params ...Props) (*RowIterator, error) {
	body := map[string]any{"query": cypher}

	// Merge params if provided.
	if len(params) > 0 {
		merged := make(Props)
		for _, p := range params {
			for k, v := range p {
				merged[k] = v
			}
		}
		if len(merged) > 0 {
			body["params"] = merged
		}
	}

	resp, err := c.doRaw(ctx, http.MethodPost, "/api/cypher/stream", body)
	if err != nil {
		return nil, err
	}

	if err := checkStatus(resp); err != nil {
		resp.Body.Close()
		return nil, err
	}

	return newRowIterator(resp), nil
}
