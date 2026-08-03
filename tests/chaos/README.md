# Chaos scenarios

- etcd partition: block port 2379, verify register fails gracefully
- Redis slow: inject latency, circuit breaker still persists state
- GuardLoop timeout: score_based falls back to latency/cost ranking

Run manually in staging per ops/staging.md.
