# KeyGate

Self-hosted API key gateway, sub-millisecond in-memory verification engine, and multi-tier rate limiter in Go 1.23 and React 19.

KeyGate runs as an inline reverse proxy or as a standalone key verification service. It issues prefix-encoded cryptographic keys, stores only salted SHA-256 hashes in local SQLite (WAL mode), validates incoming keys in memory via Ristretto, enforces sliding-window rate limits with standard IETF headers, and embeds a React 19 management studio.

---

## Features

- **Sub-Millisecond Verification**: In-memory cache powered by `dgraph-io/ristretto` validates active keys in under 0.5ms with automatic invalidation.
- **Irreversible Key Storage**: Issues Base58 keys formatted as `sk_live_{workspace}_{random}`. Only salted SHA-256 hashes are persisted in SQLite.
- **Sliding-Window Rate Limiter**: Multi-tier quotas (Free, Pro, Enterprise, Custom) using sliding-window counters with token-bucket burst handling.
- **Standard IETF Headers**: Returns `RateLimit-Limit`, `RateLimit-Remaining`, `RateLimit-Reset`, and `Retry-After` on all endpoints.
- **Inline Reverse Proxy**: Listens on port 8000, intercepts client keys, injects trusted identity headers (`X-KeyGate-Key-Id`, `X-KeyGate-Scopes`), strips credentials, and forwards to upstream services.
- **Standalone REST API**: Exposes direct endpoints including `POST /v1/keys.verifyKey` for microservices verifying keys out of band.
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
│  3. Verify Expiry, Scopes & IP Allowlist      │
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
  }
}
```

---

## Rate Limiting & IETF Headers

KeyGate enforces sliding-window quotas per key tier:

| Tier | Limit | Window | Burst Tolerance |
|---|---|---|---|
| `free` | 10 requests | 60 seconds | 2 requests |
| `pro` | 500 requests | 60 seconds | 25 requests |
| `enterprise` | 10,000 requests | 60 seconds | 500 requests |
| `custom` | Configurable | Configurable | Configurable |

All responses include IETF draft headers:

```http
RateLimit-Limit: 500
RateLimit-Remaining: 492
RateLimit-Reset: 28
```

If quota is exceeded:

```http
HTTP/1.1 429 Too Many Requests
Retry-After: 28
RateLimit-Limit: 500
RateLimit-Remaining: 0
RateLimit-Reset: 28
```

---

## Development & Testing

```bash
# Run backend tests
go test -v -race ./...

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
