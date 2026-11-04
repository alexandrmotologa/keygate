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
|  * Standalone /v1/keys.verifyKey|                 |  * Scope, IP & Quota RBAC     |
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
                        |  * Workspaces & Multi-Tenancy     |
                        |  * Audit Log & Usage Aggregations |
                        +-----------------+-----------------+
                                          |
                        +-----------------v-----------------+
                        |   Embedded React 19 Web Studio    |
                        |  * Live Request Stream (SSE)      |
                        |  * Key Inspector & Live Editor    |
                        |  * Multi-Workspace Switcher       |
                        |  * Code Generator (5 Languages)   |
                        |  * Prometheus /metrics Exporter   |
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
│   ├── keys/
│   │   ├── generator.go          # Base58 secure random key generator
│   │   ├── generator_test.go     # Key generation & parsing unit tests
│   │   ├── hasher.go             # Salted SHA-256 cryptographic hasher
│   │   ├── validator.go          # High-speed verification with Ristretto cache
│   │   └── validator_test.go     # Verification unit and benchmark tests
│   ├── ratelimit/
│   │   ├── sliding_window.go     # Sliding window token-bucket rate limiter
│   │   ├── tiers.go              # Pre-configured tiers (Free, Pro, Enterprise)
│   │   └── sliding_window_test.go# Rate limiter unit tests
│   ├── storage/
│   │   ├── models.go             # Data models: Key, Workspace, UsageEvent, AuditLog, Stats
│   │   ├── sqlite.go             # SQLite connection pool, WAL mode, migrations
│   │   └── sqlite_test.go        # Storage integration tests
│   ├── proxy/
│   │   ├── reverse_proxy.go      # Reverse proxy with key interception & stripping
│   │   ├── upstream.go           # Internal mock upstream server for local testing
│   │   └── proxy_test.go         # Proxy routing and header injection tests
│   ├── metrics/
│   │   ├── prometheus.go         # Native Prometheus /metrics exposition handler
│   │   └── prometheus_test.go    # Metrics formatting tests
│   └── server/
│       ├── api.go                # REST API endpoints (/v1/keys.*, /v1/workspaces.*, /v1/audit.*)
│       ├── api_test.go           # REST API unit and integration tests
│       ├── sse.go                # Server-Sent Events broker for real-time telemetry
│       └── static.go             # Embedded React 19 SPA file server
├── web/
│   ├── src/
│   │   ├── components/
│   │   │   ├── DashboardOverview.tsx # Metrics cards, 24h volume, quick actions
│   │   │   ├── KeyManagementTable.tsx# Paginated key table, search, revoke, drawer trigger
│   │   │   ├── KeyDetailDrawer.tsx   # Detailed key inspector, live edit, recent requests
│   │   │   ├── KeyCreationModal.tsx  # Key generator modal (prefix, tier, max uses, scopes)
│   │   │   ├── SecretKeyModal.tsx    # One-time plaintext reveal with multi-language code
│   │   │   ├── KeyRotationModal.tsx  # Zero-downtime key rotation with grace period
│   │   │   ├── WorkspaceModal.tsx    # Create new workspace modal
│   │   │   ├── AuditLogTable.tsx     # Security audit trail of key and policy changes
│   │   │   ├── RateLimitPolicy.tsx   # Tier editor (Free, Pro, Enterprise)
│   │   │   ├── LiveRequestFeed.tsx   # Real-time SSE event log
│   │   │   ├── ApiTester.tsx         # In-browser key verification playground
│   │   │   └── CodeGenerator.tsx     # Multi-language snippets (cURL, TS, Py, Go, Rust)
│   │   ├── hooks/
│   │   │   └── useEventStream.ts     # SSE connection hook with auto-reconnect
│   │   ├── services/
│   │   │   └── api.ts                # Typed client for KeyGate REST API
│   │   ├── types/
│   │   │   └── index.ts              # TypeScript domain types
│   │   ├── App.tsx                   # Main layout and tab navigation
│   │   └── main.tsx                  # React 19 mount point
│   ├── embed.go                  # Go embed filesystem bridge
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

### Phase 1: Core Engine & Gateway (Completed)
- [x] 1.1 Repository initialization and portfolio synchronization.
- [x] 1.2 Cryptographic key issuance (Base58) and salted SHA-256 storage in SQLite WAL mode.
- [x] 1.3 In-memory Ristretto verification engine (< 0.5ms benchmarked).
- [x] 1.4 Sliding-window token-bucket rate limiter with standard IETF headers.
- [x] 1.5 Reverse proxy with credential stripping and identity header injection.
- [x] 1.6 REST API and real-time Server-Sent Events broker.
- [x] 1.7 Single binary Go embed React 19 web studio.
- [x] 1.8 Unified CLI and load benchmark engine.

### Phase 2: Advanced Capabilities & Production Hardening
- [x] 2.1 Ephemeral / Quota-Capped API Keys (`MaxUses`):
  - Add `max_uses` to SQLite schema and domain model.
  - Reject expired usage quotas with `QUOTA_EXCEEDED` code.
  - Add quota bar visualization in React 19 table and drawer.
- [x] 2.2 Zero-Downtime Key Rotation:
  - Implement `POST /v1/keys.rotateKey` with configurable grace period (e.g., 24h).
  - Issue new key linked to same parameters, set old key expiry to grace window rollover.
  - Implement `KeyRotationModal.tsx` in UI.
- [x] 2.3 Multi-Workspace Management:
  - Add `GET /v1/workspaces.list` and `POST /v1/workspaces.create`.
  - Workspace selector dropdown in UI navbar.
  - Workspace isolation for keys, analytics, and rate limiting.
- [x] 2.4 Security Audit Trail & Data Export:
  - Add `audit_logs` table in SQLite.
  - Record administrative actions (`key.created`, `key.revoked`, `key.updated`, `key.rotated`).
  - Add `GET /v1/audit.list` and `GET /v1/keys.export?format=csv|json`.
  - Add `AuditLogTable.tsx` and Export buttons in UI.
- [x] 2.5 Native Prometheus Metrics Exporter:
  - Implement `/metrics` endpoint with counters, gauges, and latency histograms.
- [x] 2.6 Key Inspector Side Drawer:
  - Implement `KeyDetailDrawer.tsx` allowing on-the-fly quota modification, toggling active/disabled state, and inspecting recent requests.
- [x] 2.7 Multi-Language Code Snippet Generator:
  - Implement `CodeGenerator.tsx` generating verified code for cURL, TypeScript, Python, Go, and Rust.
- [x] 2.8 Verification, Test Suite & GitHub Commit:
  - Run all Go and TypeScript tests, compile single-binary, and push changes to GitHub.
