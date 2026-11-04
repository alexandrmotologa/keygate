package server

import (
	"context"
	"encoding/json"
	"fmt"
	"net/http"
	"strconv"
	"strings"
	"time"

	"github.com/alexandrmotologa/keygate/internal/keys"
	"github.com/alexandrmotologa/keygate/internal/proxy"
	"github.com/alexandrmotologa/keygate/internal/ratelimit"
	"github.com/alexandrmotologa/keygate/internal/storage"
)

// API handles control plane HTTP endpoints.
type API struct {
	db        *storage.DB
	validator *keys.Validator
	limiter   *ratelimit.Limiter
	broker    *EventBroker
}

// APIConfig holds dependencies for the REST API server.
type APIConfig struct {
	DB        *storage.DB
	Validator *keys.Validator
	Limiter   *ratelimit.Limiter
	Broker    *EventBroker
}

// NewAPI initializes the REST API router.
func NewAPI(cfg APIConfig) *API {
	return &API{
		db:        cfg.DB,
		validator: cfg.Validator,
		limiter:   cfg.Limiter,
		broker:    cfg.Broker,
	}
}

// RegisterRoutes registers all API endpoints on the given ServeMux.
func (a *API) RegisterRoutes(mux *http.ServeMux) {
	mux.HandleFunc("/v1/keys.verifyKey", a.handleVerifyKey)
	mux.HandleFunc("/v1/keys.createKey", a.handleCreateKey)
	mux.HandleFunc("/v1/keys.listKeys", a.handleListKeys)
	mux.HandleFunc("/v1/keys.revokeKey", a.handleRevokeKey)
	mux.HandleFunc("/v1/keys.updateKey", a.handleUpdateKey)
	mux.HandleFunc("/v1/analytics.summary", a.handleAnalyticsSummary)
	mux.Handle("/v1/analytics.stream", a.broker)
	mux.HandleFunc("/healthz", a.handleHealthCheck)
}

func (a *API) respondJSON(w http.ResponseWriter, status int, data interface{}) {
	w.Header().Set("Content-Type", "application/json")
	w.Header().Set("Access-Control-Allow-Origin", "*")
	w.Header().Set("Access-Control-Allow-Headers", "Content-Type, Authorization, X-API-Key")
	w.WriteHeader(status)
	_ = json.NewEncoder(w).Encode(data)
}

func (a *API) respondError(w http.ResponseWriter, status int, code, message string) {
	a.respondJSON(w, status, map[string]interface{}{
		"error":   code,
		"message": message,
	})
}

// handleHealthCheck returns service health status.
func (a *API) handleHealthCheck(w http.ResponseWriter, r *http.Request) {
	a.respondJSON(w, http.StatusOK, map[string]interface{}{
		"status":    "healthy",
		"service":   "keygate",
		"version":   "1.0.0",
		"timestamp": time.Now().UTC().Format(time.RFC3339),
	})
}

// handleVerifyKey processes out-of-band verification requests.
func (a *API) handleVerifyKey(w http.ResponseWriter, r *http.Request) {
	if r.Method == http.MethodOptions {
		w.Header().Set("Access-Control-Allow-Origin", "*")
		w.Header().Set("Access-Control-Allow-Methods", "POST, OPTIONS")
		w.Header().Set("Access-Control-Allow-Headers", "Content-Type, Authorization, X-API-Key")
		w.WriteHeader(http.StatusNoContent)
		return
	}
	if r.Method != http.MethodPost {
		a.respondError(w, http.StatusMethodNotAllowed, "method_not_allowed", "POST required")
		return
	}

	var req keys.ValidationRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		a.respondError(w, http.StatusBadRequest, "invalid_json", "Invalid JSON request body")
		return
	}

	if req.ClientIP == "" {
		req.ClientIP = proxy.ExtractClientIP(r)
	}

	start := time.Now()
	valRes := a.validator.Verify(r.Context(), req)

	if !valRes.Valid {
		a.emitEvent(r.Context(), "", req.WorkspaceID, http.StatusUnauthorized, time.Since(start), req.ClientIP, r.URL.Path, r.Method)
		a.respondJSON(w, http.StatusOK, map[string]interface{}{
			"valid":      false,
			"code":       string(valRes.Code),
			"message":    valRes.Message,
			"cached":     valRes.Cached,
			"durationUs": valRes.DurationUs,
		})
		return
	}

	apiKey := valRes.Key

	// Evaluate rate limit
	duration := time.Duration(apiKey.RateDuration) * time.Second
	if duration <= 0 {
		duration = 60 * time.Second
	}
	rateRes := a.limiter.Allow(ratelimit.LimitOptions{
		Key:      apiKey.ID,
		Limit:    apiKey.RateLimit,
		Duration: duration,
		Burst:    apiKey.Burst,
	})

	for k, v := range rateRes.FormatHeaders() {
		w.Header().Set(k, v)
	}

	if !rateRes.Allowed {
		a.emitEvent(r.Context(), apiKey.ID, apiKey.WorkspaceID, http.StatusTooManyRequests, time.Since(start), req.ClientIP, r.URL.Path, r.Method)
		a.respondJSON(w, http.StatusOK, map[string]interface{}{
			"valid":   false,
			"code":    "RATE_LIMITED",
			"message": fmt.Sprintf("Sliding window rate limit reached. Retry in %d seconds.", rateRes.RetryAfter),
			"rateLimit": map[string]interface{}{
				"tier":       apiKey.Tier,
				"limit":      apiKey.RateLimit,
				"remaining":  0,
				"reset":      rateRes.ResetSec,
				"retryAfter": rateRes.RetryAfter,
			},
			"cached":     valRes.Cached,
			"durationUs": valRes.DurationUs,
		})
		return
	}

	a.emitEvent(r.Context(), apiKey.ID, apiKey.WorkspaceID, http.StatusOK, time.Since(start), req.ClientIP, r.URL.Path, r.Method)

	go func(id string) {
		bgCtx, cancel := context.WithTimeout(context.Background(), 2*time.Second)
		defer cancel()
		_ = a.db.IncrementKeyUsage(bgCtx, id, time.Now().UTC())
	}(apiKey.ID)

	a.respondJSON(w, http.StatusOK, map[string]interface{}{
		"valid":       true,
		"code":        "VALID",
		"keyId":       apiKey.ID,
		"ownerId":     apiKey.OwnerID,
		"workspaceId": apiKey.WorkspaceID,
		"name":        apiKey.Name,
		"tier":        apiKey.Tier,
		"scopes":      apiKey.Scopes,
		"rateLimit": map[string]interface{}{
			"tier":      apiKey.Tier,
			"limit":     apiKey.RateLimit,
			"remaining": rateRes.Remaining,
			"reset":     rateRes.ResetSec,
		},
		"cached":     valRes.Cached,
		"durationUs": valRes.DurationUs,
	})
}

// CreateKeyRequest models incoming parameters to issue a new API key.
type CreateKeyRequest struct {
	Name         string            `json:"name"`
	WorkspaceID  string            `json:"workspaceId"`
	Prefix       string            `json:"prefix"`
	OwnerID      string            `json:"ownerId"`
	Tier         string            `json:"tier"`
	RateLimit    int               `json:"rateLimit"`
	RateDuration int               `json:"rateDuration"`
	Burst        int               `json:"burst"`
	Scopes       []string          `json:"scopes"`
	IPAllowlist  []string          `json:"ipAllowlist"`
	ExpiresAt    *time.Time        `json:"expiresAt"`
	Metadata     map[string]string `json:"metadata"`
}

// handleCreateKey generates and persists a new API key.
func (a *API) handleCreateKey(w http.ResponseWriter, r *http.Request) {
	if r.Method == http.MethodOptions {
		w.Header().Set("Access-Control-Allow-Origin", "*")
		w.Header().Set("Access-Control-Allow-Methods", "POST, OPTIONS")
		w.Header().Set("Access-Control-Allow-Headers", "Content-Type, Authorization, X-API-Key")
		w.WriteHeader(http.StatusNoContent)
		return
	}
	if r.Method != http.MethodPost {
		a.respondError(w, http.StatusMethodNotAllowed, "method_not_allowed", "POST required")
		return
	}

	var req CreateKeyRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		a.respondError(w, http.StatusBadRequest, "invalid_json", "Invalid JSON request body")
		return
	}

	name := strings.TrimSpace(req.Name)
	if name == "" {
		name = "Default API Key"
	}

	wsID := strings.TrimSpace(req.WorkspaceID)
	if wsID == "" {
		wsID = "ws_default"
	}

	prefix := strings.TrimSpace(req.Prefix)
	if prefix == "" {
		prefix = "sk_live"
	}

	ownerID := strings.TrimSpace(req.OwnerID)
	if ownerID == "" {
		ownerID = "default_user"
	}

	tier := strings.ToLower(strings.TrimSpace(req.Tier))
	if tier == "" {
		tier = "free"
	}

	tierConfig := ratelimit.ResolveTier(tier)
	rateLimit := req.RateLimit
	if rateLimit <= 0 {
		rateLimit = tierConfig.Limit
	}
	rateDuration := req.RateDuration
	if rateDuration <= 0 {
		rateDuration = tierConfig.Duration
	}
	burst := req.Burst
	if burst <= 0 {
		burst = tierConfig.Burst
	}

	// Generate key
	gen, err := keys.Generate(keys.KeyGenerationOptions{
		Prefix:      prefix,
		WorkspaceID: wsID,
	})
	if err != nil {
		a.respondError(w, http.StatusInternalServerError, "generation_error", err.Error())
		return
	}

	// Fetch salt
	salt, err := a.db.GetWorkspaceSalt(r.Context(), wsID)
	if err != nil {
		a.respondError(w, http.StatusInternalServerError, "salt_error", err.Error())
		return
	}

	keyHash := keys.HashKey(gen.RawKey, salt)
	now := time.Now().UTC()

	apiKey := &storage.APIKey{
		ID:           gen.KeyID,
		KeyHash:      keyHash,
		Prefix:       gen.Prefix,
		Name:         name,
		OwnerID:      ownerID,
		WorkspaceID:  wsID,
		Tier:         tier,
		RateLimit:    rateLimit,
		RateDuration: rateDuration,
		Burst:        burst,
		Scopes:       req.Scopes,
		IPAllowlist:  req.IPAllowlist,
		Metadata:     req.Metadata,
		ExpiresAt:    req.ExpiresAt,
		Enabled:      true,
		CreatedAt:    now,
		UpdatedAt:    now,
	}

	if err := a.db.CreateKey(r.Context(), apiKey); err != nil {
		a.respondError(w, http.StatusInternalServerError, "storage_error", err.Error())
		return
	}

	a.respondJSON(w, http.StatusCreated, map[string]interface{}{
		"keyId":        apiKey.ID,
		"key":          gen.RawKey, // Plaintext returned once
		"prefix":       apiKey.Prefix,
		"name":         apiKey.Name,
		"workspaceId":  apiKey.WorkspaceID,
		"ownerId":      apiKey.OwnerID,
		"tier":         apiKey.Tier,
		"rateLimit":    apiKey.RateLimit,
		"rateDuration": apiKey.RateDuration,
		"burst":        apiKey.Burst,
		"scopes":       apiKey.Scopes,
		"ipAllowlist":  apiKey.IPAllowlist,
		"expiresAt":    apiKey.ExpiresAt,
		"createdAt":    apiKey.CreatedAt,
	})
}

// handleListKeys returns paginated API keys.
func (a *API) handleListKeys(w http.ResponseWriter, r *http.Request) {
	if r.Method == http.MethodOptions {
		w.Header().Set("Access-Control-Allow-Origin", "*")
		w.Header().Set("Access-Control-Allow-Methods", "GET, OPTIONS")
		w.Header().Set("Access-Control-Allow-Headers", "Content-Type, Authorization, X-API-Key")
		w.WriteHeader(http.StatusNoContent)
		return
	}
	if r.Method != http.MethodGet {
		a.respondError(w, http.StatusMethodNotAllowed, "method_not_allowed", "GET required")
		return
	}

	q := r.URL.Query()
	wsID := q.Get("workspaceId")
	status := q.Get("status")
	search := q.Get("search")
	limit, _ := strconv.Atoi(q.Get("limit"))
	offset, _ := strconv.Atoi(q.Get("offset"))

	keysList, total, err := a.db.ListKeys(r.Context(), storage.ListKeysOptions{
		WorkspaceID: wsID,
		Status:      status,
		Search:      search,
		Limit:       limit,
		Offset:      offset,
	})
	if err != nil {
		a.respondError(w, http.StatusInternalServerError, "storage_error", err.Error())
		return
	}

	a.respondJSON(w, http.StatusOK, map[string]interface{}{
		"keys":   keysList,
		"total":  total,
		"limit":  limit,
		"offset": offset,
	})
}

// handleRevokeKey immediately disables an API key.
func (a *API) handleRevokeKey(w http.ResponseWriter, r *http.Request) {
	if r.Method == http.MethodOptions {
		w.Header().Set("Access-Control-Allow-Origin", "*")
		w.Header().Set("Access-Control-Allow-Methods", "POST, OPTIONS")
		w.Header().Set("Access-Control-Allow-Headers", "Content-Type, Authorization, X-API-Key")
		w.WriteHeader(http.StatusNoContent)
		return
	}
	if r.Method != http.MethodPost {
		a.respondError(w, http.StatusMethodNotAllowed, "method_not_allowed", "POST required")
		return
	}

	var req struct {
		KeyID string `json:"keyId"`
	}
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil || req.KeyID == "" {
		a.respondError(w, http.StatusBadRequest, "invalid_request", "KeyID required")
		return
	}

	// Invalidate cache immediately
	a.validator.InvalidateByID(req.KeyID)

	revokedAt, err := a.db.RevokeKey(r.Context(), req.KeyID)
	if err != nil {
		a.respondError(w, http.StatusNotFound, "not_found", "Key not found or already revoked")
		return
	}

	a.respondJSON(w, http.StatusOK, map[string]interface{}{
		"success":   true,
		"keyId":     req.KeyID,
		"revokedAt": revokedAt,
	})
}

// handleUpdateKey modifies an existing API key.
func (a *API) handleUpdateKey(w http.ResponseWriter, r *http.Request) {
	if r.Method == http.MethodOptions {
		w.Header().Set("Access-Control-Allow-Origin", "*")
		w.Header().Set("Access-Control-Allow-Methods", "POST, OPTIONS")
		w.Header().Set("Access-Control-Allow-Headers", "Content-Type, Authorization, X-API-Key")
		w.WriteHeader(http.StatusNoContent)
		return
	}
	if r.Method != http.MethodPost {
		a.respondError(w, http.StatusMethodNotAllowed, "method_not_allowed", "POST required")
		return
	}

	var req struct {
		KeyID        string    `json:"keyId"`
		Name         *string   `json:"name,omitempty"`
		Tier         *string   `json:"tier,omitempty"`
		RateLimit    *int      `json:"rateLimit,omitempty"`
		RateDuration *int      `json:"rateDuration,omitempty"`
		Burst        *int      `json:"burst,omitempty"`
		Scopes       *[]string `json:"scopes,omitempty"`
		IPAllowlist  *[]string `json:"ipAllowlist,omitempty"`
		Enabled      *bool     `json:"enabled,omitempty"`
	}
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil || req.KeyID == "" {
		a.respondError(w, http.StatusBadRequest, "invalid_request", "KeyID required")
		return
	}

	a.validator.InvalidateByID(req.KeyID)

	err := a.db.UpdateKey(r.Context(), req.KeyID, storage.UpdateKeyOptions{
		Name:         req.Name,
		Tier:         req.Tier,
		RateLimit:    req.RateLimit,
		RateDuration: req.RateDuration,
		Burst:        req.Burst,
		Scopes:       req.Scopes,
		IPAllowlist:  req.IPAllowlist,
		Enabled:      req.Enabled,
	})
	if err != nil {
		a.respondError(w, http.StatusInternalServerError, "update_failed", err.Error())
		return
	}

	a.respondJSON(w, http.StatusOK, map[string]interface{}{
		"success": true,
		"keyId":   req.KeyID,
	})
}

// handleAnalyticsSummary returns high-level operational telemetry.
func (a *API) handleAnalyticsSummary(w http.ResponseWriter, r *http.Request) {
	if r.Method == http.MethodOptions {
		w.Header().Set("Access-Control-Allow-Origin", "*")
		w.Header().Set("Access-Control-Allow-Methods", "GET, OPTIONS")
		w.Header().Set("Access-Control-Allow-Headers", "Content-Type, Authorization, X-API-Key")
		w.WriteHeader(http.StatusNoContent)
		return
	}
	if r.Method != http.MethodGet {
		a.respondError(w, http.StatusMethodNotAllowed, "method_not_allowed", "GET required")
		return
	}

	wsID := r.URL.Query().Get("workspaceId")
	summary, err := a.db.GetAnalyticsSummary(r.Context(), wsID)
	if err != nil {
		a.respondError(w, http.StatusInternalServerError, "analytics_error", err.Error())
		return
	}

	a.respondJSON(w, http.StatusOK, summary)
}

func (a *API) emitEvent(ctx context.Context, keyID, wsID string, status int, dur time.Duration, clientIP, path, method string) {
	if wsID == "" {
		wsID = "ws_default"
	}
	event := &storage.UsageEvent{
		KeyID:       keyID,
		WorkspaceID: wsID,
		StatusCode:  status,
		LatencyMs:   float64(dur.Microseconds()) / 1000.0,
		ClientIP:    clientIP,
		Path:        path,
		Method:      method,
		Timestamp:   time.Now().UTC(),
	}

	if a.broker != nil {
		a.broker.RecordEvent(event)
	}

	go func(ev *storage.UsageEvent) {
		bgCtx, cancel := context.WithTimeout(context.Background(), 2*time.Second)
		defer cancel()
		_ = a.db.RecordUsageEvent(bgCtx, ev)
	}(event)
}
