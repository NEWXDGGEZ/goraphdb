// Package goraphdb provides a Go client SDK for the GoraphDB graph database.
//
// The SDK communicates with a GoraphDB server over its HTTP/JSON API and
// provides typed, idiomatic Go access to all database operations including
// node/edge CRUD, Cypher queries, streaming results, prepared statements,
// index management, and cluster administration.
//
// Basic usage:
//
//	client := goraphdb.New("http://localhost:8080")
//	id, err := client.CreateNode(ctx, goraphdb.Props{"name": "Alice"})
//	result, err := client.Query(ctx, `MATCH (n) RETURN n.name LIMIT 10`)
package goraphdb

// ---------------------------------------------------------------------------
// Identifiers
// ---------------------------------------------------------------------------

// NodeID uniquely identifies a node in the graph.
type NodeID = uint64

// EdgeID uniquely identifies an edge in the graph.
type EdgeID = uint64

// Props holds arbitrary key-value properties for nodes and edges.
type Props = map[string]any

// ---------------------------------------------------------------------------
// Core Graph Types
// ---------------------------------------------------------------------------

// Node represents a vertex in the graph with labels and properties.
type Node struct {
	ID     NodeID   `json:"id"`
	Labels []string `json:"labels,omitempty"`
	Props  Props    `json:"props,omitempty"`
}

// Edge represents a directed, labeled relationship between two nodes.
type Edge struct {
	ID    EdgeID `json:"id"`
	From  NodeID `json:"from"`
	To    NodeID `json:"to"`
	Label string `json:"label"`
	Props Props  `json:"props,omitempty"`
}

// ---------------------------------------------------------------------------
// Cypher Result Types
// ---------------------------------------------------------------------------

// CypherResult holds the full result of a Cypher query.
type CypherResult struct {
	Columns    []string         `json:"columns"`
	Rows       []map[string]any `json:"rows"`
	Graph      GraphData        `json:"graph"`
	RowCount   int              `json:"rowCount"`
	ExecTimeMs float64          `json:"execTimeMs"`
}

// GraphData represents a subgraph extracted from Cypher results.
type GraphData struct {
	Nodes []GraphNode `json:"nodes"`
	Edges []GraphEdge `json:"edges"`
}

// GraphNode is a node representation used in graph visualization data.
type GraphNode struct {
	ID    uint64 `json:"id"`
	Props Props  `json:"props"`
	Label string `json:"label"`
}

// GraphEdge is an edge representation used in graph visualization data.
type GraphEdge struct {
	ID    uint64 `json:"id"`
	From  uint64 `json:"from"`
	To    uint64 `json:"to"`
	Label string `json:"label"`
}

// ---------------------------------------------------------------------------
// Pagination Types
// ---------------------------------------------------------------------------

// NodeListResult holds offset-paginated node results.
type NodeListResult struct {
	Nodes  []Node `json:"nodes"`
	Total  uint64 `json:"total"`
	Limit  int    `json:"limit"`
	Offset int    `json:"offset"`
}

// NodePage holds cursor-paginated node results.
type NodePage struct {
	Nodes      []Node `json:"nodes"`
	NextCursor uint64 `json:"next_cursor"`
	HasMore    bool   `json:"has_more"`
	Limit      int    `json:"limit"`
}

// EdgePage holds cursor-paginated edge results.
type EdgePage struct {
	Edges      []Edge `json:"edges"`
	NextCursor uint64 `json:"next_cursor"`
	HasMore    bool   `json:"has_more"`
	Limit      int    `json:"limit"`
}

// ---------------------------------------------------------------------------
// Neighborhood
// ---------------------------------------------------------------------------

// Neighborhood contains a center node, its direct neighbors, and connecting edges.
type Neighborhood struct {
	Center    GraphNode   `json:"center"`
	Neighbors []GraphNode `json:"neighbors"`
	Edges     []GraphEdge `json:"edges"`
}

// ---------------------------------------------------------------------------
// Admin & Observability Types
// ---------------------------------------------------------------------------

// Health represents the health status of a GoraphDB node.
type Health struct {
	Status   string `json:"status"`
	Role     string `json:"role"`
	Readable bool   `json:"readable"`
	Writable bool   `json:"writable"`
	NodeID   string `json:"node_id,omitempty"`
	LeaderID string `json:"leader_id,omitempty"`
}

// Stats holds database statistics.
type Stats struct {
	NodeCount     uint64 `json:"node_count"`
	EdgeCount     uint64 `json:"edge_count"`
	ShardCount    int    `json:"shard_count"`
	DiskSizeBytes int64  `json:"disk_size_bytes"`
}

// SlowQuery represents a slow query log entry.
type SlowQuery struct {
	Query      string  `json:"query"`
	DurationMs float64 `json:"duration_ms"`
	Rows       int     `json:"rows"`
	Timestamp  string  `json:"timestamp"`
}

// CacheStats holds query cache and prepared statement statistics.
type CacheStats struct {
	QueryCache         map[string]any `json:"query_cache"`
	PreparedStatements int            `json:"prepared_statements"`
}

// ClusterStatus represents the cluster state of a single node.
type ClusterStatus struct {
	Mode       string `json:"mode"`
	NodeID     string `json:"node_id,omitempty"`
	Role       string `json:"role"`
	LeaderID   string `json:"leader_id,omitempty"`
	DBRole     string `json:"db_role,omitempty"`
	HTTPAddr   string `json:"http_addr,omitempty"`
	GRPCAddr   string `json:"grpc_addr,omitempty"`
	RaftAddr   string `json:"raft_addr,omitempty"`
	WALLastLSN uint64 `json:"wal_last_lsn,omitempty"`
	AppliedLSN uint64 `json:"applied_lsn,omitempty"`
}

// ClusterNodesResult holds aggregated cluster information.
type ClusterNodesResult struct {
	Mode     string        `json:"mode"`
	Self     string        `json:"self"`
	LeaderID string        `json:"leader_id"`
	Nodes    []ClusterNode `json:"nodes"`
}

// ClusterNode represents a single node in the cluster.
type ClusterNode struct {
	NodeID      string         `json:"node_id"`
	Role        string         `json:"role"`
	Status      string         `json:"status"`
	Readable    bool           `json:"readable"`
	Writable    bool           `json:"writable"`
	HTTPAddr    string         `json:"http_addr"`
	GRPCAddr    string         `json:"grpc_addr,omitempty"`
	RaftAddr    string         `json:"raft_addr,omitempty"`
	Stats       map[string]any `json:"stats,omitempty"`
	Replication map[string]any `json:"replication,omitempty"`
	Metrics     map[string]any `json:"metrics,omitempty"`
	Reachable   bool           `json:"reachable"`
	Error       string         `json:"error,omitempty"`
}

// ---------------------------------------------------------------------------
// Index & Constraint Types
// ---------------------------------------------------------------------------

// Constraint represents a unique constraint on a label+property pair.
type Constraint struct {
	Label    string `json:"label"`
	Property string `json:"property"`
}
