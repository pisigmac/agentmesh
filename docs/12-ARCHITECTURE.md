# Architecture

AgentMesh separates **control plane** (registry, routing decisions, REST/gRPC API) from **data plane** (request proxy, tracing, short-lived route cache). Both are implemented in **Go 1.22**.

## System context

```mermaid
flowchart TB
  subgraph clients [Clients]
    ORCH[Orchestrator / IDE]
    UI[Next.js UI]
  end

  subgraph agentmesh [AgentMesh]
    CP[Control Plane<br/>gRPC 9090 + HTTP 8080]
    DP[Data Plane<br/>HTTP 8081]
  end

  subgraph stores [Infrastructure]
    ETCD[(etcd registry)]
    REDIS[(Redis<br/>circuit + rate limit)]
    CH[(ClickHouse<br/>routing rollups)]
  end

  subgraph external [External]
    GL[GuardLoop optional]
    TL[TraceLens OTLP<br/>POST /v1/traces]
    AGENTS[Agent endpoints<br/>9 adapter types]
  end

  ORCH --> CP
  UI --> CP
  ORCH --> DP
  DP --> CP
  CP --> ETCD
  CP --> REDIS
  CP --> CH
  CP --> GL
  DP --> AGENTS
  DP --> TL
  CP --> TL
```

## Request flow (route)

```mermaid
sequenceDiagram
  participant C as Client
  participant CP as Control Plane
  participant E as etcd
  participant R as Redis
  participant CH as ClickHouse
  participant GL as GuardLoop
  participant DP as Data Plane
  participant A as Agent

  C->>CP: POST /v1/route
  CP->>E: List agents for org
  CP->>CH: Read latency/success EWMA
  opt score-based
    CP->>GL: Fetch scores
  end
  CP->>R: Check circuit + rate limit
  CP-->>C: RouteResponse + trace_context
  C->>DP: Proxy to endpoint_url
  DP->>A: Forward request
  DP->>TL: OTLP span (JWT)
  DP->>CH: Insert route event
  alt failure
    DP->>R: RecordFailure
  else success
    DP->>R: RecordSuccess
  end
```

## Control plane

- **gRPC** implements `RegisterAgent`, `DiscoverAgents`, `RouteRequest`, `ReportHealth`, `GetCircuitState`.
- **REST gateway** (`internal/rest`) translates JSON to gRPC for `/v1/*`.
- **Router engine** applies strategy plugins; `score-based` delegates to GuardLoop when enabled.

## Data plane

- Mode `gateway` (default): proxies to selected `endpoint_url` with timeout and retries from config.
- **TraceLens**: OpenTelemetry OTLP HTTP exporter to `TRACELENS_OTLP_ENDPOINT` with optional `TRACELENS_JWT`. This is **not** Jaeger.
- Short **route cache** (`DATAPLANE_ROUTE_CACHE_TTL_MS`) reduces duplicate control plane calls.

## Multi-tenancy

Authentication binds requests to `org_id` via API key validation and headers `X-Org-ID` / `X-Workspace-ID`. etcd keys use configurable prefix (`ETCD_PREFIX`, default `/agentmesh`).

## Deployment topology

Kubernetes: Helm chart `deploy/helm/agentmesh` deploys control plane with **HorizontalPodAutoscaler** (CPU 70%, 2–20 replicas) and **PodDisruptionBudget** (`minAvailable: 1`). Data plane may run as sibling Deployment (compose locally).

## Failure domains

| Component | Blast radius | Mitigation |
|-----------|--------------|------------|
| etcd | No registration/discovery | HA etcd cluster, PDB on control plane |
| Redis | Circuits/limits inaccurate | Redis Sentinel/Cluster |
| ClickHouse | Analytics lag | Async inserts, batch workers |
| GuardLoop | Score-based degrades | Timeout + fallback ranking |

## Related docs

- [13-TECH-STACK.md](13-TECH-STACK.md)
- [16-DB-SCHEMA.md](16-DB-SCHEMA.md)
- [15-DEPLOY.md](15-DEPLOY.md)
