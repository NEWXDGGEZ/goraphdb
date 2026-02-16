package goraphdb

import (
	"encoding/json"
	"net/http"
	"strings"
	"sync"
)

// RowIterator streams rows from a GoraphDB NDJSON streaming response.
//
// It implements the same iteration pattern as database/sql.Rows:
//
//	for iter.Next() {
//	    row := iter.Row()
//	    // process row...
//	}
//	if err := iter.Err(); err != nil { ... }
//
// The iterator MUST be closed after use to release the HTTP connection:
//
//	defer iter.Close()
type RowIterator struct {
	resp    *http.Response
	decoder *json.Decoder
	columns []string
	row     map[string]any
	err     error
	closed  bool
	mu      sync.Mutex
}

// newRowIterator creates an iterator from a streaming HTTP response.
// It extracts column names from the X-Columns header.
func newRowIterator(resp *http.Response) *RowIterator {
	var columns []string
	if hdr := resp.Header.Get("X-Columns"); hdr != "" {
		columns = strings.Split(hdr, ",")
	}

	return &RowIterator{
		resp:    resp,
		decoder: json.NewDecoder(resp.Body),
		columns: columns,
	}
}

// Next advances the iterator to the next row.
// Returns true if a row is available via Row(), false when iteration is
// complete or an error has occurred.
func (it *RowIterator) Next() bool {
	it.mu.Lock()
	defer it.mu.Unlock()

	if it.closed || it.err != nil {
		return false
	}

	if !it.decoder.More() {
		return false
	}

	var row map[string]any
	if err := it.decoder.Decode(&row); err != nil {
		it.err = err
		return false
	}

	// Check if the server sent an error as the final NDJSON line.
	if errMsg, ok := row["error"].(string); ok && len(row) == 1 {
		it.err = &APIError{StatusCode: 0, Message: errMsg}
		return false
	}

	it.row = row
	return true
}

// Row returns the current row data. Only valid after Next() returns true.
func (it *RowIterator) Row() map[string]any {
	it.mu.Lock()
	defer it.mu.Unlock()
	return it.row
}

// Columns returns the column names from the query's RETURN clause.
// These are extracted from the X-Columns response header.
func (it *RowIterator) Columns() []string {
	return it.columns
}

// Err returns the first error encountered during iteration, if any.
// Always check Err() after the iteration loop completes.
func (it *RowIterator) Err() error {
	it.mu.Lock()
	defer it.mu.Unlock()
	return it.err
}

// Close releases all resources held by the iterator, including the
// underlying HTTP connection. It is safe to call Close multiple times.
func (it *RowIterator) Close() error {
	it.mu.Lock()
	defer it.mu.Unlock()

	if it.closed {
		return nil
	}
	it.closed = true

	if it.resp != nil && it.resp.Body != nil {
		return it.resp.Body.Close()
	}
	return nil
}

// Collect reads all remaining rows from the iterator into a slice.
// This is a convenience method for when you want streaming parsing
// but still need all results in memory.
//
// The iterator is NOT automatically closed — the caller should still
// defer iter.Close().
func (it *RowIterator) Collect() ([]map[string]any, error) {
	var rows []map[string]any
	for it.Next() {
		rows = append(rows, it.Row())
	}
	if err := it.Err(); err != nil {
		return rows, err
	}
	return rows, nil
}
