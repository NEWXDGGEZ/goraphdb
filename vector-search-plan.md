# Vector Search Plan for GoraphDB

## Architecture

In-memory HNSW index + bbolt persistence (per shard):

```
                    ┌─────────────────────────┐
  VectorSearch()    │   In-Memory HNSW Index   │  ← fast ANN queries (~ms)
        │           │   (per-shard, in RAM)     │
        ▼           └────────────┬──────────────┘
   k-nearest IDs                 │ rebuilt on startup
        │           ┌────────────▼──────────────┐
        ▼           │   bbolt "vectors" bucket   │  ← durable storage
   GetNode(id)      │   nodeID → float32 vector  │
   + graph ops      └───────────────────────────┘
```

## Data Layout

New bbolt bucket per shard:

```go
bucketVectors = []byte("vectors")   // nodeID+propName → float32 vector bytes

// Key encoding: nodeID (8 bytes) + propName bytes
// Value: dimension (4 bytes) + float32 array (dim * 4 bytes)
```

The in-memory HNSW index is rebuilt at startup by scanning `bucketVectors` (same pattern as bloom filter rebuild from `adj_out` in `bloom.go`).

## Public API

```go
// Store a vector embedding for a node
db.SetVector(ctx, nodeID, "embedding", []float32{0.1, 0.23, ...})

// Find the 10 nearest neighbors by cosine similarity
results, err := db.VectorSearch(ctx, VectorSearchOptions{
    Property:    "embedding",
    Vector:      queryVector,      // []float32
    K:           10,               // top-k results
    Metric:      CosineSimilarity,
    MinScore:    0.7,              // optional threshold
    LabelFilter: "Product",       // optional: only search nodes with this label
})

// Each result: {NodeID, Score, Node}
for _, r := range results {
    fmt.Printf("Node %d (score: %.3f): %v\n", r.NodeID, r.Score, r.Node.Props)

    // Traverse the graph from similar nodes
    edges, _ := db.OutEdges(r.NodeID, "BOUGHT_BY")
}

// Combined: vector search + graph traversal in Cypher (Phase 4)
// CALL db.vectorSearch('embedding', $queryVec, 10) YIELD node, score
// MATCH (node)-[:SIMILAR_TO]->(rec)
// RETURN rec, score
```

## WAL & Replication

Follows existing WAL/applier pattern:

```go
// New WAL operations (wal_entry.go)
OpSetVector    OpType = iota + 19  // store/update vector
OpDeleteVector                      // remove vector

// Payload structs
type WALSetVector struct {
    NodeID   NodeID    `msgpack:"id"`
    Property string    `msgpack:"prop"`
    Vector   []float32 `msgpack:"vec"`
}

type WALDeleteVector struct {
    NodeID   NodeID `msgpack:"id"`
    Property string `msgpack:"prop"`
}

// In applier.go
case OpSetVector:
    var p WALSetVector
    decodeWALPayload(entry.Payload, &p)
    // Write to bbolt + update in-memory HNSW

case OpDeleteVector:
    var p WALDeleteVector
    decodeWALPayload(entry.Payload, &p)
    // Remove from bbolt + remove from in-memory HNSW
```

## New Files

| New File           | Purpose                                                                  | Estimated Lines |
| ------------------ | ------------------------------------------------------------------------ | --------------- |
| `vector.go`        | HNSW index, distance functions (cosine, euclidean, dot), SetVector, VectorSearch, DeleteVector | ~500-700        |
| `vector_test.go`   | Tests for vector operations                                              | ~300-400        |

## Modified Files

| File             | What Changes                                                  | Size of Change |
| ---------------- | ------------------------------------------------------------- | -------------- |
| `storage.go`     | Add `bucketVectors` bucket name, add to `allBuckets`          | ~3 lines       |
| `types.go`       | Add `VectorSearchResult` type, vector options to `Options`    | ~20 lines      |
| `graphdb.go`     | Init HNSW index on startup, rebuild from bbolt                | ~30-50 lines   |
| `wal_entry.go`   | Add `OpSetVector`, `OpDeleteVector`, payload structs          | ~30 lines      |
| `applier.go`     | Add `applySetVector`, `applyDeleteVector` handlers            | ~40-60 lines   |
| `node.go`        | Optional: auto-delete vector when node is deleted             | ~5 lines       |

## Optional (Cypher Integration)

| File               | What Changes                                  |
| ------------------ | --------------------------------------------- |
| `cypher_parser.go` | Parse `CALL db.vectorSearch(...)` syntax      |
| `cypher_exec.go`   | Execute vector search as a query source       |
| `cypher_ast.go`    | AST node for vector search calls              |

## Go Libraries for HNSW

| Library                | CGo? | Notes                                    |
| ---------------------- | ---- | ---------------------------------------- |
| `github.com/viterin/vek` | No   | SIMD-optimized vector math in pure Go  |
| `github.com/coder/hnsw`  | No   | Pure Go HNSW implementation            |
| Custom HNSW            | No   | ~300-400 lines for a basic implementation |

## Memory Considerations

| Vectors | Dimensions         | Memory (HNSW, M=16) |
| ------- | ------------------ | -------------------- |
| 100K    | 384 (MiniLM)       | ~200 MB              |
| 100K    | 1536 (OpenAI)      | ~700 MB              |
| 1M      | 384                | ~2 GB                |
| 1M      | 1536               | ~7 GB                |

For large datasets, product quantization (PQ) or scalar quantization can reduce memory 4-8x.

## Implementation Phases

| Phase | Work                                           | Files                                  | Time        |
| ----- | ---------------------------------------------- | -------------------------------------- | ----------- |
| 1     | Core vector storage + HNSW index + public API  | `vector.go`, `storage.go`, `types.go`  | **3-5 days** |
| 2     | WAL + replication support                       | `wal_entry.go`, `applier.go`           | **1-2 days** |
| 3     | Tests                                           | `vector_test.go`                       | **2-3 days** |
| 4     | (Optional) Cypher integration                   | `cypher_parser.go`, `cypher_exec.go`, `cypher_ast.go` | **3-5 days** |

**Total estimate: ~1-2 weeks**
