# AgentMesh

> **Canonical README:** [docs/01-README.md](docs/01-README.md)

AgentMesh routes requests across registered coding agents using a Go 1.22 control plane (etcd registry, Redis circuit breaking) and a data plane with ClickHouse analytics and TraceLens OTLP tracing.

## Quick links

- [Features](docs/02-FEATURES.md) · [API](docs/05-API.md) · [OpenAPI](docs/openapi.yml)
- [Routing questions to agents](docs/22-ROUTING-QUESTIONS.md)
- [Future plan](FUTURE_PLAN.md) · [Short roadmap](docs/10-FUTURE.md)
- [Architecture](docs/12-ARCHITECTURE.md) · [Deploy](docs/15-DEPLOY.md) · [Environment](docs/14-ENV.md)
- [Operations runbooks](ops/01-PAYMENTS.md)

## Quick start

```bash
make docker-up
curl -s http://localhost:48080/health
```

For installation, authentication (`Bearer` + `X-Org-ID` / `X-Workspace-ID`), and full examples, open **[docs/01-README.md](docs/01-README.md)**.
