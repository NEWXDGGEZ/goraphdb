package controllers

// ============================================================================
// Health Checking
//
// The operator monitors GoraphDB pod health by polling the /api/health
// endpoint on each pod. This is separate from Kubernetes' built-in probes
// (which handle liveness/readiness) — the operator's health checks provide
// richer information for status updates and decision-making.
//
// GoraphDB health endpoint behavior (server/server.go handleHealth()):
//
//   GET /api/health → 200 OK
//   {
//     "status":    "ok" | "readonly",
//     "role":      "leader" | "follower" | "standalone",
//     "readable":  true,
//     "writable":  true | false,
//     "node_id":   "goraphdb-ha-0",      // cluster mode only
//     "leader_id": "goraphdb-ha-2"       // cluster mode only
//   }
//
//   GET /api/health → 503 Service Unavailable
//   {
//     "status": "unavailable",
//     "reason": "database is closed"
//   }
//
// Status meanings:
//   - "ok":          fully operational — reads and writes work
//   - "readonly":    reads work but no leader available (quorum lost)
//   - "unavailable": DB is closed or pod is unreachable
//
// The operator uses this information to:
//   1. Update MemberStatus in the CRD status subresource
//   2. Determine the current leader for pod label updates
//   3. Calculate cluster phase (Running, Degraded, Failed)
//   4. Emit events for operational visibility
//
// GoraphDB cluster endpoint (server/server.go handleClusterStatus()):
//
//   GET /api/cluster → 200 OK
//   {
//     "mode":         "cluster",
//     "node_id":      "goraphdb-ha-0",
//     "role":         "leader",
//     "leader_id":    "goraphdb-ha-0",
//     "wal_last_lsn": 42857,           // leader's latest WAL LSN
//     "applied_lsn":  42855            // follower's replication progress
//   }
//
// The applied_lsn vs wal_last_lsn difference is the replication lag.
// The operator tracks this per-member in MemberStatus.WAL_LSN.
//
// Reference:
//   - server/server.go: handleHealth(), handleClusterStatus()
//   - replication/cluster.go: WALLastLSN(), AppliedLSN()
// ============================================================================

import (
	"context"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"time"

	corev1 "k8s.io/api/core/v1"
	"k8s.io/apimachinery/pkg/types"

	goraphdbv1alpha1 "github.com/mstrYoda/goraphdb/operator/api/v1alpha1"
)

// healthResponse represents the JSON response from GET /api/health.
// Maps to the response structure in server/server.go handleHealth().
type healthResponse struct {
	Status   string `json:"status"`    // "ok", "readonly", "unavailable"
	Role     string `json:"role"`      // "leader", "follower", "standalone"
	Readable bool   `json:"readable"`
	Writable bool   `json:"writable"`
	NodeID   string `json:"node_id"`   // empty in standalone mode
	LeaderID string `json:"leader_id"` // empty if no leader elected
}

// clusterResponse represents the JSON response from GET /api/cluster.
// Maps to the response structure in server/server.go handleClusterStatus().
type clusterResponse struct {
	Mode       string `json:"mode"`         // "standalone" or "cluster"
	NodeID     string `json:"node_id"`
	Role       string `json:"role"`
	LeaderID   string `json:"leader_id"`
	WALLastLSN uint64 `json:"wal_last_lsn"` // leader: latest committed LSN
	AppliedLSN uint64 `json:"applied_lsn"`  // follower: last applied LSN
}

// httpClient is a shared HTTP client with a short timeout for health checks.
// GoraphDB /api/health should respond instantly (it just checks db.IsClosed()
// and cluster state), so 3s is generous.
var httpClient = &http.Client{
	Timeout: 3 * time.Second,
}

// checkPodHealth queries a single pod's /api/health and /api/cluster endpoints.
// Returns the member status for the CRD status update.
//
// The pod's HTTP address is constructed from the headless Service DNS:
//   http://{pod-name}.{headless-svc}.{namespace}.svc.cluster.local:{http-port}
//
// This works because the headless Service has publishNotReadyAddresses: true,
// so DNS resolves even for pods that haven't passed readiness yet.
func (r *GoraphDBClusterReconciler) checkPodHealth(
	ctx context.Context,
	cluster *goraphdbv1alpha1.GoraphDBCluster,
	podOrdinal int,
) goraphdbv1alpha1.MemberStatus {
	name := podName(cluster, podOrdinal)
	member := goraphdbv1alpha1.MemberStatus{
		Name:   name,
		Health: "unknown",
	}

	// First check if the pod exists in Kubernetes.
	var pod corev1.Pod
	if err := r.Get(ctx, types.NamespacedName{Name: name, Namespace: cluster.Namespace}, &pod); err != nil {
		member.Health = "unavailable"
		return member
	}

	member.Ready = isPodReady(&pod)
	if role, ok := pod.Labels[LabelRole]; ok {
		member.Role = role
	}

	// Construct pod HTTP URL via headless Service DNS.
	podURL := fmt.Sprintf("http://%s.%s.%s.svc.cluster.local:%d",
		name,
		headlessServiceName(cluster),
		cluster.Namespace,
		cluster.Spec.Ports.HTTP,
	)

	// Query /api/health.
	health, err := fetchHealth(ctx, podURL)
	if err != nil {
		r.Log.V(1).Info("health check failed", "pod", name, "error", err)
		member.Health = "unavailable"
		return member
	}

	member.Health = health.Status
	member.Role = health.Role

	// Query /api/cluster for replication state (cluster mode only).
	if *cluster.Spec.Replicas > 1 {
		clusterInfo, err := fetchClusterStatus(ctx, podURL)
		if err == nil {
			if clusterInfo.Role == "leader" {
				member.WAL_LSN = clusterInfo.WALLastLSN
			} else {
				member.WAL_LSN = clusterInfo.AppliedLSN
			}
		}
	}

	return member
}

// fetchHealth makes a GET request to a pod's /api/health endpoint.
func fetchHealth(ctx context.Context, baseURL string) (*healthResponse, error) {
	req, err := http.NewRequestWithContext(ctx, "GET", baseURL+"/api/health", nil)
	if err != nil {
		return nil, err
	}

	resp, err := httpClient.Do(req)
	if err != nil {
		return nil, err
	}
	defer resp.Body.Close()

	body, err := io.ReadAll(io.LimitReader(resp.Body, 1<<16)) // 64KB limit
	if err != nil {
		return nil, err
	}

	var health healthResponse
	if err := json.Unmarshal(body, &health); err != nil {
		return nil, fmt.Errorf("invalid health response: %w", err)
	}

	return &health, nil
}

// fetchClusterStatus makes a GET request to a pod's /api/cluster endpoint.
func fetchClusterStatus(ctx context.Context, baseURL string) (*clusterResponse, error) {
	req, err := http.NewRequestWithContext(ctx, "GET", baseURL+"/api/cluster", nil)
	if err != nil {
		return nil, err
	}

	resp, err := httpClient.Do(req)
	if err != nil {
		return nil, err
	}
	defer resp.Body.Close()

	body, err := io.ReadAll(io.LimitReader(resp.Body, 1<<16))
	if err != nil {
		return nil, err
	}

	var cluster clusterResponse
	if err := json.Unmarshal(body, &cluster); err != nil {
		return nil, fmt.Errorf("invalid cluster response: %w", err)
	}

	return &cluster, nil
}
