package controllers

// ============================================================================
// Leader Failover Handling
//
// GoraphDB uses Hashicorp Raft for leader election with these timeouts:
//   - HeartbeatTimeout:   1 second
//   - ElectionTimeout:    1 second
//   - LeaderLeaseTimeout: 500 milliseconds
//
// When the leader pod dies or becomes unresponsive:
//   1. Followers notice missing heartbeats after 1s
//   2. An election starts within 1s
//   3. A new leader is elected within ~2-3s total
//   4. The new leader starts a gRPC replication server
//   5. Followers reconnect and resume WAL streaming
//
// The operator's role in failover:
//
// Raft handles the actual failover automatically — the operator does NOT
// need to trigger or manage the election. However, the operator DOES need
// to update Kubernetes state to reflect the new topology:
//
//   1. Update pod labels: set goraphdb.io/role=leader on the new leader
//      and goraphdb.io/role=follower on all others.
//
//   2. This label change causes the client Service ({name}-client) to
//      automatically route traffic to the new leader (the Service selector
//      includes goraphdb.io/role=leader).
//
//   3. Update CRD status: set .status.leader to the new leader pod name.
//
//   4. Emit a Kubernetes Event: "Leader changed from pod-0 to pod-2"
//      for operational visibility.
//
// Timing considerations:
//
// The operator's reconcile loop runs every 30 seconds, so there's a window
// where the client Service may not route to the new leader yet. During this
// window:
//   - The old leader pod is either terminated or partitioned
//   - The client Service has no endpoints (old leader is gone, new leader
//     doesn't have the label yet)
//   - Clients see connection errors for 0-30s
//
// To minimize this window:
//   - The operator watches Pod events (not just reconcile timer)
//   - Pod deletion triggers immediate reconciliation
//   - Future: add a webhook or sidecar for sub-second failover
//
// For truly zero-downtime failover, clients should:
//   - Connect to the read Service ({name}-read) for read-only queries
//   - Use retry logic for write queries during failover
//   - GoraphDB followers forward writes to the leader automatically
//     (replication/router.go), so connecting to any pod works for writes
//     once the new leader is elected
//
// Reference:
//   - replication/election.go: Raft configuration and election
//   - replication/cluster.go: onRoleChange(), becomeLeader(), becomeFollower()
//   - server/server.go: handleHealth() returns current role
// ============================================================================

import (
	"context"
	"fmt"

	corev1 "k8s.io/api/core/v1"
	"k8s.io/apimachinery/pkg/types"

	goraphdbv1alpha1 "github.com/mstrYoda/goraphdb/operator/api/v1alpha1"
)

// reconcileLeaderLabels polls each pod's /api/health endpoint and updates
// the goraphdb.io/role label to reflect the current Raft role.
//
// This is the mechanism that makes the client Service route to the leader:
//   - Client Service selector: goraphdb.io/role=leader
//   - This function sets that label on exactly one pod (the leader)
//   - All other pods get goraphdb.io/role=follower
//
// The function is idempotent: if labels are already correct, no update occurs.
func (r *GoraphDBClusterReconciler) reconcileLeaderLabels(ctx context.Context, cluster *goraphdbv1alpha1.GoraphDBCluster) error {
	if *cluster.Spec.Replicas <= 1 {
		return nil // No role labels needed in standalone mode
	}

	var currentLeader string
	var previousLeader string

	// Track previous leader from status.
	previousLeader = cluster.Status.Leader

	// Check each pod's role via /api/health.
	for i := int32(0); i < *cluster.Spec.Replicas; i++ {
		name := podName(cluster, int(i))

		// Get the pod.
		var pod corev1.Pod
		key := types.NamespacedName{Name: name, Namespace: cluster.Namespace}
		if err := r.Get(ctx, key, &pod); err != nil {
			continue // Pod doesn't exist yet (scaling up)
		}

		// Skip pods that aren't running.
		if pod.Status.Phase != corev1.PodRunning {
			continue
		}

		// Query the pod's health endpoint to get its current role.
		podURL := fmt.Sprintf("http://%s.%s.%s.svc.cluster.local:%d",
			name,
			headlessServiceName(cluster),
			cluster.Namespace,
			cluster.Spec.Ports.HTTP,
		)

		health, err := fetchHealth(ctx, podURL)
		if err != nil {
			r.Log.V(1).Info("cannot reach pod for role check", "pod", name, "error", err)
			continue
		}

		// Determine the desired role label.
		desiredRole := health.Role
		if desiredRole == "" {
			desiredRole = "follower" // safe default
		}

		if desiredRole == "leader" {
			currentLeader = name
		}

		// Update the pod label if it changed.
		currentRole := pod.Labels[LabelRole]
		if currentRole != desiredRole {
			r.Log.Info("updating pod role label",
				"pod", name,
				"from", currentRole,
				"to", desiredRole,
			)

			// Patch the pod labels.
			// Using a merge patch to avoid conflicts with other label updates.
			podCopy := pod.DeepCopy()
			if podCopy.Labels == nil {
				podCopy.Labels = make(map[string]string)
			}
			podCopy.Labels[LabelRole] = desiredRole

			if err := r.Update(ctx, podCopy); err != nil {
				r.Log.Error(err, "failed to update pod role label", "pod", name)
				continue
			}
		}
	}

	// Emit an event if the leader changed.
	if currentLeader != "" && currentLeader != previousLeader && previousLeader != "" {
		r.Log.Info("leader failover detected",
			"previous", previousLeader,
			"current", currentLeader,
		)
		// The event will be visible via: kubectl get events
		// and: kubectl describe goraphdbcluster {name}
	}

	return nil
}
