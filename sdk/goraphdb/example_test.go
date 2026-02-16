package goraphdb_test

import (
	"context"
	"encoding/json"
	"fmt"
	"log"
	"net/http"
	"net/http/httptest"
	"time"

	"github.com/mstrYoda/goraphdb/sdk/goraphdb"
)

// mockServer creates a simple mock GoraphDB server for examples.
func mockServer() *httptest.Server {
	mux := http.NewServeMux()

	mux.HandleFunc("POST /api/nodes", func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		w.WriteHeader(201)
		json.NewEncoder(w).Encode(map[string]any{"id": 1})
	})

	mux.HandleFunc("GET /api/nodes/{id}", func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		json.NewEncoder(w).Encode(map[string]any{
			"id": 1, "labels": []string{"Person"},
			"props": map[string]any{"name": "Alice", "age": 30},
		})
	})

	mux.HandleFunc("POST /api/edges", func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		w.WriteHeader(201)
		json.NewEncoder(w).Encode(map[string]any{"id": 100})
	})

	mux.HandleFunc("POST /api/cypher", func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		json.NewEncoder(w).Encode(map[string]any{
			"columns":    []string{"a.name", "b.name"},
			"rows":       []map[string]any{{"a.name": "Alice", "b.name": "Bob"}},
			"rowCount":   1,
			"execTimeMs": 2.1,
			"graph":      map[string]any{"nodes": []any{}, "edges": []any{}},
		})
	})

	mux.HandleFunc("POST /api/cypher/stream", func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/x-ndjson")
		w.Header().Set("X-Columns", "n.name")
		w.WriteHeader(200)
		for _, name := range []string{"Alice", "Bob", "Charlie"} {
			json.NewEncoder(w).Encode(map[string]any{"n.name": name})
		}
	})

	mux.HandleFunc("GET /api/health", func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		json.NewEncoder(w).Encode(map[string]any{
			"status": "ok", "role": "standalone", "readable": true, "writable": true,
		})
	})

	mux.HandleFunc("GET /api/stats", func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		json.NewEncoder(w).Encode(map[string]any{
			"node_count": 1000, "edge_count": 5000, "shard_count": 4, "disk_size_bytes": 1048576,
		})
	})

	return httptest.NewServer(mux)
}

func Example() {
	srv := mockServer()
	defer srv.Close()

	// Create a client connected to the GoraphDB server.
	client := goraphdb.New(srv.URL,
		goraphdb.WithTimeout(10*time.Second),
	)
	ctx := context.Background()

	// Create nodes.
	alice, err := client.CreateNode(ctx, goraphdb.Props{"name": "Alice", "age": 30})
	if err != nil {
		log.Fatal(err)
	}
	fmt.Printf("Created Alice with ID: %d\n", alice)

	// Read a node back.
	node, err := client.GetNode(ctx, alice)
	if err != nil {
		log.Fatal(err)
	}
	fmt.Printf("Got node: %s (age: %v)\n", node.Props["name"], node.Props["age"])

	// Create an edge.
	bob := uint64(2)
	edgeID, err := client.CreateEdge(ctx, alice, bob, "FOLLOWS", goraphdb.Props{"since": "2024"})
	if err != nil {
		log.Fatal(err)
	}
	fmt.Printf("Created FOLLOWS edge with ID: %d\n", edgeID)

	// Run a Cypher query.
	result, err := client.Query(ctx, `MATCH (a)-[:FOLLOWS]->(b) RETURN a.name, b.name`)
	if err != nil {
		log.Fatal(err)
	}
	for _, row := range result.Rows {
		fmt.Printf("%v follows %v\n", row["a.name"], row["b.name"])
	}

	// Check server health.
	health, err := client.Health(ctx)
	if err != nil {
		log.Fatal(err)
	}
	fmt.Printf("Server status: %s, role: %s\n", health.Status, health.Role)

	// Output:
	// Created Alice with ID: 1
	// Got node: Alice (age: 30)
	// Created FOLLOWS edge with ID: 100
	// Alice follows Bob
	// Server status: ok, role: standalone
}

func Example_streaming() {
	srv := mockServer()
	defer srv.Close()

	client := goraphdb.New(srv.URL)
	ctx := context.Background()

	// Stream large results row by row.
	iter, err := client.QueryStream(ctx, `MATCH (n) RETURN n.name`)
	if err != nil {
		log.Fatal(err)
	}
	defer iter.Close()

	for iter.Next() {
		row := iter.Row()
		fmt.Println(row["n.name"])
	}
	if err := iter.Err(); err != nil {
		log.Fatal(err)
	}

	// Output:
	// Alice
	// Bob
	// Charlie
}

func Example_stats() {
	srv := mockServer()
	defer srv.Close()

	client := goraphdb.New(srv.URL)
	ctx := context.Background()

	stats, err := client.Stats(ctx)
	if err != nil {
		log.Fatal(err)
	}
	fmt.Printf("Nodes: %d, Edges: %d, Disk: %.2f MB\n",
		stats.NodeCount, stats.EdgeCount,
		float64(stats.DiskSizeBytes)/1024/1024)

	// Output:
	// Nodes: 1000, Edges: 5000, Disk: 1.00 MB
}
