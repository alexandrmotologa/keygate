# KeyGate

Self-hosted API key gateway, sub-millisecond in-memory verification engine, and multi-tier rate limiter in Go 1.23 and React 19.

KeyGate runs as an inline reverse proxy or as a standalone key verification service. It issues prefix-encoded cryptographic keys, stores only salted SHA-256 hashes in local SQLite (WAL mode), validates incoming keys in memory via Ristretto, enforces sliding-window rate limits with standard IETF headers, and embeds a React 19 management studio.

---

## Features

- **Sub-Millisecond Verification**: In-memory cache powered by `dgraph-io/ristretto` validates active keys in under 0.5ms (benchmark: 211 ns/op) with automatic invalidation.
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

## Quick Start

### 1. Run via Prebuilt Binary or Go

```bash
# Clone the repository
git clone https://github.com/alexandrmotologa/keygate.git
cd keygate

# Build single binary
go build -o bin/keygate cmd/keygate/main.go

# Start KeyGate with internal mock upstream
./bin/keygate serve --port 8000 --mock-upstream --db ./keygate.db
```

Access the embedded web studio at `http://localhost:8000`.

### 2. Reverse Proxy Mode

Route traffic through KeyGate to protect an upstream backend (such as an internal Node.js or Python API running on port 3000):

```bash
./bin/keygate serve --port 8000 --target http://localhost:3000 --db ./keygate.db
```

Incoming requests to `http://localhost:8000` with header `Authorization: Bearer sk_live_...` are verified, rate limited, and forwarded upstream with injected `X-KeyGate-*` headers.

---

## CLI Commands

```bash
# Start server
keygate serve --port 8000 --target http://localhost:3000 --db ./keygate.db

# Issue an API key via CLI
keygate key create --name "Backend Service" --tier pro --workspace ws_default

# Verify a key
keygate key verify --key sk_live_ws_default_...

# Revoke a key
keygate key revoke --id key_01jh8b2c4e5f6g7h8j9k0l1m

# List keys in a workspace
keygate key list --workspace ws_default

# Run throughput and latency benchmark
keygate benchmark --keys 1000 --concurrency 50 --requests 50000
```

---

## Architecture Overview

```
Client Request
      │  (Authorization: Bearer sk_live_...)
      ▼
┌───────────────────────────────────────────────┐
│                KeyGate Proxy                  │
│                                               │
│  1. Extract & compute Salted SHA-256 Hash     │
│  2. In-Memory Cache Lookup (Ristretto)        │
│     ├── Hit:  < 0.2ms                         │
│     └── Miss: Read SQLite (WAL) & Cache       │
│  3. Verify Expiry, Max Uses & IP Allowlist    │
│  4. Evaluate Sliding-Window Rate Limiter      │
│  5. Strip Key & Inject X-KeyGate-* Headers    │
└───────────────────────┬───────────────────────┘
                        │
                        ▼ (Clean, Authenticated Request)
             Target Upstream Server
```

For detailed specifications, see:
- [Architecture & Security Model](docs/ARCHITECTURE.md)
- [REST API Reference](docs/API_REFERENCE.md)

---

## Direct Verification API

For microservices that verify API keys directly rather than through a proxy:

```bash
curl -X POST http://localhost:8000/v1/keys.verifyKey \
  -H "Content-Type: application/json" \
  -d '{
    "key": "sk_live_ws_default_example123456",
    "requiredScopes": ["read:users"]
  }'
```

Response:

```json
{
  "valid": true,
  "code": "VALID",
  "keyId": "key_01jh8b2c4e5f6g7h8j9k0l1m",
  "ownerId": "usr_9981",
  "workspaceId": "ws_default",
  "name": "Backend Service",
  "scopes": ["read:users", "write:users"],
  "rateLimit": {
    "tier": "pro",
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

## Development & Testing

```bash
# Run backend tests
go test -v -count=1 ./...

# Run frontend development server
cd web
npm install
npm run dev

# Build frontend assets for single binary embedding
npm run build
cd ..
go build -o bin/keygate cmd/keygate/main.go
```

---

## License

MIT License. See [LICENSE](LICENSE) for details.
