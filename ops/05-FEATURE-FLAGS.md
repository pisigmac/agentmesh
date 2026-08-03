# Feature flags

Runtime toggles use environment variables read by `internal/featureflags/flags.go`.

## Convention

```
FF_<NAME>=true|1|false
```

Unset variables fall back to code defaults passed to `featureflags.Enabled(name, defaultVal)`.

## Known flags

| Flag | Default | Description |
|------|---------|-------------|
| `FF_EVENTS_WEBHOOKS` | false | Outbound webhook dispatch |
| `FF_SCORE_CACHE_SHARD` | false | Redis-sharded GuardLoop score cache |
| `FF_ROUTE_CACHE_V2` | false | Extended data plane cache keys |
| `FF_CLERK_AUTH` | false | Clerk JWT validation path |
| `FF_STRICT_ORG_HEADER` | false | Reject requests without X-Org-ID |

Add new flags in `flags.go` call sites with explicit defaults—avoid silent behavior changes.

## Helm

Map flags under `controlPlane.env` / `dataPlane.env`:

```yaml
controlPlane:
  extraEnv:
    - name: FF_EVENTS_WEBHOOKS
      value: "true"
```

## Rollout process

1. Ship dark (default false) in release N
2. Enable in staging for 48h
3. Enable per-org allowlist (future) then global

## Observability

Log flag state at startup JSON blob `feature_flags`.

## GuardLoop

Product feature `GUARDLOOP_ENABLED` is separate from `FF_*`—GuardLoop is a configured integration, not a code-path flag.

## Kill switch

For incidents, set `FF_*` false and redeploy control plane; PDB ensures minAvailable during roll.

## Testing

Unit tests set `t.Setenv("FF_NAME", "true")` where supported.
