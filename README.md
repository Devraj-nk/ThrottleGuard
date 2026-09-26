# ThrottleGuard

Rate Limiter + Abuse Detection Gateway

Build a reverse proxy (Go/Node) that sits in front of an API and does sliding-window rate limiting, IP reputation scoring, and anomaly detection (e.g., flag if one IP hits 10 different endpoints in 2 seconds — classic scraping/credential-stuffing pattern).

Real problem solved: This is literally what companies pay Cloudflare/Kong for.
Resume line: "Built a Redis-backed rate limiter with sliding window + token bucket algorithms, reducing simulated brute-force success rate by X%."
Depth to add: Distributed rate limiting across multiple gateway instances (this is the hard, interesting part — naive per-instance limiting breaks under horizontal scaling).

## Current implementation

The initial Go service provides a reverse proxy, a health endpoint, trusted connection-based client identity, and a thread-safe fixed-window in-memory limiter. Redis-backed distributed limiting is the next planned implementation phase.

### Run locally

Start a backend at `http://localhost:9000`, then run the gateway:

```powershell
go run ./cmd/throttleguard
```

The gateway listens on `http://localhost:8080`. Configuration is available through `THROTTLEGUARD_ADDRESS`, `THROTTLEGUARD_BACKEND_URL`, `THROTTLEGUARD_RATE_LIMIT`, and `THROTTLEGUARD_RATE_WINDOW`.

Run the checks with:

```powershell
go test ./...
go vet ./...
```

Or start the gateway and an example backend together:

```powershell
docker compose up --build
```

## Tech stack

| Layer | Choice | Why |
|---|---|---|
| Gateway language | **Go** | Rate limiting is a concurrency problem; goroutines/channels/mutexes map directly onto it. `net/http/httputil.ReverseProxy` gives a working proxy skeleton for free. |
| Shared state | **Redis** (single instance -> Redis Cluster later if needed) | Atomic counters, sorted sets for sliding-window logs, Lua/`EVAL` for check-and-increment atomicity, Pub/Sub to broadcast bans/reputation changes to all gateway instances. |
| Rate limit algorithm | **Sliding window counter** (default) + **token bucket** (opt-in per route) | Sliding window counter is the accuracy/memory sweet spot; token bucket for routes that should tolerate bursts. Both implemented as Redis Lua scripts for atomicity. |
| Anomaly/reputation store | Redis hashes + sets (`SADD endpoints:{ip}:{window}`, `HINCRBY reputation:{ip}`) | Reuses the same store instead of adding a second system; scores can carry a TTL-based decay. |
| Config | YAML file, hot-reloaded (`fsnotify`) | Per-route limits/thresholds without redeploying. |
| Metrics | **Prometheus** (`client_golang`) + **Grafana** | Counters for allowed/blocked/flagged requests and histograms for proxy latency. |
| Logging | `log/slog` (structured, stdlib) | No extra logging dependency is needed at this scale. |
| Load generation | **k6** (or `vegeta`) | Scriptable normal, brute-force, and distributed traffic patterns. |
| Local topology | **Docker Compose**: N gateway replicas + nginx (LB) + Redis + Prometheus + Grafana | Makes distributed rate limiting provable locally. |
| CI | GitHub Actions: `go vet`, `go test`, `golangci-lint`, `docker build` | Keeps the implementation reliable as detection logic grows. |

Explicitly **not** doing for v1: a database beyond Redis, a web admin UI, auth/API-key tiers, or external IP-reputation feeds.

## High-level design

```text
					+------------------+
					|     Clients      |
					|  Client / bot    |
					+--------+---------+
						    |
						    v
					+------------------+
					| Load balancer    |
					|      nginx       |
					+--------+---------+
						    |
			  +----------------+----------------+
			  |                |                |
			  v                v                v
		+-------------+  +-------------+  +-------------+
		|  Gateway 1  |  |  Gateway 2  |  |  Gateway N  |
		|  stateless  |  |  stateless  |  |  stateless  |
		+------+------+  +------+------+  +------+------+
			  |                |                |
			  +----------------+----------------+
						    |
						    v
			  +----------------------------------+
			  | Redis                            |
			  | counters | sorted sets |         |
			  | reputation | pub/sub             |
			  +----------------+-----------------+
						    |
						    v
					   +-----------+
					   | Backend   |
					   | API       |
					   +-----------+

  rules.yaml (hot-reloaded) ---> all gateway instances
  gateways --------------------> Prometheus ------> Grafana
```

Gateway instances are **stateless**: all shared state (counts, reputation, and bans) lives in Redis, so any instance can serve any request without losing rate-limit accuracy.

## Gateway internals (per-instance)

```text
 +------------------+
 | Incoming request |
 +--------+---------+
		|
		v
 +-----------------------------+
 | Identity extraction         |
 | real client IP              |
 +-------------+---------------+
			|
			v
 +-----------------------------+
 | Reputation / blocklist      |
 +----------+-------------+----+
		  |             |
	  banned|             |ok
		  v             v
	  +---------+  +--------------------------+
	  |  403    |  | Rate limiter             |
	  +----+----+  | Lua: check + incr atomic |
		  |       +-------------+------------+
		  |                     |
		  |          over limit | under limit
		  |                     v
		  |              +--------------------------+
		  |              | Anomaly detector          |
		  |              | diversity | timing |      |
		  |              | error ratio               |
		  |              +-----------+--------------+
		  |                          |
		  |                flagged   |   clean
		  |                          v
		  |              +-----------+---------------+
		  |              | Tiered response | Backend |
		  |              | log / delay /   | proxy   |
		  |              | block           |         |
		  |              +----------------+----+-----+
		  |                                      |
		  |                                      v
		  |                               +-------------+
		  +------------------------------>| Response    |
								          +------+------+ 
										     |
										     v
							   +------------------------+
							   | Prometheus counters    |
							   +------------------------+
```

**Real client IP extraction matters**: only trust forwarding headers from the known load-balancer hop. Otherwise, a client can spoof `X-Forwarded-For` and bypass IP-based rate limiting, reputation, and anomaly checks.

## Data flow: single request

```text
 Client          Gateway             Redis             Backend
   |               |                  |                  |
   |-- HTTP ------>|                  |                  |
   |               |-- resolve IP --->|                  |
   |               |-- GET reputation:{ip} ------------->|
   |               |                  |                  |
   |               |  [banned / below threshold]         |
   |<-- 403 -------|                  |                  |
   |               |                  |                  |
   |               |  [not banned]    |                  |
   |               |-- EVALSHA rate limit -------------->|
   |               |<------------- allowed / rejected ---|
   |               |                  |                  |
   |               |  [over limit]    |                  |
   |               |-- HINCRBY violations -------------->|
   |<-- 429 + Retry-After -------------------------------|
   |               |                  |                  |
   |               |  [under limit]   |                  |
   |               |-- SADD endpoint ------------------->|
   |               |-- SCARD endpoint ------------------>|
   |               |                  |                  |
   |               |  [anomaly threshold exceeded]       |
   |               |-- HINCRBY anomaly_score ----------->|
   |<-- 429 or delayed response -------------------------|
   |               |                  |                  |
   |               |  [normal pattern]                   |
   |               |------------------------------->     |
   |               |<-------------------------------     |
   |<-- response --|                  |                  |
   |               |                  |                  |
   |               |-- emit Prometheus metrics (async)   |
```

## Distributed rate limiting

Naive per-instance counters break under horizontal scaling:

```text
 WRONG: local in-memory counters

 +----------------------+  +----------------------+  +----------------------+
 | Instance A           |  | Instance B           |  | Instance C           |
 | local count: 8 / 10  |  | local count: 8 / 10  |  | local count: 8 / 10  |
 +----------------------+  +----------------------+  +----------------------+

 Each instance thinks it is under the limit.
 Combined result: 24 requests pass through a 10 requests/window limit.


 RIGHT: centralized count in Redis

 +-------------+       +---------------------------+       +-------------+
 | Instance A  |------>|                           |<------| Instance B  |
 +-------------+       | Redis: count = 10 / 10    |       +-------------+
					   |                           |
 +-------------+       |                           |       +-------------+
 | Instance C  |------>|                           |       | Shared state |
 +-------------+       +---------------------------+       +-------------+

 Every instance reads and writes the same counter.
 Trade-off: one Redis round-trip per request.
```

Once centralized counting is proven correct under load, explore local caching with periodic sync or approximate counting to reduce Redis round-trips.