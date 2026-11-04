package keys

import (
	"context"
	"fmt"
	"net"
	"strings"
	"sync"
	"time"

	"github.com/alexandrmotologa/keygate/internal/storage"
	"github.com/dgraph-io/ristretto"
)

// ValidationCode indicates the specific reason for key validity or failure.
type ValidationCode string

const (
	CodeValid              ValidationCode = "VALID"
	CodeKeyNotFound        ValidationCode = "KEY_NOT_FOUND"
	CodeKeyRevoked         ValidationCode = "KEY_REVOKED"
	CodeKeyExpired         ValidationCode = "KEY_EXPIRED"
	CodeKeyDisabled        ValidationCode = "KEY_DISABLED"
	CodeInsufficientScopes ValidationCode = "INSUFFICIENT_SCOPES"
	CodeIPForbidden        ValidationCode = "IP_FORBIDDEN"
	CodeRateLimited        ValidationCode = "RATE_LIMITED"
	CodeInternalError      ValidationCode = "INTERNAL_ERROR"
)

// ValidationRequest contains parameters required to authenticate a key.
type ValidationRequest struct {
	RawKey         string   `json:"key"`
	WorkspaceID    string   `json:"workspaceId,omitempty"`
	RequiredScopes []string `json:"requiredScopes,omitempty"`
	ClientIP       string   `json:"clientIp,omitempty"`
}

// ValidationResponse describes the result of key verification.
type ValidationResponse struct {
	Valid      bool            `json:"valid"`
	Code       ValidationCode  `json:"code"`
	Message    string          `json:"message"`
	Key        *storage.APIKey `json:"key,omitempty"`
	Cached     bool            `json:"cached"`
	DurationUs int64           `json:"durationUs"`
}

// Validator validates API keys with an in-memory Ristretto cache and SQLite fallback.
type Validator struct {
	db          *storage.DB
	cache       *ristretto.Cache
	idToHashMap sync.Map
	saltCache   sync.Map
	ttl         time.Duration
}

// ValidatorConfig configures cache size and TTL.
type ValidatorConfig struct {
	NumCounters int64         // default 1_000_000
	MaxCost     int64         // default 100_000 items
	TTL         time.Duration // default 5 minutes
}

// NewValidator initializes the in-memory cache and connects storage.
func NewValidator(db *storage.DB, cfg ...ValidatorConfig) (*Validator, error) {
	numCounters := int64(1_000_000)
	maxCost := int64(100_000)
	ttl := 5 * time.Minute

	if len(cfg) > 0 {
		if cfg[0].NumCounters > 0 {
			numCounters = cfg[0].NumCounters
		}
		if cfg[0].MaxCost > 0 {
			maxCost = cfg[0].MaxCost
		}
		if cfg[0].TTL > 0 {
			ttl = cfg[0].TTL
		}
	}

	cache, err := ristretto.NewCache(&ristretto.Config{
		NumCounters: numCounters,
		MaxCost:     maxCost,
		BufferItems: 64,
	})
	if err != nil {
		return nil, fmt.Errorf("failed to initialize ristretto cache: %w", err)
	}

	return &Validator{
		db:    db,
		cache: cache,
		ttl:   ttl,
	}, nil
}

// Close cleans up cache resources.
func (v *Validator) Close() {
	if v.cache != nil {
		v.cache.Close()
	}
}

// getSalt returns cached or persistent workspace salt.
func (v *Validator) getSalt(ctx context.Context, workspaceID string) (string, error) {
	if val, ok := v.saltCache.Load(workspaceID); ok {
		return val.(string), nil
	}

	salt, err := v.db.GetWorkspaceSalt(ctx, workspaceID)
	if err != nil {
		return "", err
	}
	v.saltCache.Store(workspaceID, salt)
	return salt, nil
}

// Verify validates an incoming API key against cache, storage, scopes, and IP rules.
func (v *Validator) Verify(ctx context.Context, req ValidationRequest) *ValidationResponse {
	start := time.Now()
	rawKey := strings.TrimSpace(req.RawKey)

	if rawKey == "" {
		return &ValidationResponse{
			Valid:      false,
			Code:       CodeKeyNotFound,
			Message:    "API key is missing or empty.",
			DurationUs: time.Since(start).Microseconds(),
		}
	}

	wsID := strings.TrimSpace(req.WorkspaceID)
	if wsID == "" {
		_, parsedWs, _, err := ParseKey(rawKey)
		if err == nil && parsedWs != "" {
			wsID = parsedWs
		} else {
			wsID = "ws_default"
		}
	}

	salt, err := v.getSalt(ctx, wsID)
	if err != nil {
		return &ValidationResponse{
			Valid:      false,
			Code:       CodeInternalError,
			Message:    fmt.Sprintf("Failed to load workspace salt: %v", err),
			DurationUs: time.Since(start).Microseconds(),
		}
	}

	keyHash := HashKey(rawKey, salt)

	// Step 1: Check in-memory cache
	var apiKey *storage.APIKey
	cached := false

	if val, found := v.cache.Get(keyHash); found {
		if k, ok := val.(*storage.APIKey); ok {
			apiKey = k
			cached = true
		}
	}

	// Step 2: Database fallback on cache miss
	if apiKey == nil {
		dbKey, err := v.db.GetKeyByHash(ctx, keyHash)
		if err != nil {
			return &ValidationResponse{
				Valid:      false,
				Code:       CodeInternalError,
				Message:    fmt.Sprintf("Storage lookup error: %v", err),
				DurationUs: time.Since(start).Microseconds(),
			}
		}
		if dbKey == nil {
			return &ValidationResponse{
				Valid:      false,
				Code:       CodeKeyNotFound,
				Message:    "API key not found.",
				DurationUs: time.Since(start).Microseconds(),
			}
		}

		apiKey = dbKey
		v.cache.SetWithTTL(keyHash, apiKey, 1, v.ttl)
		v.idToHashMap.Store(apiKey.ID, keyHash)
	}

	// Step 3: Check active/revocation/expiry status
	now := time.Now().UTC()
	if apiKey.RevokedAt != nil && !apiKey.RevokedAt.IsZero() {
		return &ValidationResponse{
			Valid:      false,
			Code:       CodeKeyRevoked,
			Message:    "API key has been revoked.",
			Key:        apiKey,
			Cached:     cached,
			DurationUs: time.Since(start).Microseconds(),
		}
	}

	if apiKey.ExpiresAt != nil && !apiKey.ExpiresAt.IsZero() && now.After(*apiKey.ExpiresAt) {
		return &ValidationResponse{
			Valid:      false,
			Code:       CodeKeyExpired,
			Message:    "API key has expired.",
			Key:        apiKey,
			Cached:     cached,
			DurationUs: time.Since(start).Microseconds(),
		}
	}

	if !apiKey.Enabled {
		return &ValidationResponse{
			Valid:      false,
			Code:       CodeKeyDisabled,
			Message:    "API key has been disabled.",
			Key:        apiKey,
			Cached:     cached,
			DurationUs: time.Since(start).Microseconds(),
		}
	}

	// Step 4: Check IP Allowlist
	if len(apiKey.IPAllowlist) > 0 && req.ClientIP != "" {
		if !checkIPAllowlist(req.ClientIP, apiKey.IPAllowlist) {
			return &ValidationResponse{
				Valid:      false,
				Code:       CodeIPForbidden,
				Message:    fmt.Sprintf("Client IP %s is not permitted for this API key.", req.ClientIP),
				Key:        apiKey,
				Cached:     cached,
				DurationUs: time.Since(start).Microseconds(),
			}
		}
	}

	// Step 5: Check Scopes
	if len(req.RequiredScopes) > 0 {
		if !hasRequiredScopes(apiKey.Scopes, req.RequiredScopes) {
			return &ValidationResponse{
				Valid:      false,
				Code:       CodeInsufficientScopes,
				Message:    "API key does not possess the required permission scopes.",
				Key:        apiKey,
				Cached:     cached,
				DurationUs: time.Since(start).Microseconds(),
			}
		}
	}

	return &ValidationResponse{
		Valid:      true,
		Code:       CodeValid,
		Message:    "API key verified successfully.",
		Key:        apiKey,
		Cached:     cached,
		DurationUs: time.Since(start).Microseconds(),
	}
}

// Invalidate removes a key from the in-memory cache by hash.
func (v *Validator) Invalidate(keyHash string) {
	v.cache.Del(keyHash)
}

// InvalidateByID removes a key from cache using its key ID.
func (v *Validator) InvalidateByID(keyID string) {
	if hashVal, ok := v.idToHashMap.Load(keyID); ok {
		if hashStr, ok := hashVal.(string); ok {
			v.cache.Del(hashStr)
			v.idToHashMap.Delete(keyID)
		}
	}
}

// InvalidateWorkspaceSalt clears salt cache for a workspace.
func (v *Validator) InvalidateWorkspaceSalt(workspaceID string) {
	v.saltCache.Delete(workspaceID)
}

func checkIPAllowlist(clientIP string, allowlist []string) bool {
	parsedClientIP := net.ParseIP(strings.TrimSpace(clientIP))
	if parsedClientIP == nil {
		return false
	}

	for _, item := range allowlist {
		item = strings.TrimSpace(item)
		if strings.Contains(item, "/") {
			_, ipNet, err := net.ParseCIDR(item)
			if err == nil && ipNet.Contains(parsedClientIP) {
				return true
			}
		} else {
			targetIP := net.ParseIP(item)
			if targetIP != nil && targetIP.Equal(parsedClientIP) {
				return true
			}
		}
	}
	return false
}

func hasRequiredScopes(keyScopes, requiredScopes []string) bool {
	scopeMap := make(map[string]bool, len(keyScopes))
	for _, s := range keyScopes {
		scopeMap[s] = true
	}
	for _, req := range requiredScopes {
		if !scopeMap[req] {
			return false
		}
	}
	return true
}
