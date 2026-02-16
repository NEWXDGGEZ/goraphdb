package goraphdb_test

import (
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
	"time"

	"github.com/mstrYoda/goraphdb/sdk/goraphdb"
)

// ---------------------------------------------------------------------------
// Test helpers
// ---------------------------------------------------------------------------

// newTestServer creates an httptest.Server that routes requests to the given handler map.
// Keys are "METHOD /path" strings.
func newTestServer(t *testing.T, handlers map[string]http.HandlerFunc) (*httptest.Server, *goraphdb.Client) {
	t.Helper()

	mux := http.NewServeMux()
	for pattern, handler := range handlers {
		mux.HandleFunc(pattern, handler)
	}
	srv := httptest.NewServer(mux)
	t.Cleanup(srv.Close)

	client := goraphdb.New(srv.URL, goraphdb.WithTimeout(5*time.Second))
	return srv, client
}

func writeJSON(w http.ResponseWriter, status int, v any) {
	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(status)
	json.NewEncoder(w).Encode(v)
}

// ---------------------------------------------------------------------------
// Node Tests
// ---------------------------------------------------------------------------

func TestCreateNode(t *testing.T) {
	_, client := newTestServer(t, map[string]http.HandlerFunc{
		"POST /api/nodes": func(w http.ResponseWriter, r *http.Request) {
			var body struct {
				Props map[string]any `json:"props"`
			}
			json.NewDecoder(r.Body).Decode(&body)

			if body.Props["name"] != "Alice" {
				writeJSON(w, 400, map[string]string{"error": "unexpected props"})
				return
			}
			writeJSON(w, 201, map[string]any{"id": 42})
		},
	})

	id, err := client.CreateNode(context.Background(), goraphdb.Props{"name": "Alice"})
	if err != nil {
		t.Fatalf("CreateNode: %v", err)
	}
	if id != 42 {
		t.Fatalf("expected id=42, got %d", id)
	}
}

func TestGetNode(t *testing.T) {
	_, client := newTestServer(t, map[string]http.HandlerFunc{
		"GET /api/nodes/{id}": func(w http.ResponseWriter, r *http.Request) {
			id := r.PathValue("id")
			if id != "42" {
				writeJSON(w, 404, map[string]string{"error": "not found"})
				return
			}
			writeJSON(w, 200, map[string]any{
				"id":     42,
				"labels": []string{"Person"},
				"props":  map[string]any{"name": "Alice"},
			})
		},
	})

	node, err := client.GetNode(context.Background(), 42)
	if err != nil {
		t.Fatalf("GetNode: %v", err)
	}
	if node.ID != 42 {
		t.Fatalf("expected id=42, got %d", node.ID)
	}
	if node.Props["name"] != "Alice" {
		t.Fatalf("expected name=Alice, got %v", node.Props["name"])
	}
	if len(node.Labels) != 1 || node.Labels[0] != "Person" {
		t.Fatalf("expected labels=[Person], got %v", node.Labels)
	}
}

func TestGetNode_NotFound(t *testing.T) {
	_, client := newTestServer(t, map[string]http.HandlerFunc{
		"GET /api/nodes/{id}": func(w http.ResponseWriter, r *http.Request) {
			writeJSON(w, 404, map[string]string{"error": "node not found"})
		},
	})

	_, err := client.GetNode(context.Background(), 999)
	if err == nil {
		t.Fatal("expected error, got nil")
	}
	if !goraphdb.IsNotFound(err) {
		t.Fatalf("expected NotFound error, got %v", err)
	}
}

func TestUpdateNode(t *testing.T) {
	_, client := newTestServer(t, map[string]http.HandlerFunc{
		"PUT /api/nodes/{id}": func(w http.ResponseWriter, r *http.Request) {
			writeJSON(w, 200, map[string]string{"status": "updated"})
		},
	})

	err := client.UpdateNode(context.Background(), 42, goraphdb.Props{"age": 31})
	if err != nil {
		t.Fatalf("UpdateNode: %v", err)
	}
}

func TestDeleteNode(t *testing.T) {
	_, client := newTestServer(t, map[string]http.HandlerFunc{
		"DELETE /api/nodes/{id}": func(w http.ResponseWriter, r *http.Request) {
			writeJSON(w, 200, map[string]string{"status": "deleted"})
		},
	})

	err := client.DeleteNode(context.Background(), 42)
	if err != nil {
		t.Fatalf("DeleteNode: %v", err)
	}
}

func TestListNodesCursor(t *testing.T) {
	_, client := newTestServer(t, map[string]http.HandlerFunc{
		"GET /api/nodes/cursor": func(w http.ResponseWriter, r *http.Request) {
			writeJSON(w, 200, map[string]any{
				"nodes": []map[string]any{
					{"id": 1, "props": map[string]any{"name": "A"}},
					{"id": 2, "props": map[string]any{"name": "B"}},
				},
				"next_cursor": 3,
				"has_more":    true,
				"limit":       2,
			})
		},
	})

	page, err := client.ListNodesCursor(context.Background(), 0, 2)
	if err != nil {
		t.Fatalf("ListNodesCursor: %v", err)
	}
	if len(page.Nodes) != 2 {
		t.Fatalf("expected 2 nodes, got %d", len(page.Nodes))
	}
	if !page.HasMore {
		t.Fatal("expected HasMore=true")
	}
	if page.NextCursor != 3 {
		t.Fatalf("expected NextCursor=3, got %d", page.NextCursor)
	}
}

// ---------------------------------------------------------------------------
// Edge Tests
// ---------------------------------------------------------------------------

func TestCreateEdge(t *testing.T) {
	_, client := newTestServer(t, map[string]http.HandlerFunc{
		"POST /api/edges": func(w http.ResponseWriter, r *http.Request) {
			var body struct {
				From  uint64         `json:"from"`
				To    uint64         `json:"to"`
				Label string         `json:"label"`
				Props map[string]any `json:"props"`
			}
			json.NewDecoder(r.Body).Decode(&body)

			if body.Label != "FOLLOWS" {
				writeJSON(w, 400, map[string]string{"error": "expected FOLLOWS label"})
				return
			}
			writeJSON(w, 201, map[string]any{"id": 100})
		},
	})

	id, err := client.CreateEdge(context.Background(), 1, 2, "FOLLOWS", nil)
	if err != nil {
		t.Fatalf("CreateEdge: %v", err)
	}
	if id != 100 {
		t.Fatalf("expected id=100, got %d", id)
	}
}

func TestDeleteEdge(t *testing.T) {
	_, client := newTestServer(t, map[string]http.HandlerFunc{
		"DELETE /api/edges/{id}": func(w http.ResponseWriter, r *http.Request) {
			writeJSON(w, 200, map[string]string{"status": "deleted"})
		},
	})

	err := client.DeleteEdge(context.Background(), 100)
	if err != nil {
		t.Fatalf("DeleteEdge: %v", err)
	}
}

// ---------------------------------------------------------------------------
// Cypher Tests
// ---------------------------------------------------------------------------

func TestQuery(t *testing.T) {
	_, client := newTestServer(t, map[string]http.HandlerFunc{
		"POST /api/cypher": func(w http.ResponseWriter, r *http.Request) {
			var body struct {
				Query string `json:"query"`
			}
			json.NewDecoder(r.Body).Decode(&body)

			writeJSON(w, 200, map[string]any{
				"columns":    []string{"n.name"},
				"rows":       []map[string]any{{"n.name": "Alice"}, {"n.name": "Bob"}},
				"rowCount":   2,
				"execTimeMs": 1.5,
				"graph":      map[string]any{"nodes": []any{}, "edges": []any{}},
			})
		},
	})

	result, err := client.Query(context.Background(), `MATCH (n) RETURN n.name`)
	if err != nil {
		t.Fatalf("Query: %v", err)
	}
	if result.RowCount != 2 {
		t.Fatalf("expected 2 rows, got %d", result.RowCount)
	}
	if len(result.Columns) != 1 || result.Columns[0] != "n.name" {
		t.Fatalf("unexpected columns: %v", result.Columns)
	}
}

func TestQueryStream(t *testing.T) {
	_, client := newTestServer(t, map[string]http.HandlerFunc{
		"POST /api/cypher/stream": func(w http.ResponseWriter, r *http.Request) {
			w.Header().Set("Content-Type", "application/x-ndjson")
			w.Header().Set("X-Columns", "name,age")
			w.WriteHeader(200)

			flusher, _ := w.(http.Flusher)
			for _, row := range []map[string]any{
				{"name": "Alice", "age": 30},
				{"name": "Bob", "age": 25},
			} {
				json.NewEncoder(w).Encode(row)
				flusher.Flush()
			}
		},
	})

	iter, err := client.QueryStream(context.Background(), `MATCH (n) RETURN n.name, n.age`)
	if err != nil {
		t.Fatalf("QueryStream: %v", err)
	}
	defer iter.Close()

	cols := iter.Columns()
	if len(cols) != 2 || cols[0] != "name" || cols[1] != "age" {
		t.Fatalf("unexpected columns: %v", cols)
	}

	var rows []map[string]any
	for iter.Next() {
		rows = append(rows, iter.Row())
	}
	if err := iter.Err(); err != nil {
		t.Fatalf("iterator error: %v", err)
	}
	if len(rows) != 2 {
		t.Fatalf("expected 2 rows, got %d", len(rows))
	}
	if rows[0]["name"] != "Alice" {
		t.Fatalf("expected Alice, got %v", rows[0]["name"])
	}
}

func TestQueryStream_Collect(t *testing.T) {
	_, client := newTestServer(t, map[string]http.HandlerFunc{
		"POST /api/cypher/stream": func(w http.ResponseWriter, r *http.Request) {
			w.Header().Set("Content-Type", "application/x-ndjson")
			w.Header().Set("X-Columns", "x")
			w.WriteHeader(200)
			for i := 0; i < 5; i++ {
				json.NewEncoder(w).Encode(map[string]any{"x": i})
			}
		},
	})

	iter, err := client.QueryStream(context.Background(), `MATCH (n) RETURN n`)
	if err != nil {
		t.Fatalf("QueryStream: %v", err)
	}
	defer iter.Close()

	rows, err := iter.Collect()
	if err != nil {
		t.Fatalf("Collect: %v", err)
	}
	if len(rows) != 5 {
		t.Fatalf("expected 5 rows, got %d", len(rows))
	}
}

// ---------------------------------------------------------------------------
// Prepared Statement Tests
// ---------------------------------------------------------------------------

func TestPreparedStatement(t *testing.T) {
	_, client := newTestServer(t, map[string]http.HandlerFunc{
		"POST /api/cypher/prepare": func(w http.ResponseWriter, r *http.Request) {
			writeJSON(w, 200, map[string]string{
				"stmt_id": "abc123",
				"query":   "MATCH (n {name: $name}) RETURN n",
				"status":  "prepared",
			})
		},
		"POST /api/cypher/execute": func(w http.ResponseWriter, r *http.Request) {
			var body struct {
				StmtID string         `json:"stmt_id"`
				Params map[string]any `json:"params"`
			}
			json.NewDecoder(r.Body).Decode(&body)

			if body.StmtID != "abc123" {
				writeJSON(w, 404, map[string]string{"error": "statement not found"})
				return
			}

			writeJSON(w, 200, map[string]any{
				"columns":    []string{"n"},
				"rows":       []map[string]any{{"n": map[string]any{"name": body.Params["name"]}}},
				"rowCount":   1,
				"execTimeMs": 0.5,
				"graph":      map[string]any{"nodes": []any{}, "edges": []any{}},
			})
		},
	})

	stmt, err := client.Prepare(context.Background(), `MATCH (n {name: $name}) RETURN n`)
	if err != nil {
		t.Fatalf("Prepare: %v", err)
	}
	if stmt.ID() != "abc123" {
		t.Fatalf("expected stmt_id=abc123, got %s", stmt.ID())
	}

	result, err := stmt.Execute(context.Background(), goraphdb.Props{"name": "Alice"})
	if err != nil {
		t.Fatalf("Execute: %v", err)
	}
	if result.RowCount != 1 {
		t.Fatalf("expected 1 row, got %d", result.RowCount)
	}
}

// ---------------------------------------------------------------------------
// Index Tests
// ---------------------------------------------------------------------------

func TestListIndexes(t *testing.T) {
	_, client := newTestServer(t, map[string]http.HandlerFunc{
		"GET /api/indexes": func(w http.ResponseWriter, r *http.Request) {
			writeJSON(w, 200, map[string]any{"indexes": []string{"name", "age"}})
		},
	})

	indexes, err := client.ListIndexes(context.Background())
	if err != nil {
		t.Fatalf("ListIndexes: %v", err)
	}
	if len(indexes) != 2 {
		t.Fatalf("expected 2 indexes, got %d", len(indexes))
	}
}

func TestCreateIndex(t *testing.T) {
	_, client := newTestServer(t, map[string]http.HandlerFunc{
		"POST /api/indexes": func(w http.ResponseWriter, r *http.Request) {
			writeJSON(w, 201, map[string]string{"status": "created", "property": "email"})
		},
	})

	err := client.CreateIndex(context.Background(), "email")
	if err != nil {
		t.Fatalf("CreateIndex: %v", err)
	}
}

// ---------------------------------------------------------------------------
// Constraint Tests
// ---------------------------------------------------------------------------

func TestCreateConstraint(t *testing.T) {
	_, client := newTestServer(t, map[string]http.HandlerFunc{
		"POST /api/constraints": func(w http.ResponseWriter, r *http.Request) {
			writeJSON(w, 200, map[string]string{"status": "created"})
		},
	})

	err := client.CreateConstraint(context.Background(), "Person", "email")
	if err != nil {
		t.Fatalf("CreateConstraint: %v", err)
	}
}

// ---------------------------------------------------------------------------
// Admin Tests
// ---------------------------------------------------------------------------

func TestHealth(t *testing.T) {
	_, client := newTestServer(t, map[string]http.HandlerFunc{
		"GET /api/health": func(w http.ResponseWriter, r *http.Request) {
			writeJSON(w, 200, map[string]any{
				"status":   "ok",
				"role":     "standalone",
				"readable": true,
				"writable": true,
			})
		},
	})

	health, err := client.Health(context.Background())
	if err != nil {
		t.Fatalf("Health: %v", err)
	}
	if health.Status != "ok" {
		t.Fatalf("expected status=ok, got %s", health.Status)
	}
	if health.Role != "standalone" {
		t.Fatalf("expected role=standalone, got %s", health.Role)
	}
	if !health.Writable {
		t.Fatal("expected writable=true")
	}
}

func TestPing(t *testing.T) {
	_, client := newTestServer(t, map[string]http.HandlerFunc{
		"GET /api/health": func(w http.ResponseWriter, r *http.Request) {
			writeJSON(w, 200, map[string]any{"status": "ok", "role": "standalone", "readable": true, "writable": true})
		},
	})

	err := client.Ping(context.Background())
	if err != nil {
		t.Fatalf("Ping: %v", err)
	}
}

func TestStats(t *testing.T) {
	_, client := newTestServer(t, map[string]http.HandlerFunc{
		"GET /api/stats": func(w http.ResponseWriter, r *http.Request) {
			writeJSON(w, 200, map[string]any{
				"node_count":      1000,
				"edge_count":      5000,
				"shard_count":     4,
				"disk_size_bytes": 1048576,
			})
		},
	})

	stats, err := client.Stats(context.Background())
	if err != nil {
		t.Fatalf("Stats: %v", err)
	}
	if stats.NodeCount != 1000 {
		t.Fatalf("expected 1000 nodes, got %d", stats.NodeCount)
	}
	if stats.EdgeCount != 5000 {
		t.Fatalf("expected 5000 edges, got %d", stats.EdgeCount)
	}
}

func TestClusterStatus(t *testing.T) {
	_, client := newTestServer(t, map[string]http.HandlerFunc{
		"GET /api/cluster": func(w http.ResponseWriter, r *http.Request) {
			writeJSON(w, 200, map[string]any{
				"mode": "standalone",
				"role": "standalone",
			})
		},
	})

	status, err := client.ClusterStatus(context.Background())
	if err != nil {
		t.Fatalf("ClusterStatus: %v", err)
	}
	if status.Mode != "standalone" {
		t.Fatalf("expected mode=standalone, got %s", status.Mode)
	}
}

// ---------------------------------------------------------------------------
// Error Handling Tests
// ---------------------------------------------------------------------------

func TestAPIError_Unwrap(t *testing.T) {
	tests := []struct {
		status int
		check  func(error) bool
		name   string
	}{
		{400, goraphdb.IsBadRequest, "BadRequest"},
		{404, goraphdb.IsNotFound, "NotFound"},
		{409, goraphdb.IsConflict, "Conflict"},
		{500, goraphdb.IsServerError, "ServerError"},
		{503, goraphdb.IsReadOnly, "ReadOnly"},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			_, client := newTestServer(t, map[string]http.HandlerFunc{
				"GET /api/health": func(w http.ResponseWriter, r *http.Request) {
					writeJSON(w, tt.status, map[string]string{"error": "test error"})
				},
			})

			_, err := client.Health(context.Background())
			if err == nil {
				t.Fatal("expected error")
			}
			if !tt.check(err) {
				t.Fatalf("expected %s error, got %v", tt.name, err)
			}
		})
	}
}

func TestNeighborhood(t *testing.T) {
	_, client := newTestServer(t, map[string]http.HandlerFunc{
		"GET /api/nodes/{id}/neighborhood": func(w http.ResponseWriter, r *http.Request) {
			writeJSON(w, 200, map[string]any{
				"center": map[string]any{
					"id": 1, "props": map[string]any{"name": "Alice"}, "label": "Alice",
				},
				"neighbors": []map[string]any{
					{"id": 2, "props": map[string]any{"name": "Bob"}, "label": "Bob"},
				},
				"edges": []map[string]any{
					{"id": 10, "from": 1, "to": 2, "label": "FOLLOWS"},
				},
			})
		},
	})

	hood, err := client.GetNeighborhood(context.Background(), 1)
	if err != nil {
		t.Fatalf("GetNeighborhood: %v", err)
	}
	if hood.Center.Label != "Alice" {
		t.Fatalf("expected center=Alice, got %s", hood.Center.Label)
	}
	if len(hood.Neighbors) != 1 {
		t.Fatalf("expected 1 neighbor, got %d", len(hood.Neighbors))
	}
	if len(hood.Edges) != 1 {
		t.Fatalf("expected 1 edge, got %d", len(hood.Edges))
	}
}

// ---------------------------------------------------------------------------
// Client Configuration Tests
// ---------------------------------------------------------------------------

func TestNew_NormalizesURL(t *testing.T) {
	tests := []struct {
		input    string
		expected string
	}{
		{"http://localhost:8080/", "http://localhost:8080"},
		{"http://localhost:8080", "http://localhost:8080"},
		{"localhost:8080", "http://localhost:8080"},
		{"https://db.example.com/", "https://db.example.com"},
	}

	for _, tt := range tests {
		c := goraphdb.New(tt.input)
		if c.BaseURL() != tt.expected {
			t.Errorf("New(%q).BaseURL() = %q, want %q", tt.input, c.BaseURL(), tt.expected)
		}
	}
}

func TestWithHeader(t *testing.T) {
	var gotHeader string
	_, client := newTestServer(t, map[string]http.HandlerFunc{
		"GET /api/health": func(w http.ResponseWriter, r *http.Request) {
			gotHeader = r.Header.Get("X-Custom")
			writeJSON(w, 200, map[string]any{"status": "ok", "role": "standalone", "readable": true, "writable": true})
		},
	})

	// Recreate with header option.
	client = goraphdb.New(client.BaseURL(), goraphdb.WithHeader("X-Custom", "test-value"))
	client.Health(context.Background())

	if gotHeader != "test-value" {
		t.Fatalf("expected X-Custom=test-value, got %q", gotHeader)
	}
}

func TestWithRetry(t *testing.T) {
	attempts := 0
	_, client := newTestServer(t, map[string]http.HandlerFunc{
		"GET /api/health": func(w http.ResponseWriter, r *http.Request) {
			attempts++
			if attempts < 3 {
				writeJSON(w, 503, map[string]string{"error": "service unavailable"})
				return
			}
			writeJSON(w, 200, map[string]any{"status": "ok", "role": "standalone", "readable": true, "writable": true})
		},
	})

	client = goraphdb.New(client.BaseURL(),
		goraphdb.WithRetry(3, 10*time.Millisecond),
		goraphdb.WithTimeout(10*time.Second),
	)

	health, err := client.Health(context.Background())
	if err != nil {
		t.Fatalf("Health with retry: %v", err)
	}
	if health.Status != "ok" {
		t.Fatalf("expected ok, got %s", health.Status)
	}
	if attempts != 3 {
		t.Fatalf("expected 3 attempts, got %d", attempts)
	}
}

// ---------------------------------------------------------------------------
// ReadOnlyReplica Test
// ---------------------------------------------------------------------------

func TestReadOnlyReplica(t *testing.T) {
	_, client := newTestServer(t, map[string]http.HandlerFunc{
		"POST /api/nodes": func(w http.ResponseWriter, r *http.Request) {
			writeJSON(w, 503, map[string]string{"error": "read-only replica"})
		},
	})

	_, err := client.CreateNode(context.Background(), goraphdb.Props{"name": "test"})
	if err == nil {
		t.Fatal("expected error")
	}
	if !goraphdb.IsReadOnly(err) {
		t.Fatalf("expected read-only error, got %v", err)
	}
}

// ---------------------------------------------------------------------------
// Context Cancellation Test
// ---------------------------------------------------------------------------

func TestContextCancellation(t *testing.T) {
	_, client := newTestServer(t, map[string]http.HandlerFunc{
		"GET /api/health": func(w http.ResponseWriter, r *http.Request) {
			// Simulate slow server.
			select {
			case <-r.Context().Done():
				return
			case <-time.After(5 * time.Second):
			}
			writeJSON(w, 200, map[string]any{"status": "ok"})
		},
	})

	ctx, cancel := context.WithTimeout(context.Background(), 50*time.Millisecond)
	defer cancel()

	_, err := client.Health(ctx)
	if err == nil {
		t.Fatal("expected context deadline error")
	}
	if !strings.Contains(err.Error(), "context deadline exceeded") &&
		!strings.Contains(err.Error(), "context canceled") {
		t.Fatalf("expected context error, got: %v", err)
	}
}
