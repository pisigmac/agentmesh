# Chaos scenarios

## Automated

```bash
go test ./tests/chaos/ ./internal/dataplane/ -count=1
```

- **E8 failover:** `failover_test.go` — primary 5xx → secondary succeeds within `max_retries` when failover is enabled.

## Manual / staging

- etcd partition: block port 2379, verify register fails gracefully
- Redis slow: inject latency, circuit breaker still persists state
- GuardLoop timeout: score_based falls back to latency/cost ranking

Run manually in staging per ops/staging.md.
