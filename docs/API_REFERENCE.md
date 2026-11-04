# KeyGate API Reference

KeyGate exposes two primary interfaces: an inline reverse proxy gateway and a direct JSON REST API for key verification and control plane management.

---

## Base URLs

- Reverse Proxy Gateway: `http://localhost:8000/`
- Control Plane API: `http://localhost:8000/v1`
- Studio UI: `http://localhost:8000/` (serves dashboard when accessed via browser)

---

## 1. Key Verification Endpoint

Verify an incoming API key programmatically without running through the reverse proxy.

### Request

```http
POST /v1/keys.verifyKey
Content-Type: application/json

{
  "key": "sk_live_prod_9F8k2mLx4P1qR5tY7wZ3vA6b",
  "workspaceId": "ws_default",
  "requiredScopes": ["read:users"]
}
```

#### Fields

| Field | Type | Required | Description |
|---|---|---|---|
| `key` | string | Yes | Full plaintext API key presented by the client. |
| `workspaceId` | string | No | Restricts validation to a specific workspace. |
| `requiredScopes` | array | No | List of required permission scopes. All must be present on the key. |

### Successful Response (`200 OK`)

```json
{
  "valid": true,
  "code": "VALID",
  "keyId": "key_01jh8b2c4e5f6g7h8j9k0l1m",
  "ownerId": "usr_9981",
  "workspaceId": "ws_default",
  "name": "Production Backend",
  "scopes": ["read:users", "write:users"],
  "rateLimit": {
    "tier": "pro",
    "limit": 500,
    "remaining": 499,
    "reset": 58
  }
}
```

### Failure Responses

#### Invalid or Revoked Key (`200 OK` with valid: false)

```json
{
  "valid": false,
  "code": "KEY_REVOKED",
  "message": "API key has been revoked."
}
```

Error codes include:
- `KEY_NOT_FOUND`: The key does not exist or has an invalid format.
- `KEY_REVOKED`: The key was explicitly revoked.
- `KEY_EXPIRED`: The key expiration timestamp has passed.
- `INSUFFICIENT_SCOPES`: The key lacks one or more required scopes.
- `IP_FORBIDDEN`: Client IP address is not included in the key allowlist.
- `RATE_LIMITED`: The key exceeded its sliding window request quota.

---

## 2. Key Management Endpoints

### Create Key

Generates a new prefix-encoded cryptographic API key. The plaintext key is returned once in the response.

```http
POST /v1/keys.createKey
Content-Type: application/json

{
  "name": "Mobile Client Service",
  "workspaceId": "ws_default",
  "prefix": "sk_live",
  "ownerId": "usr_102",
  "tier": "pro",
  "rateLimit": 500,
  "rateDuration": 60,
  "scopes": ["read:data", "write:logs"],
  "ipAllowlist": ["192.168.1.0/24", "10.0.0.5"],
  "expiresAt": "2027-01-01T00:00:00Z",
  "metadata": {
    "department": "core-infrastructure",
    "environment": "production"
  }
}
```

#### Response (`201 Created`)

```json
{
  "keyId": "key_01jh8b2c4e5f6g7h8j9k0l1m",
  "key": "sk_live_ws_default_8F7d6c5b4a3...plaintext",
  "prefix": "sk_live",
  "name": "Mobile Client Service",
  "workspaceId": "ws_default",
  "ownerId": "usr_102",
  "tier": "pro",
  "rateLimit": 500,
  "rateDuration": 60,
  "scopes": ["read:data", "write:logs"],
  "ipAllowlist": ["192.168.1.0/24", "10.0.0.5"],
  "expiresAt": "2027-01-01T00:00:00Z",
  "createdAt": "2026-11-04T19:40:00Z"
}
```

### List Keys

Retrieves active and revoked keys.

```http
GET /v1/keys.listKeys?workspaceId=ws_default&status=active&search=Mobile&limit=20&offset=0
```

#### Query Parameters

| Parameter | Type | Default | Description |
|---|---|---|---|
| `workspaceId` | string | `""` | Filter by workspace ID. |
| `status` | string | `all` | Filter by status (`active`, `revoked`, `expired`, `all`). |
| `search` | string | `""` | Search substring against key name, prefix, or owner ID. |
| `limit` | integer | `50` | Maximum items to return. |
| `offset` | integer | `0` | Number of items to skip. |

### Revoke Key

Revokes an API key immediately and invalidates its in-memory cache entry.

```http
POST /v1/keys.revokeKey
Content-Type: application/json

{
  "keyId": "key_01jh8b2c4e5f6g7h8j9k0l1m"
}
```

#### Response (`200 OK`)

```json
{
  "success": true,
  "keyId": "key_01jh8b2c4e5f6g7h8j9k0l1m",
  "revokedAt": "2026-11-04T19:42:15Z"
}
```

---

## 3. Reverse Proxy Gateway

When KeyGate runs in proxy mode, incoming client traffic passes through port 8000 and is routed to the configured upstream server (for example, `http://localhost:3000`).

### Authentication Headers

Clients provide authentication using standard headers:

```http
GET /api/v1/users HTTP/1.1
Host: api.yourcompany.com
Authorization: Bearer sk_live_ws_default_9F8k2mLx...
```

or via:

```http
GET /api/v1/users HTTP/1.1
Host: api.yourcompany.com
X-API-Key: sk_live_ws_default_9F8k2mLx...
```

### Headers Injected Upstream

When the request is authenticated, KeyGate strips the client API key and injects trusted identity headers:

- `X-KeyGate-Key-Id`: Internal key identifier.
- `X-KeyGate-Owner-Id`: Owner or tenant ID.
- `X-KeyGate-Workspace`: Workspace identifier.
- `X-KeyGate-Scopes`: Comma-separated permissions granted to this key.
- `X-KeyGate-Tier`: Rate limit tier (`free`, `pro`, `enterprise`, or `custom`).

### Downstream RateLimit Headers (IETF Compliant)

All responses returned to the client include standardized rate limit headers:

```http
HTTP/1.1 200 OK
Content-Type: application/json
RateLimit-Limit: 500
RateLimit-Remaining: 482
RateLimit-Reset: 34
```

When quota is exhausted:

```http
HTTP/1.1 429 Too Many Requests
Content-Type: application/json
Retry-After: 34
RateLimit-Limit: 500
RateLimit-Remaining: 0
RateLimit-Reset: 34

{
  "error": "rate_limit_exceeded",
  "message": "Sliding window rate limit reached. Retry in 34 seconds."
}
```

---

## 4. Telemetry & Analytics

### Summary Metrics

```http
GET /v1/analytics.summary?workspaceId=ws_default&timeRange=24h
```

Returns aggregate verifications, latency percentiles (p50, p95, p99), error rates, and volume per tier.

### Live Telemetry Stream (Server-Sent Events)

```http
GET /v1/analytics.stream
Accept: text/event-stream
```

Streams real-time verification and proxy events as JSON messages for live monitoring dashboards.
