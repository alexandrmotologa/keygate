# KeyGate: Master Implementation Plan

High-throughput API key issuance, sub-millisecond in-memory verification, and distributed rate-limiting gateway.

KeyGate is an open-source, self-hosted alternative to Unkey.dev built with Go 1.23, pure Go SQLite (WAL mode), in-memory caching via Ristretto, and an embedded React 19 management studio. It operates as an inline reverse proxy or as a standalone key verification service.

---

## 1. Architectural Blueprint

```
+-----------------------------------------------------------------------------------+
|                                      KEYGATE                                      |
+-----------------------------------------+-----------------------------------------+
                                          |
        +---------------------------------+---------------------------------+
        |                                                                   |
+-------v-------------------------+                 +-----------------------v-------+
|   Reverse Proxy & Verification  |                 |    Prefix-Encoded Key Vault   |
|  * Authorization: Bearer <key>  |                 |  * sk_live_<prefix>_<random>  |
|  * Sub-Millisecond Cache Check  |                 |  * Salted SHA-256 Storage     |
|  * Standalone /v1/keys.verifyKey|                 |  * Scope & IP Allowlist RBAC  |
+-------+-------------------------+                 +-----------------------+-------+
        |                                                                   |
        +---------------------------------+---------------------------------+
                                          |
                        +-----------------v-----------------+
                        |   Sliding-Window Rate Limiter     |
                        |  * Weighted Window Calculation    |
                        |  * Token Bucket Burst Tolerance   |
                        |  * Standard IETF RateLimit Headers|
                        +-----------------+-----------------+
                                          |
                        +-----------------v-----------------+
                        |   Durable Storage & Persistence   |
                        |  * Pure Go SQLite WAL Mode        |
                        |  * Automatic Schema Migrations    |
                        |  * Usage Logging & Aggregations   |
                        +-----------------+-----------------+
                                          |
                        +-----------------v-----------------+
                        |   Embedded React 19 Web Studio    |
                        |  * Live Request Stream (SSE)      |
                        |  * 1-Click Key Issuance & Revoke  |
                        |  * Interactive In-Browser Tester  |
                        +-----------------------------------+
```

---

## 2. Directory Layout

```
keygate/
├── cmd/
│   └── keygate/
│       └── main.go               # Unified CLI: serve, key commands, benchmark
├── internal/
│   ├── config/
│   │   └── config.go             # Environment, flags, and runtime defaults
│   ├── keys/
│   │   ├── generator.go          # Base58/Base62 secure random key generator
│   │   ├── hasher.go             # Salted SHA-256 cryptographic hasher
│   │   ├── validator.go          # High-speed verification with Ristretto cache
│   │   └── validator_test.go     # Verification unit and benchmark tests
│   ├── ratelimit/
│   │   ├── sliding_window.go     # Sliding window rate limiter
│   │   ├── token_bucket.go       # Token bucket with burst capacity
│   │   ├── headers.go            # IETF RateLimit-* header builder
│   │   └── sliding_window_test.go# Rate limiter unit tests
│   ├── storage/
│   │   ├── models.go             # Data models: Key, Workspace, UsageEvent, Stats
│   │   ├── sqlite.go             # SQLite connection pool, WAL mode, migrations
│   │   ├── repository.go         # Key, Workspace, Usage persistence interface
│   │   └── sqlite_test.go        # Storage integration tests
│   ├── proxy/
│   │   ├── reverse_proxy.go      # Reverse proxy with key interception & stripping
│   │   ├── upstream.go           # Internal mock upstream server for local testing
│   │   └── proxy_test.go         # Proxy routing and header injection tests
│   └── server/
│       ├── api.go                # REST API endpoints (/v1/keys.*, /v1/analytics.*)
│       ├── sse.go                # Server-Sent Events broker for real-time telemetry
│       └── static.go             # Embedded React 19 SPA file server
├── web/
│   ├── src/
│   │   ├── components/
│   │   │   ├── DashboardOverview.tsx # Metrics cards, live charts, quick actions
│   │   │   ├── KeyManagementTable.tsx# Paginated key table, search, revoke actions
│   │   │   ├── KeyCreationModal.tsx  # Key generator modal (prefix, tier, scopes)
│   │   │   ├── SecretKeyModal.tsx    # One-time plaintext reveal with curl snippet
│   │   │   ├── RateLimitPolicy.tsx   # Tier editor (Free, Pro, Enterprise)
│   │   │   ├── LiveRequestFeed.tsx   # Real-time SSE event log
│   │   │   └── ApiTester.tsx         # In-browser key verification playground
│   │   ├── hooks/
│   │   │   └── useEventStream.ts     # SSE connection hook with auto-reconnect
│   │   ├── services/
│   │   │   └── api.ts                # Typed client for KeyGate REST API
│   │   ├── types/
│   │   │   └── index.ts              # TypeScript domain types
│   │   ├── App.tsx                   # Main layout and tab navigation
│   │   └── main.tsx                  # React 19 mount point
│   ├── package.json
│   ├── tailwind.config.js
│   ├── postcss.config.js
│   ├── tsconfig.json
│   └── vite.config.ts
├── docs/
│   ├── API_REFERENCE.md          # REST API specifications and curl examples
│   └── ARCHITECTURE.md           # Security model, hashing, and cache topology
├── go.mod
├── go.sum
├── Makefile
├── LICENSE
└── README.md
```

---

## 3. Work Breakdown & Detailed Subtasks

### Task 1: Repository Foundation & Documentation
- [x] 1.1 Initialize Git repository on `main` branch.
- [x] 1.2 Sync Phase 1 portfolio registry in `mtlg-site` and `mtlglabs-space`.
- [x] 1.3 Create public GitHub repository `alexandrmotologa/keygate` with 14 topics and humanized description.
- [ ] 1.4 Write comprehensive `README.md`, `LICENSE` (Apache 2.0 or MIT), and `docs/API_REFERENCE.md`.

### Task 2: Core Storage & Database Layer (`internal/storage`)
- [ ] 2.1 Set up Go module `github.com/alexandrmotologa/keygate`.
- [ ] 2.2 Define domain structs in `internal/storage/models.go` (Workspace, Key, UsageEvent, AnalyticsSummary, RateLimitTier).
- [ ] 2.3 Implement pure Go SQLite connection in `internal/storage/sqlite.go` with WAL mode, busy timeout, and connection pooling.
- [ ] 2.4 Implement schema migrations: `workspaces`, `api_keys`, `usage_events`, and `hourly_stats` tables with proper indexes.
- [ ] 2.5 Implement repository operations: CreateKey, GetKeyByHash, ListKeys, RevokeKey, UpdateKey, RecordUsageEvent, GetAnalyticsSummary.
- [ ] 2.6 Write storage unit tests in `internal/storage/sqlite_test.go`.

### Task 3: Cryptographic Key Generator & Salted SHA-256 Hasher (`internal/keys`)
- [ ] 3.1 Implement secure random generator in `internal/keys/generator.go` using `crypto/rand` and Base58 character set. Format: `sk_live_{workspace}_{24_random_bytes}`.
- [ ] 3.2 Implement salted SHA-256 hasher in `internal/keys/hasher.go` with `subtle.ConstantTimeCompare`.
- [ ] 3.3 Write unit tests in `internal/keys/generator_test.go` checking entropy, prefix formats, and collision avoidance.

### Task 4: In-Memory Verification Engine & Cache (`internal/keys`)
- [ ] 4.1 Initialize `github.com/dgraph-io/ristretto` cache with configurable capacity and cost management in `internal/keys/validator.go`.
- [ ] 4.2 Implement sub-millisecond cache check: return verified key metadata immediately on hit.
- [ ] 4.3 Implement cache miss fallback: fetch from SQLite, populate Ristretto with TTL, return result.
- [ ] 4.4 Implement validation rules: active state, revocation timestamp, expiration date, required scopes, and CIDR/IP allowlist.
- [ ] 4.5 Implement cache invalidation on key revocation and updates.
- [ ] 4.6 Write benchmark tests demonstrating sub-millisecond response latency for cached keys.

### Task 5: Sliding-Window Rate Limiter & IETF Headers (`internal/ratelimit`)
- [ ] 5.1 Implement sliding-window counter algorithm in `internal/ratelimit/sliding_window.go`:
  $$\text{Count} = \text{PrevWindowCount} \times \left(1 - \frac{t - t_0}{\text{WindowSize}}\right) + \text{CurrWindowCount}$$
- [ ] 5.2 Implement token-bucket burst handling in `internal/ratelimit/token_bucket.go`.
- [ ] 5.3 Implement IETF draft header injection (`RateLimit-Limit`, `RateLimit-Remaining`, `RateLimit-Reset`, `Retry-After`) in `internal/ratelimit/headers.go`.
- [ ] 5.4 Support predefined tiers (`Free`: 10/min, `Pro`: 500/min, `Enterprise`: 10,000/min) and custom per-key limits.
- [ ] 5.5 Write unit tests verifying quota boundaries, window transitions, and header calculations.

### Task 6: Transparent Reverse Proxy & Upstream Mock (`internal/proxy`)
- [ ] 6.1 Implement HTTP reverse proxy in `internal/proxy/reverse_proxy.go`.
- [ ] 6.2 Intercept `Authorization: Bearer <key>` and `X-API-Key: <key>`.
- [ ] 6.3 Authenticate against `validator.go` and rate-limit against `sliding_window.go`.
- [ ] 6.4 Strip client API key before forwarding to target upstream.
- [ ] 6.5 Inject internal headers: `X-KeyGate-Key-Id`, `X-KeyGate-Owner-Id`, `X-KeyGate-Workspace`, `X-KeyGate-Scopes`, `X-KeyGate-Tier`.
- [ ] 6.6 Build mock upstream server in `internal/proxy/upstream.go` enabled via `--mock-upstream` flag for testing without external servers.
- [ ] 6.7 Write integration tests for proxy pipeline, 401 unauthorized, and 429 rate limit responses.

### Task 7: REST API & Server-Sent Events Telemetry (`internal/server`)
- [ ] 7.1 Implement REST API handlers in `internal/server/api.go`:
  - `POST /v1/keys.verifyKey` (standalone verification endpoint)
  - `POST /v1/keys.createKey` (key creation with one-time plaintext return)
  - `GET /v1/keys.listKeys` (paginated list with filter by status/workspace)
  - `POST /v1/keys.revokeKey` (instant revocation)
  - `POST /v1/keys.updateKey` (update limits, scopes, IP allowlist)
  - `GET /v1/analytics.summary` (throughput, p50/p95/p99 latency, status breakdown)
  - `GET /healthz` (liveness check)
- [ ] 7.2 Implement real-time SSE broker in `internal/server/sse.go` streaming live verification events.
- [ ] 7.3 Implement embedded SPA file server in `internal/server/static.go` using `embed.FS`.

### Task 8: React 19 Frontend Web Studio (`web/`)
- [ ] 8.1 Initialize Vite + React 19 + Tailwind CSS + Lucide project in `web/`.
- [ ] 8.2 Build modern dark tech UI with dashboard navigation (Overview, Keys, Rate Limits, Analytics, API Tester).
- [ ] 8.3 Implement `DashboardOverview.tsx`: real-time stats cards (Total Keys, Active Keys, Total Requests, P99 Latency), throughput charts.
- [ ] 8.4 Implement `KeyManagementTable.tsx`: search, filter by tier/status, copy prefix, view details, revoke button.
- [ ] 8.5 Implement `KeyCreationModal.tsx`: configure name, workspace, prefix, rate limit tier, custom quota, scopes, IP allowlist, expiry.
- [ ] 8.6 Implement `SecretKeyModal.tsx`: display raw generated key once with copy button and curl code example.
- [ ] 8.7 Implement `ApiTester.tsx`: interactive playground to test key verification and proxy forwarding with live header inspection.
- [ ] 8.8 Implement `LiveRequestFeed.tsx`: live SSE stream displaying incoming requests, latency, and status codes.
- [ ] 8.9 Build and verify production bundle in `web/dist`.

### Task 9: Unified CLI & Benchmarking Engine (`cmd/keygate`)
- [ ] 9.1 Build CLI in `cmd/keygate/main.go` supporting:
  - `keygate serve --port 8000 --target http://localhost:3000 --db ./keygate.db [--mock-upstream]`
  - `keygate key create --name "Frontend" --tier pro`
  - `keygate key verify --key sk_live_...`
  - `keygate key revoke --id ...`
  - `keygate benchmark --keys 1000 --concurrency 50 --requests 50000`
- [ ] 9.2 Implement `benchmark` command running thousands of concurrent key verifications to measure throughput (ops/sec) and p50/p95/p99 latency.

### Task 10: Verification, Testing & Final Polish
- [ ] 10.1 Run all Go unit, integration, and benchmark tests (`go test -v -race ./...`).
- [ ] 10.2 Verify single-binary build compiles cleanly (`go build -o bin/keygate cmd/keygate/main.go`).
- [ ] 10.3 Perform end-to-end smoke test of proxy, key verification, rate limiting, and web studio.
- [ ] 10.4 Commit, tag, and push to GitHub repository `alexandrmotologa/keygate`.
