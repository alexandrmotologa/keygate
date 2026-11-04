# KeyGate API Reference

KeyGate exposes two primary interfaces: an inline reverse proxy gateway and a direct JSON REST API for key verification and control plane management.

---

## Base URLs

- Reverse Proxy Gateway: `http://localhost:8000/`
- Control Plane API: `http://localhost:8000/v1`
- Studio UI: `http://localhost:8000/` (serves dashboard when accessed via browser)
- Prometheus Metrics: `http://localhost:8000/metrics`

---

## 1. Key Verification Endpoint

Verify an incoming API key programmatically without running through the reverse proxy.

### Request

```http
POST /v1/keys.verifyKey
Content-Type: application/json

{
  "key": "sk_live_ws_default_9F8k2mLx4P1qR5tY7wZ3vA6b",
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
  },
  "cached": true,
  "durationUs": 211
}
```

### Failure Codes

Error codes include:
- `KEY_NOT_FOUND`: The key does not exist or has an invalid format.
- `KEY_REVOKED`: The key was explicitly revoked.
- `KEY_EXPIRED`: The key expiration timestamp has passed.
- `QUOTA_EXCEEDED`: Lifetime request quota (`maxUses`) has been exhausted.
- `KEY_DISABLED`: Key has been toggled off administratively.
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
  "maxUses": 100,
  "scopes": ["read:data", "write:logs"],
  "ipAllowlist": ["192.168.1.0/24", "10.0.0.5"],
  "expiresAt": "2027-01-01T00:00:00Z"
}
```

### Rotate Key (Zero-Downtime)

Issues a replacement key with the same configuration and sets the old key to expire after a specified grace period.

```http
POST /v1/keys.rotateKey
Content-Type: application/json

{
  "keyId": "key_01jh8b2c4e5f6g7h8j9k0l1m",
  "gracePeriodHours": 24
}
```

Response (`200 OK`):

```json
{
  "success": true,
  "oldKeyId": "key_01jh8b2c4e5f6g7h8j9k0l1m",
  "newKeyId": "key_02ka9c3d5e6f7g8h9j0k1l2m",
  "newKey": "sk_live_ws_default_Ka72QFZeuUdrt4cgtMwrZYRbJu6FMmH1u",
  "gracePeriodHours": 24,
  "oldKeyExpiresAt": "2026-11-05T20:00:00Z"
}
```

### Get Key Details & History

```http
GET /v1/keys.getKey?id=key_01jh8b2c4e5f6g7h8j9k0l1m
```

### List Keys

```http
GET /v1/keys.listKeys?workspaceId=ws_default&status=active&search=Mobile&limit=50&offset=0
```

### Revoke Key

```http
POST /v1/keys.revokeKey
Content-Type: application/json

{
  "keyId": "key_01jh8b2c4e5f6g7h8j9k0l1m"
}
```

### Export Keys

Downloadable CSV or JSON dump of keys in a workspace:

```http
GET /v1/keys.export?workspaceId=ws_default&format=csv
GET /v1/keys.export?workspaceId=ws_default&format=json
```

---

## 3. Workspaces & Multi-Tenancy

### List Workspaces

```http
GET /v1/workspaces.list
```

### Create Workspace

```http
POST /v1/workspaces.create
Content-Type: application/json

{
  "id": "ws_staging",
  "name": "Staging Environment"
}
```

---

## 4. Security Audit Trail

```http
GET /v1/audit.list?workspaceId=ws_default&limit=50&offset=0
```

Returns chronological security logs for key creations, revocations, quota modifications, and key rotations.

---

## 5. Prometheus Metrics

```http
GET /metrics
```

Standard Prometheus text exposition format providing:
- `keygate_uptime_seconds`
- `keygate_requests_total{status="200|429|401"}`
- `keygate_active_keys`, `keygate_revoked_keys`
- `keygate_cache_events_total{result="hit|miss"}`
- `keygate_verification_latency_ms{quantile="0.5|0.95|0.99|avg"}`
