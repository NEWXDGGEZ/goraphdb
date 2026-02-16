# GoraphDB Kubernetes Operator

A Kubernetes operator for deploying and managing [GoraphDB](https://github.com/mstrYoda/goraphdb) graph database clusters on Kubernetes.

## Overview

The GoraphDB operator automates the deployment, scaling, and lifecycle management of GoraphDB clusters as a Kubernetes-native experience. It watches `GoraphDBCluster` custom resources and reconciles them into the appropriate Kubernetes objects.

### What the Operator Manages

| Kubernetes Object | Purpose |
|---|---|
| **StatefulSet** | One pod per database node with stable network identity and persistent storage |
| **Headless Service** | DNS-based peer discovery for Raft election and gRPC WAL replication |
| **Client Service** | Leader-only routing for read/write traffic (via `goraphdb.io/role=leader` label selector) |
| **Read Service** | All-replica routing for read-only traffic (load-balanced across all healthy pods) |
| **ConfigMap** | Entrypoint script that computes cluster topology from pod hostname + DNS |
| **PodDisruptionBudget** | Protects Raft quorum during voluntary disruptions (node drains, upgrades) |
| **ServiceMonitor** | Prometheus metrics scraping of GoraphDB's `/metrics` endpoint |

### Architecture

```
                    GoraphDBCluster CR
                          │
                    ┌─────┴─────┐
                    │  Operator  │
                    │ Controller │
                    └─────┬─────┘
          ┌───────────────┼───────────────┐
          │               │               │
    ┌─────┴─────┐   ┌────┴────┐   ┌──────┴──────┐
    │StatefulSet│   │Services │   │  ConfigMap   │
    │           │   │ (3x)    │   │  + PDB       │
    └─────┬─────┘   └─────────┘   └─────────────┘
          │
    ┌─────┼─────┐
    │     │     │
   pod-0 pod-1 pod-2
  LEADER FOLLOWER FOLLOWER
    │     ▲     ▲
    │     │     │
    └─WAL Replication──┘
      (gRPC StreamWAL)
```

## Quick Start

### Prerequisites

- Kubernetes 1.28+
- `kubectl` configured to access your cluster
- (Optional) Prometheus Operator for metrics

### Install the Operator

```bash
# Install the CRD
kubectl apply -f config/crd/

# Create namespace and deploy the operator
kubectl create namespace goraphdb-system
kubectl apply -f config/rbac/ -n goraphdb-system
kubectl apply -f config/manager/ -n goraphdb-system
```

Or use the Makefile:

```bash
make deploy
```

### Deploy a Single-Node Instance (Development)

```bash
kubectl apply -f config/samples/single-node.yaml
```

This creates a standalone GoraphDB instance (no Raft, no WAL) suitable for development.

### Deploy a 3-Node HA Cluster

```bash
kubectl apply -f config/samples/three-node-cluster.yaml
```

This creates a 3-node cluster with:
- Raft-based leader election (tolerates 1 failure)
- WAL replication from leader to followers
- Automatic write forwarding from followers to leader

### Verify the Deployment

```bash
# Check cluster status
kubectl get goraphdbclusters
# or shorthand:
kubectl get gdb

# Expected output:
# NAME          PHASE     READY   LEADER           AGE
# goraphdb-ha   Running   3       goraphdb-ha-0    5m

# Check pods
kubectl get pods -l app.kubernetes.io/name=goraphdb

# Check services
kubectl get svc -l app.kubernetes.io/part-of=goraphdb

# Query the database via the client Service
kubectl port-forward svc/goraphdb-ha-client 7474:7474
curl http://localhost:7474/api/health
curl -X POST http://localhost:7474/api/cypher -d '{"query":"MATCH (n) RETURN n LIMIT 10"}'
```

## CRD Reference

### GoraphDBCluster Spec

Every field maps to a specific GoraphDB configuration option. The table below shows the mapping:

| CRD Field | GoraphDB Source | Default | Description |
|---|---|---|---|
| `replicas` | StatefulSet replicas | `1` | Cluster size (must be odd: 1, 3, 5) |
| `image` | Container image | — | GoraphDB server image (cmd/graphdb-ui) |
| `shardCount` | `Options.ShardCount` | `1` | Number of bbolt shards per node |
| `workerPoolSize` | `Options.WorkerPoolSize` | `8` | Concurrent query goroutines |
| `cacheBudget` | `Options.CacheBudget` | `128Mi` | Node LRU cache memory budget |
| `mmapSize` | `Options.MmapSize` | `256Mi` | Initial bbolt mmap size |
| `noSync` | `Options.NoSync` | `true` | Disable per-tx fsync (200ms background sync) |
| `compactionInterval` | `Options.CompactionInterval` | `0s` | Background compaction interval |
| `slowQueryThreshold` | `Options.SlowQueryThreshold` | `100ms` | Slow query logging threshold |
| `maxResultRows` | `Options.MaxResultRows` | `0` | Max rows per query (0 = unlimited) |
| `defaultQueryTimeout` | `Options.DefaultQueryTimeout` | `0s` | Default query timeout |
| `writeQueueSize` | `Options.WriteQueueSize` | `64` | Max concurrent writes per shard |
| `writeTimeout` | `Options.WriteTimeout` | `5s` | Write queue wait timeout |
| `storage.dataSize` | PVC for `-db` directory | `10Gi` | Shard data volume size |
| `storage.walSize` | PVC for `wal/` directory | — | WAL volume (optional, uses data vol) |
| `storage.raftSize` | PVC for `raft/` directory | — | Raft state volume (optional) |
| `storage.storageClassName` | PVC StorageClass | — | Storage class for all PVCs |
| `ports.http` | `-addr` flag | `7474` | HTTP API port |
| `ports.raft` | `-raft-addr` flag | `7000` | Raft transport port |
| `ports.grpc` | `-grpc-addr` flag | `7001` | gRPC replication port |

### Status Fields

| Field | Source | Description |
|---|---|---|
| `phase` | Operator logic | Lifecycle: Creating → Bootstrapping → Running → Degraded → Failed |
| `readyReplicas` | StatefulSet status | Pods passing readiness probe |
| `leader` | `GET /api/health` | Current Raft leader pod name |
| `members[].role` | `GET /api/health` | Per-pod role: leader/follower/standalone |
| `members[].walLSN` | `GET /api/cluster` | WAL LSN (replication progress) |
| `members[].health` | `GET /api/health` | Health: ok/readonly/unavailable |

## Service Topology

The operator creates three Services for each cluster:

### Headless Service (`{name}-headless`)

- **Type:** `ClusterIP: None`
- **Purpose:** Stable per-pod DNS for Raft and gRPC peer discovery
- **DNS:** `{pod}.{name}-headless.{ns}.svc.cluster.local`
- **Ports:** HTTP (7474), Raft (7000), gRPC (7001)
- **Special:** `publishNotReadyAddresses: true` (required for bootstrap)

### Client Service (`{name}-client`)

- **Type:** `ClusterIP`
- **Purpose:** Route all client traffic to the current Raft leader
- **Selector:** `goraphdb.io/role=leader` (updated by operator)
- **Ports:** HTTP (7474)
- **Use for:** All read/write operations via a single endpoint

### Read Service (`{name}-read`)

- **Type:** `ClusterIP`
- **Purpose:** Load-balance reads across all replicas
- **Selector:** All healthy pods (no role filter)
- **Ports:** HTTP (7474)
- **Use for:** Read-heavy workloads that can tolerate slight replication lag

## Health Probes

The operator configures three Kubernetes health probes hitting `GET /api/health`:

| Probe | Initial Delay | Period | Failure Threshold | Total Budget |
|---|---|---|---|---|
| **Startup** | 5s | 5s | 60 | 5 minutes (for initial data loading) |
| **Liveness** | 30s | 10s | 6 | 60s before restart |
| **Readiness** | 10s | 5s | 3 | 15s before endpoint removal |

GoraphDB's `/api/health` endpoint returns:
- `200 OK` with `{"status":"ok","role":"leader|follower|standalone"}`
- `503` with `{"status":"unavailable"}` when the database is closed

## Monitoring

### Prometheus

If the Prometheus Operator is installed, the operator creates a `ServiceMonitor` that scrapes GoraphDB's `/metrics` endpoint.

```bash
# Deploy monitoring resources
make deploy-monitoring
```

Key metrics to watch:
- `graphdb_queries_total` — query throughput
- `graphdb_slow_queries_total` — performance issues
- `graphdb_cache_hits_total / (hits + misses)` — cache effectiveness
- `graphdb_nodes_current`, `graphdb_edges_current` — data size

### Grafana Dashboard

Import `config/monitoring/grafana-dashboard.json` into Grafana for a pre-built overview dashboard with panels for query rate, cache usage, write operations, and more.

## Failover Behavior

### Raft Election (Automatic)

GoraphDB uses Hashicorp Raft with these timeouts:
- **HeartbeatTimeout:** 1 second
- **ElectionTimeout:** 1 second
- **LeaderLeaseTimeout:** 500ms

When the leader pod dies:
1. Followers detect missing heartbeats (~1s)
2. Election starts (~1s)
3. New leader elected (~2-3s total)
4. New leader starts gRPC replication server
5. Followers reconnect for WAL streaming

### Operator Response (Label Update)

The operator detects the new leader via `/api/health` and updates pod labels:
- Sets `goraphdb.io/role=leader` on the new leader pod
- Sets `goraphdb.io/role=follower` on all other pods
- Client Service automatically routes to the new leader

**Failover timeline:** Raft election ~2-3s + operator label update ≤30s

### Client Resilience

During failover, clients connected to the client Service may see brief errors. Recommended mitigations:
- Use the **read Service** for read-only queries (always available if ≥1 pod is healthy)
- Implement **retry logic** for write queries (2-3 retries with backoff)
- GoraphDB followers **automatically forward writes** to the leader, so connecting to any pod works once the new leader is elected

## Scaling

### Scale Up

```bash
kubectl patch goraphdbcluster goraphdb-ha -p '{"spec":{"replicas":5}}' --type=merge
```

The operator:
1. Updates the StatefulSet replica count
2. New pods start and join the Raft cluster via `-bootstrap` (idempotent)
3. New followers catch up via gRPC WAL streaming from the leader

### Scale Down

```bash
kubectl patch goraphdbcluster goraphdb-ha -p '{"spec":{"replicas":3}}' --type=merge
```

The StatefulSet removes the highest-ordinal pods first. Ensure the resulting cluster size maintains Raft quorum (odd number ≥ 1).

## Storage Layout

Each pod's persistent volume contains:

```
/data/db/
├── shard_0000.db          # bbolt B+tree shard (mmap-heavy reads)
├── shard_0001.db          # bbolt B+tree shard
├── ...
├── wal/
│   ├── wal-0000000000.log # WAL segment (64MB max, append-only)
│   ├── wal-0000000001.log
│   └── ...
└── raft/
    ├── raft-log.db        # Raft log store (bbolt)
    └── raft-stable.db     # Raft stable store (bbolt)
```

## Security

GoraphDB does not currently have built-in authentication or TLS. For production deployments:

1. **TLS termination:** Use a service mesh (Istio/Linkerd) for mTLS between pods, or terminate TLS at an Ingress controller
2. **Network policies:** Restrict Raft (7000) and gRPC (7001) traffic to within the cluster namespace
3. **RBAC:** The operator ServiceAccount has least-privilege permissions

## Directory Structure

```
deployment/k8s/operator/
├── README.md                          # This file
├── go.mod                             # Go module (controller-runtime)
├── main.go                            # Operator entrypoint
├── Dockerfile                         # Multi-stage build
├── Makefile                           # Build & deploy targets
├── api/
│   └── v1alpha1/
│       ├── groupversion_info.go       # API group registration
│       ├── types.go                   # CRD types with field mappings
│       └── defaults.go                # Default values (mirrors DefaultOptions)
├── config/
│   ├── crd/
│   │   └── goraphdb.io_goraphdbclusters.yaml  # CRD manifest
│   ├── rbac/
│   │   ├── role.yaml                  # ClusterRole
│   │   ├── role_binding.yaml          # ClusterRoleBinding
│   │   └── service_account.yaml       # ServiceAccount
│   ├── manager/
│   │   └── manager.yaml               # Operator Deployment
│   ├── samples/
│   │   ├── single-node.yaml           # Dev/test: standalone instance
│   │   ├── three-node-cluster.yaml    # HA: 3-node Raft cluster
│   │   └── production.yaml            # Production: 5-node with full config
│   └── monitoring/
│       ├── service-monitor.yaml       # Prometheus ServiceMonitor
│       └── grafana-dashboard.json     # Grafana dashboard
└── controllers/
    ├── goraphdbcluster_controller.go  # Main reconcile loop
    ├── statefulset.go                 # StatefulSet builder
    ├── services.go                    # Service topology (3 services)
    ├── bootstrap.go                   # Cluster bootstrap sequencing
    ├── health.go                      # Pod health checking
    ├── failover.go                    # Leader failover handling
    └── labels.go                      # Label/selector helpers
```

## Future Roadmap

- [ ] **Phase 4:** Scaling operations with `raft.AddVoter()`/`RemoveServer()`
- [ ] **Phase 4:** Rolling update strategy (followers first, leader last)
- [ ] **Phase 5:** Backup/Restore CRDs (`GoraphDBBackup`, `GoraphDBRestore`)
- [ ] **Phase 5:** WAL segment pruning based on follower LSN positions
- [ ] **Phase 6:** TLS via cert-manager integration
- [ ] **Phase 6:** NetworkPolicies for Raft/gRPC traffic isolation
- [ ] **Phase 7:** `kubectl goraphdb` CLI plugin
- [ ] **Phase 7:** E2E tests with kind
- [ ] **Phase 7:** Helm chart and OLM bundle
