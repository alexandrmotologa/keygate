# KeyGate Architecture & Security Model

KeyGate functions as a high-throughput API key control plane and reverse proxy gateway. It provides sub-millisecond key verification, multi-tier sliding-window rate limiting, and irreversible key storage.

---

## 1. Cryptographic Key Design & Storage

### Plaintext Key Format

Keys are generated with entropy derived from `crypto/rand` and encoded with Base58:

```
sk_live_{workspace}_{24_random_bytes}
```

Example: `sk_live_prod_5K3mP8rQ9vT2xW1yZ4aB7cDe`

1. **Prefix Identification**: Enables instant routing, tenant isolation, and leak detection (e.g., GitHub secret scanners identify `sk_live_` patterns).
2. **Entropy**: 24 cryptographically random bytes provide 192 bits of entropy, exceeding NIST SP 800-131A standards.
3. **One-Time Exposure**: The full plaintext string is returned to the client exactly once upon creation. KeyGate never retains the raw key.

### Salted SHA-256 Storage

```
KeyHash = SHA-256(WorkspaceSalt + PlaintextKey)
```

Only the hex-encoded `KeyHash` is written to SQLite. If the underlying database file is dumped or exposed, offline precomputed rainbow table attacks are computationally infeasible due to the unique workspace salt. Verification utilizes constant-time comparison (`subtle.ConstantTimeCompare`) to defend against timing attacks.

---

## 2. In-Memory Verification Engine & Cache Topology

Validating an API key on every HTTP request by executing SQL queries incurs high database latency. KeyGate pairs an in-memory Ristretto cache (`github.com/dgraph-io/ristretto`) with an SQLite database configured in Write-Ahead Logging (WAL) mode.

```
                      +-----------------------------+
                      |   Incoming HTTP Request     |
                      +--------------+--------------+
                                     |
                                     v
                      +-----------------------------+
                      |  Extract Authorization /    |
                      |  X-API-Key Header           |
                      +--------------+--------------+
                                     |
                                     v
                      +-----------------------------+
                      |  Compute Salted SHA-256     |
                      |  Hash of Plaintext Key      |
                      +--------------+--------------+
                                     |
                                     v
                        /-------------------------\
                       /   Key Hash in Ristretto   \
                      <           Cache?            >
                       \                           /
                        \-------------------------/
                               /            \
                       HIT    /              \   MISS
                             v                v
             +-----------------------+   +-----------------------+
             | Retrieve Cached Key   |   | Query SQLite Database |
             | Record & Tier Info    |   | by KeyHash            |
             +-----------+-----------+   +-----------+-----------+
                         |                           |
                         |               Found?      |
                         |               /       \   |
                         |          YES /         \  | NO
                         |             v           v v
                         |      +---------------+ +---------------+
                         |      | Write Record  | | Reject: 401   |
                         |      | to Cache(5m)  | | KEY_NOT_FOUND |
                         |      +-------+-------+ +---------------+
                         |              |
                         +-------+------+
                                 |
                                 v
                 +-------------------------------+
                 | Validate Expiration, Status,  |
                 | Scopes, and IP Allowlist      |
                 +---------------+---------------+
                                 |
                                 v
                 +-------------------------------+
                 | Evaluate Sliding-Window       |
                 | Rate Limiter Counter          |
                 +---------------+---------------+
                                 |
                                 v
                 +-------------------------------+
                 | Strip Key & Forward to Target |
                 | or Return Verification JSON   |
                 +-------------------------------+
```

### Cache Latency

- Cache hit: `< 0.2ms`
- Cache miss (SQLite read): `1.5ms - 3ms`
- Cache eviction: automatic TTL (default 5 minutes), with instantaneous eviction triggered upon key revocation or tier updates.

---

## 3. Sliding-Window Rate Limiting

Fixed window counters produce traffic bursts at window boundaries (e.g., doubling allowed requests within a 2-second span around the minute mark). KeyGate avoids this using a sliding-window counter.

The sliding window calculates the estimated request count within the current dynamic window by interpolating the previous window count:

$$\text{Weight} = 1 - \frac{t - t_0}{\text{WindowSize}}$$

$$\text{EstimatedCount} = (\text{PreviousWindowCount} \times \text{Weight}) + \text{CurrentWindowCount}$$

If $\text{EstimatedCount} + 1 > \text{Limit}$, the request is rejected with `429 Too Many Requests`.

### Response Headers

KeyGate injects standard IETF RateLimit headers on all proxy responses and verification checks:

- `RateLimit-Limit`: Maximum requests permitted per window.
- `RateLimit-Remaining`: Estimated remaining quota in the current window.
- `RateLimit-Reset`: Time remaining in seconds until the current window rolls over.
- `Retry-After`: Included on `429 Too Many Requests` responses.

---

## 4. SQLite Storage Engine (WAL Mode)

KeyGate persists state inside a local SQLite database file using `modernc.org/sqlite` (pure Go, zero CGO requirements).

- **Write-Ahead Logging (WAL)**: Enabled via `PRAGMA journal_mode=WAL;`. Reads do not block writes, and writes do not block reads.
- **Synchronous Mode**: Configured to `PRAGMA synchronous=NORMAL;` for high transaction throughput while maintaining durability against application crashes.
- **Connection Pool**: Dedicated connection pool with `PRAGMA busy_timeout=5000;` to avoid database locked errors under concurrent load.
- **Tables**:
  - `workspaces`: Tenant organization and isolation.
  - `api_keys`: Hashed API keys, metadata, scopes, allowlists, rate limit policies.
  - `usage_events`: Request logs with latency, client IP, and HTTP status codes.
  - `hourly_stats`: Aggregated rollups for telemetry queries without full table scans.
