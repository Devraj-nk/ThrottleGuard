Build plan — phased

Status update: Phase 1, Phase 2, and the Redis-backed Phase 3 foundation are complete in the current working tree.
- Built the Go scaffold and reverse-proxy skeleton.
- Added a real client IP extraction path from the socket connection.
- Replaced the fixed-window approach with a sliding-window in-memory limiter.
- Added a Redis-backed limiter using a Lua script over sorted sets for atomic check-and-increment.
- Verified behavior with unit tests for memory-based and Redis-based limit exhaustion, expiry, and IP extraction.

Phase 1: Single-instance fixed window limiter
Build the dumbest version first. In-memory map of IP → count, reset every N seconds. Get it rejecting requests with 429 Too Many Requests. This is just to get the proxy skeleton working.
Identify the client by real IP, not blindly by whatever `X-Forwarded-For` says — if you skip this now, every check you build in later phases (rate limit, reputation, anomaly) is spoofable by just setting that header. Only trust it when the request comes from your own load balancer's address.
Decide now what happens if the limiter's state is unavailable (fail-open and let traffic through, or fail-closed and reject) — becomes load-bearing in Phase 3 once state moves to Redis and a network hop can fail.

Phase 2: Fix the boundary bug → sliding window
Show yourself the fixed-window bug: hit 10 req/sec limit, send 10 requests at 0:59, then 10 more at 1:01 — you just let 20 through in ~2 seconds. Fix it with a sliding window counter (weighted average of current + previous window) or sliding log in Redis sorted sets.
Implemented: timestamp-based sliding window keyed per client, with expiration of stale events and deterministic tests for window edge behavior.

Phase 3: Move state to Redis
Now make it work across restarts and prepare for multi-instance. Use ZADD with timestamp scores + ZREMRANGEBYSCORE to expire old entries, wrapped in a Lua script or MULTI/EXEC for atomicity — this is where you'll hit and learn about race conditions.
Apply the fail-open/fail-closed decision from Phase 1 here: what does the gateway do on a Redis timeout? (Fail-open is the usual real-world default — a rate limiter outage shouldn't take down the whole API — but say so explicitly rather than let it be accidental.)

Next milestone: add Redis to Docker Compose, add a `redis` client package, and implement a Lua-based rate-limit script that checks and increments in one command.

Phase 4: Reputation scoring + anomaly detection layer
These are two related but distinct things — build reputation as a persistent score per IP (e.g. a Redis hash with a decaying value), not just a byproduct of anomaly flags:
- Anomaly signals feeding the score: distinct endpoints hit in a rolling window, request timing variance (bots are too regular), failed-auth ratio.
- Reputation persists and decays over time (a bad score from an hour ago should matter less than one from now), so repeat offenders escalate and one bad burst doesn't permanently ban a real user.
- Response tiers driven by score, not just a binary flag: log-only → soft delay → hard block. Skip actual CAPTCHA integration for v1 — it's a third-party dependency that doesn't teach you anything about the rate limiter itself; a deliberate response delay is enough to demonstrate tiering.

Phase 5: The hard/interesting part — distributed rate limiting
Spin up 3 gateway instances behind a load balancer (nginx or just round-robin in a script). Prove that per-instance in-memory limiting fails (each instance thinks it's under the limit, but combined traffic isn't). Fix it by centralizing counts in Redis, then optimize — because now every request is a network call to Redis, so you'll want to discuss trade-offs: local caching with periodic sync vs strict centralized counting vs approximate algorithms (this is genuinely what Cloudflare/Stripe engineering blogs write about).

Phase 6: Prove it with numbers
Use k6 or wrk to simulate: (a) normal traffic, (b) a brute-force credential-stuffing pattern, (c) a distributed attack from multiple "IPs" hitting multiple gateway instances. Measure before/after block rates — this gives you your actual resume metric instead of a made-up percentage.

Phase 7: Observability
Add Prometheus metrics (requests allowed/blocked/flagged, proxy latency, current reputation distribution) and a Grafana dashboard. This isn't just polish — it's what lets you *watch* Phase 6's attack simulation happen live instead of only reading a summary number afterward, and it's the difference between "I built a rate limiter" and "I built something you could actually run in production."