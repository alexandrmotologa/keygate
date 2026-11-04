<p align="center">
  <img src="docs/images/logo.png?raw=true" alt="KeyGate Logo" width="130" style="border-radius: 24px;" />
</p>

<h1 align="center">KeyGate</h1>

<p align="center">
  <strong>Self-Hosted API Key Gateway, Sub-Millisecond In-Memory Verification Engine & Multi-Tier Rate Limiter</strong>
</p>

<p align="center">
  <a href="https://github.com/alexandrmotologa/keygate/actions"><img src="https://img.shields.io/badge/build-passing-brightgreen?style=flat-square" alt="Build Status" /></a>
  <a href="https://golang.org"><img src="https://img.shields.io/badge/go-1.23%20%7C%201.27-00ADD8?style=flat-square&logo=go" alt="Go Version" /></a>
  <a href="https://react.dev"><img src="https://img.shields.io/badge/react-19-61DAFB?style=flat-square&logo=react" alt="React 19" /></a>
  <a href="https://sqlite.org"><img src="https://img.shields.io/badge/sqlite-pure%20go%20(no%20cgo)-003B57?style=flat-square&logo=sqlite" alt="SQLite" /></a>
  <a href="https://prometheus.io"><img src="https://img.shields.io/badge/metrics-prometheus-E6522C?style=flat-square&logo=prometheus" alt="Prometheus" /></a>
  <a href="LICENSE"><img src="https://img.shields.io/badge/license-MIT-blue?style=flat-square" alt="License" /></a>
</p>

<p align="center">
  <img src="docs/images/keygate_demo.gif?raw=true" alt="KeyGate Execution Demo" width="920" style="border-radius: 12px; box-shadow: 0 12px 40px rgba(0,0,0,0.35);" />
</p>

KeyGate is an open-source, self-hosted alternative to Unkey.dev built with Go 1.23, pure-Go SQLite (WAL mode), in-memory Ristretto caching, and an embedded React 19 control plane.

It operates either as an inline reverse proxy on port 8000 or as an out-of-band verification API (`/v1/keys.verifyKey`). KeyGate issues prefix-encoded cryptographic keys, persists only salted SHA-256 hashes, validates active credentials in memory with sub-millisecond latency (211 ns/op cache hit benchmark), enforces sliding-window rate limits with standard IETF headers, and delivers real-time telemetry through Server-Sent Events.

---

## Features

- **Sub-Millisecond Verification**: In-memory cache powered by `dgraph-io/ristretto` validates active keys in under 0.5ms (benchmark: 211 ns/op) with automatic invalidation on mutation.
- **Irreversible Key Storage**: Issues Base58 keys formatted as `sk_live_{workspace}_{random}`. Only salted SHA-256 hashes are persisted in SQLite.
- **Sliding-Window Rate Limiter**: Multi-tier quotas (Free, Pro, Enterprise, Custom) using sliding-window counters with token-bucket burst handling.
- **Standard IETF Headers**: Returns `RateLimit-Limit`, `RateLimit-Remaining`, `RateLimit-Reset`, and `Retry-After` on all endpoints.
- **Ephemeral & Quota-Capped Keys (`maxUses`)**: Set lifetime call limits for AI agents and temporary integrations; automatically disabled upon quota exhaustion.
- **Zero-Downtime Key Rotation**: Rotate keys with configurable overlap grace periods (e.g. 24h) ensuring zero client outages during credential transition.
- **Key Inspector & Live Edit Drawer**: Modify quotas, scopes, IP allowlists, and toggle active/disabled status on the fly without reissuing keys.
- **Multi-Workspace Isolation**: Organize and separate keys, quotas, and audit logs by environment (`Production`, `Staging`, `Development`) or tenant.
- **Multi-Language Code Generator**: Generates ready-to-paste integration snippets in cURL, TypeScript, Python, Go, and Rust.
- **Security Audit Trail & Data Export**: Immutable ledger of administrative actions and 1-click export of keys to CSV and JSON.
- **Native Prometheus Exporter**: Exposes `/metrics` in standard Prometheus text format for Grafana and SRE observability.
- **Inline Reverse Proxy**: Listens on port 8000, intercepts client keys, injects trusted identity headers (`X-KeyGate-Key-Id`, `X-KeyGate-Scopes`), strips credentials, and forwards to upstream services.
- **Embedded Web Studio**: React 19 dashboard compiled directly into the Go binary with live request streaming (SSE), key issuance, and in-browser API testing.
- **Zero CGO Dependency**: Uses `modernc.org/sqlite` with WAL mode for portable, single-binary cross-compilation.

---

## Interface Showcase

### System Control Plane & 24-Hour Traffic Volume
Overview dashboard featuring real-time KPI cards (Active Keys, Total Throughput, P99 Latency, Blocked Requests), hourly distribution charts, and plan breakdown.

<p align="center">
  <img src="docs/images/keygate_overview.png?raw=true" alt="KeyGate System Control Plane" width="920" style="border-radius: 10px; border: 1px solid #1e293b;" />
</p>

### API Key Management & Quota Caps
Searchable, paginated key table with active status indicators, lifetime usage progress bars (`maxUses`), export to CSV/JSON, and credential rotation controls.

<p align="center">
  <img src="docs/images/keygate_keys.png?raw=true" alt="KeyGate API Key Table" width="920" style="border-radius: 10px; border: 1px solid #1e293b;" />
</p>

### Key Inspector Drawer & Live Reconfiguration
Slide-over configuration drawer enabling real-time adjustments to rate limits, window durations, burst capacity, IP allowlists, and permission scopes without reissuing credentials.

<p align="center">
  <img src="docs/images/keygate_drawer.png?raw=true" alt="KeyGate Key Detail Drawer" width="920" style="border-radius: 10px; border: 1px solid #1e293b;" />
</p>

### Interactive API Verification Playground & Code Generator
In-browser testing workbench to simulate key verification requests with instant JSON output, header inspection, and multi-language snippets in cURL, TypeScript, Python, Go, and Rust.

<p align="center">
  <img src="docs/images/keygate_tester.png?raw=true" alt="KeyGate API Playground" width="920" style="border-radius: 10px; border: 1px solid #1e293b;" />
</p>

### Security Audit Trail
Comprehensive chronological ledger recording administrative actions including key issuance, quota updates, rotations, and revocations with actor IP tracking.

<p align="center">
  <img src="docs/images/keygate_audit.png?raw=true" alt="KeyGate Security Audit Trail" width="920" style="border-radius: 10px; border: 1px solid #1e293b;" />
</p>

### Adaptive Mobile & Tablet Experience
Fully responsive interface optimized for mobile and tablet devices with touch-friendly navigation tabs and compact workspace switching.

<p align="center">
  <img src="docs/images/keygate_mobile.png?raw=true" alt="KeyGate Mobile Layout" width="340" style="border-radius: 12px; border: 1px solid #1e293b;" />
</p>

---

## Quick Start

### 1. Build and Run

```bash
# Clone the repository
git clone https://github.com/alexandrmotologa/keygate.git
cd keygate

# Build single binary
go build -o bin/keygate.exe cmd/keygate/main.go

# Start KeyGate with internal mock upstream
./bin/keygate.exe serve --port 8000 --mock-upstream --db ./keygate.db
```

Access the embedded web studio at `http://localhost:8000`.

### 2. Reverse Proxy Mode

Route traffic through KeyGate to protect an upstream backend (such as an internal API running on port 3000):

```bash
./bin/keygate.exe serve --port 8000 --target http://localhost:3000 --db ./keygate.db
```

Incoming requests to `http://localhost:8000` with header `Authorization: Bearer sk_live_...` are verified, rate limited, stripped of credentials, and forwarded upstream with injected `X-KeyGate-Key-Id`, `X-KeyGate-Workspace`, and `X-KeyGate-Scopes` headers.

---

## CLI Reference

```bash
# Start server
keygate serve --port 8000 --target http://localhost:3000 --db ./keygate.db

# Issue an API key via CLI
keygate key create --name "Backend Service" --tier pro --workspace ws_default

# Issue an ephemeral key with lifetime call limit
keygate key create --name "Claude Agent" --tier free --max-uses 500

# Verify an API key directly
keygate key verify --key sk_live_ws_default_...

# Revoke a key
keygate key revoke --id key_01jh8b2c4e5f6g7h8j9k0l1m

# List keys in a workspace
keygate key list --workspace ws_default

# Run throughput and latency benchmark
keygate benchmark --keys 1000 --concurrency 50 --requests 50000
```

---

## Architectural Workflow

```
Client Request
      │  (Authorization: Bearer sk_live_ws_default_...)
      ▼
┌───────────────────────────────────────────────┐
│              KeyGate Proxy Gateway            │
├───────────────────────────────────────────────┤
│  1. Parse Prefix & Workspace ID               │
│  2. In-Memory Cache Lookup (Ristretto)        │
│     ├── Hit:  < 0.2ms                         │
│     └── Miss: Read SQLite (WAL) & Cache       │
│  3. Verify Expiry, Max Uses & IP Allowlist    │
│  4. Evaluate Sliding-Window Rate Limiter      │
│  5. Strip Key & Inject X-KeyGate-* Headers    │
└───────────────────────┬───────────────────────┘
                        │
                        ▼ (Forwarded HTTP Request)
         ┌──────────────────────────────┐
         │       Upstream Service       │
         │  (Port 3000 / Internal API)  │
         └──────────────────────────────┘
```

---

## REST Endpoints

### Direct Key Verification (`POST /v1/keys.verifyKey`)

Request:

```http
POST /v1/keys.verifyKey HTTP/1.1
Host: localhost:8000
Content-Type: application/json

{
  "key": "sk_live_ws_default_9xJkL2mNpQrStUvWxYzAbCdEf",
  "workspaceId": "ws_default"
}
```

Response:

```json
{
  "valid": true,
  "code": "VALID",
  "keyId": "key_01jh8b2c4e5f6g7h8j9k0l1m",
  "ownerId": "usr_alex",
  "workspaceId": "ws_default",
  "name": "Frontend Web Client",
  "tier": "pro",
  "scopes": ["read:analytics", "write:keys"],
  "rateLimit": {
    "limit": 500,
    "remaining": 499,
    "reset": 58
  },
  "cached": true,
  "durationUs": 211
}
```

---

## Prometheus Observability

KeyGate includes an integrated Prometheus `/metrics` endpoint:

```
# HELP keygate_requests_total Total API requests processed
# TYPE keygate_requests_total counter
keygate_requests_total{status="200"} 12450
keygate_requests_total{status="429"} 14
keygate_requests_total{status="401"} 8

# HELP keygate_cache_events_total In-memory Ristretto cache events
# TYPE keygate_cache_events_total counter
keygate_cache_events_total{result="hit"} 12432
keygate_cache_events_total{result="miss"} 18

# HELP keygate_verification_latency_ms Verification latency percentiles
# TYPE keygate_verification_latency_ms gauge
keygate_verification_latency_ms{quantile="0.5"} 0.0002
keygate_verification_latency_ms{quantile="0.99"} 0.0005
```

---

## Running Tests

```bash
# Run backend tests
go test -v -count=1 ./...

# Run frontend development server
cd web
npm run dev
```

---

## License

MIT License. Designed and engineered for high-throughput self-hosted workloads.
