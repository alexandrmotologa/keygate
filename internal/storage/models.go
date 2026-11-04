package storage

import (
	"time"
)

// Workspace represents an isolated tenant environment.
type Workspace struct {
	ID        string    `json:"id"`
	Name      string    `json:"name"`
	Salt      string    `json:"-"`
	CreatedAt time.Time `json:"createdAt"`
	UpdatedAt time.Time `json:"updatedAt"`
}

// APIKey represents an issued API key with security metadata.
// The raw key is never stored; only KeyHash is persisted.
type APIKey struct {
	ID            string            `json:"id"`
	KeyHash       string            `json:"-"`
	Prefix        string            `json:"prefix"`
	Name          string            `json:"name"`
	OwnerID       string            `json:"ownerId"`
	WorkspaceID   string            `json:"workspaceId"`
	Tier          string            `json:"tier"`
	RateLimit     int               `json:"rateLimit"`
	RateDuration  int               `json:"rateDuration"`
	Burst         int               `json:"burst"`
	Scopes        []string          `json:"scopes"`
	IPAllowlist   []string          `json:"ipAllowlist"`
	Metadata      map[string]string `json:"metadata"`
	MaxUses       int               `json:"maxUses"` // 0 = unlimited usage
	ExpiresAt     *time.Time        `json:"expiresAt,omitempty"`
	RevokedAt     *time.Time        `json:"revokedAt,omitempty"`
	Enabled       bool              `json:"enabled"`
	TotalRequests int64             `json:"totalRequests"`
	LastUsedAt    *time.Time        `json:"lastUsedAt,omitempty"`
	CreatedAt     time.Time         `json:"createdAt"`
	UpdatedAt     time.Time         `json:"updatedAt"`
}

// IsActive returns true if the key is enabled, not revoked, not expired, and has not exceeded max uses.
func (k *APIKey) IsActive(now time.Time) bool {
	if !k.Enabled {
		return false
	}
	if k.RevokedAt != nil && !k.RevokedAt.IsZero() {
		return false
	}
	if k.ExpiresAt != nil && !k.ExpiresAt.IsZero() && now.After(*k.ExpiresAt) {
		return false
	}
	if k.MaxUses > 0 && k.TotalRequests >= int64(k.MaxUses) {
		return false
	}
	return true
}

// AuditLog records administrative security mutations.
type AuditLog struct {
	ID          int64             `json:"id"`
	WorkspaceID string            `json:"workspaceId"`
	Actor       string            `json:"actor"`
	Action      string            `json:"action"` // key.created, key.revoked, key.updated, key.rotated, workspace.created
	TargetID    string            `json:"targetId"`
	Details     map[string]string `json:"details"`
	Timestamp   time.Time         `json:"timestamp"`
}

// UsageEvent records telemetry for a verified or rejected request.
type UsageEvent struct {
	ID          int64     `json:"id"`
	KeyID       string    `json:"keyId"`
	WorkspaceID string    `json:"workspaceId"`
	StatusCode  int       `json:"statusCode"`
	LatencyMs   float64   `json:"latencyMs"`
	ClientIP    string    `json:"clientIp"`
	Path        string    `json:"path"`
	Method      string    `json:"method"`
	Timestamp   time.Time `json:"timestamp"`
}

// HourlyBucketData provides aggregate counts for time-series charts.
type HourlyBucketData struct {
	Timestamp            string  `json:"timestamp"`
	TotalRequests        int64   `json:"totalRequests"`
	SuccessRequests      int64   `json:"successRequests"`
	RateLimitedRequests  int64   `json:"rateLimitedRequests"`
	UnauthorizedRequests int64   `json:"unauthorizedRequests"`
	AvgLatencyMs         float64 `json:"avgLatencyMs"`
}

// AnalyticsSummary represents high-level metrics across the gateway.
type AnalyticsSummary struct {
	TotalKeys            int64              `json:"totalKeys"`
	ActiveKeys           int64              `json:"activeKeys"`
	RevokedKeys          int64              `json:"revokedKeys"`
	TotalRequests        int64              `json:"totalRequests"`
	SuccessRequests      int64              `json:"successRequests"`
	RateLimitedRequests  int64              `json:"rateLimitedRequests"`
	UnauthorizedRequests int64              `json:"unauthorizedRequests"`
	AvgLatencyMs         float64            `json:"avgLatencyMs"`
	P50LatencyMs         float64            `json:"p50LatencyMs"`
	P95LatencyMs         float64            `json:"p95LatencyMs"`
	P99LatencyMs         float64            `json:"p99LatencyMs"`
	RequestsByTier       map[string]int64   `json:"requestsByTier"`
	HourlyBuckets        []HourlyBucketData `json:"hourlyBuckets"`
}

// RateLimitTier defines preset throttling parameters.
type RateLimitTier struct {
	Name     string `json:"name"`
	Limit    int    `json:"limit"`
	Duration int    `json:"duration"`
	Burst    int    `json:"burst"`
}
