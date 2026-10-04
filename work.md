# Work log

## Status snapshot

Project: ThrottleGuard
Current milestone: Phase 2 complete; Phase 3 in progress.

## Completed work

### 1) Project scaffold
- Initialized a Go module and application structure under `cmd/` and `internal/`.
- Added configuration loading for environment variables.
- Added a basic reverse proxy and health endpoint.
- Added Docker and GitHub Actions scaffolding for local run and CI.

### 2) Reverse proxy behavior
- Implemented a Go HTTP server that accepts incoming requests and forwards them to a configured backend.
- Added graceful shutdown handling.
- Used real connection source information for client identification rather than trusting a client-supplied forwarded IP header.

### 3) Rate limiting
- Implemented a sliding-window in-memory limiter using timestamped request entries per client.
- Added tests covering:
  - request rejection once the current window is full,
  - expiry of old requests from the sliding window,
  - correct client IP extraction from the socket address.

### 4) Validation
Verified with fresh commands:

```bash
go test ./...
go vet ./...
```

Both completed successfully in the current project state.

## Current implementation notes
- The limiter is now backed by a Redis sorted-set script when a Redis address is configured.
- The in-memory sliding-window limiter remains the fallback for local single-node development without Redis.
- This satisfies the local single-node rate limiting milestone while also creating the atomic shared-state path needed for the distributed phase.

## Next implementation plan

### Phase 3: Redis-backed atomic limiter
- Add Redis dependency and Docker Compose service for Redis. ✅
- Replace the in-memory sliding window with Redis sorted sets / timestamps. ✅
- Use Lua scripts or `MULTI/EXEC` so the check and increment happen atomically. ✅
- Define explicit fail-open vs fail-closed behavior when Redis is unavailable. In progress: the current implementation defaults to fail-open if Redis is unreachable at request time.

### Phase 4: Multi-instance verification
- Run 3 gateway instances behind a local load balancer.
- Prove that a central Redis limiter prevents the per-instance count bug.
- Capture distributed traffic patterns and rate-limit outcomes.

### Phase 5: Reputation + anomaly layer
- Add per-IP reputation score with decay.
- Track endpoint diversity, timing anomalies, and failed-auth patterns.
- Introduce response tiers such as log, delay, or hard block.

### Phase 6: Observability and benchmark proof
- Add Prometheus metrics and Grafana dashboard.
- Run k6 or wrk simulations for normal, brute-force, and distributed attack traffic.
- Record before/after results and attach the numbers to the project narrative.

## Suggested next milestone
Complete the Redis-backed atomic limiter and prove it with a local integration test before moving into the reputation and anomaly layers.
