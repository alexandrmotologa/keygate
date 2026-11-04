package server

import (
	"bytes"
	"context"
	"encoding/csv"
	"encoding/json"
	"fmt"
	"net/http"
	"strconv"
	"strings"
	"time"

	"github.com/alexandrmotologa/keygate/internal/keys"
	"github.com/alexandrmotologa/keygate/internal/metrics"
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
	exporter  *metrics.Exporter
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
		exporter:  metrics.NewExporter(cfg.DB),
	}
}

// RegisterRoutes registers all API endpoints on the given ServeMux.
func (a *API) RegisterRoutes(mux *http.ServeMux) {
	mux.HandleFunc("/v1/keys.verifyKey", a.handleVerifyKey)
	mux.HandleFunc("/v1/keys.createKey", a.handleCreateKey)
	mux.HandleFunc("/v1/keys.listKeys", a.handleListKeys)
	mux.HandleFunc("/v1/keys.getKey", a.handleGetKey)
	mux.HandleFunc("/v1/keys.revokeKey", a.handleRevokeKey)
	mux.HandleFunc("/v1/keys.updateKey", a.handleUpdateKey)
	mux.HandleFunc("/v1/keys.rotateKey", a.handleRotateKey)
	mux.HandleFunc("/v1/keys.export", a.handleExportKeys)
	mux.HandleFunc("/v1/workspaces.list", a.handleListWorkspaces)
	mux.HandleFunc("/v1/workspaces.create", a.handleCreateWorkspace)
	mux.HandleFunc("/v1/audit.list", a.handleListAuditLogs)
	mux.HandleFunc("/v1/analytics.summary", a.handleAnalyticsSummary)
	mux.Handle("/v1/analytics.stream", a.broker)
	mux.Handle("/metrics", a.exporter)
	mux.HandleFunc("/healthz", a.handleHealthCheck)
}

func (a *API) respondJSON(w http.ResponseWriter, status int, data interface{}) {
	w.Header().Set("Content-Type", "application/json")
	w.Header().Set("Access-Control-Allow-Origin", "*")
	w.Header().Set("Access-Control-Allow-Methods", "GET, POST, OPTIONS")
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

	a.exporter.IncRequests()

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

	if valRes.Cached {
		a.exporter.IncCacheHit()
	} else {
		a.exporter.IncCacheMiss()
	}

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
		a.exporter.IncRateLimited()
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
		"maxUses":     apiKey.MaxUses,
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
	MaxUses      int               `json:"maxUses"`
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
		MaxUses:      req.MaxUses,
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

	// Audit Log
	_ = a.db.RecordAuditLog(r.Context(), &storage.AuditLog{
		WorkspaceID: wsID,
		Actor:       proxy.ExtractClientIP(r),
		Action:      "key.created",
		TargetID:    apiKey.ID,
		Details: map[string]string{
			"name": apiKey.Name,
			"tier": apiKey.Tier,
		},
		Timestamp: now,
	})

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
		"maxUses":      apiKey.MaxUses,
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

// handleGetKey returns details for a single key plus recent usage events.
func (a *API) handleGetKey(w http.ResponseWriter, r *http.Request) {
	if r.Method == http.MethodOptions {
		w.Header().Set("Access-Control-Allow-Origin", "*")
		w.Header().Set("Access-Control-Allow-Methods", "GET, OPTIONS")
		w.Header().Set("Access-Control-Allow-Headers", "Content-Type, Authorization, X-API-Key")
		w.WriteHeader(http.StatusNoContent)
		return
	}

	keyID := r.URL.Query().Get("id")
	if keyID == "" {
		a.respondError(w, http.StatusBadRequest, "invalid_request", "Key ID is required")
		return
	}

	apiKey, err := a.db.GetKeyByID(r.Context(), keyID)
	if err != nil || apiKey == nil {
		a.respondError(w, http.StatusNotFound, "not_found", "Key not found")
		return
	}

	history, _ := a.db.GetKeyUsageHistory(r.Context(), keyID, 30)
	if history == nil {
		history = []*storage.UsageEvent{}
	}

	a.respondJSON(w, http.StatusOK, map[string]interface{}{
		"key":     apiKey,
		"history": history,
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

	a.validator.InvalidateByID(req.KeyID)

	revokedAt, err := a.db.RevokeKey(r.Context(), req.KeyID)
	if err != nil {
		a.respondError(w, http.StatusNotFound, "not_found", "Key not found or already revoked")
		return
	}

	_ = a.db.RecordAuditLog(r.Context(), &storage.AuditLog{
		WorkspaceID: "ws_default",
		Actor:       proxy.ExtractClientIP(r),
		Action:      "key.revoked",
		TargetID:    req.KeyID,
		Details:     map[string]string{"keyId": req.KeyID},
		Timestamp:   *revokedAt,
	})

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
		KeyID        string     `json:"keyId"`
		Name         *string    `json:"name,omitempty"`
		Tier         *string    `json:"tier,omitempty"`
		RateLimit    *int       `json:"rateLimit,omitempty"`
		RateDuration *int       `json:"rateDuration,omitempty"`
		Burst        *int       `json:"burst,omitempty"`
		MaxUses      *int       `json:"maxUses,omitempty"`
		Scopes       *[]string  `json:"scopes,omitempty"`
		IPAllowlist  *[]string  `json:"ipAllowlist,omitempty"`
		ExpiresAt    *time.Time `json:"expiresAt,omitempty"`
		Enabled      *bool      `json:"enabled,omitempty"`
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
		MaxUses:      req.MaxUses,
		Scopes:       req.Scopes,
		IPAllowlist:  req.IPAllowlist,
		ExpiresAt:    req.ExpiresAt,
		Enabled:      req.Enabled,
	})
	if err != nil {
		a.respondError(w, http.StatusInternalServerError, "update_failed", err.Error())
		return
	}

	_ = a.db.RecordAuditLog(r.Context(), &storage.AuditLog{
		WorkspaceID: "ws_default",
		Actor:       proxy.ExtractClientIP(r),
		Action:      "key.updated",
		TargetID:    req.KeyID,
		Details:     map[string]string{"keyId": req.KeyID},
		Timestamp:   time.Now().UTC(),
	})

	a.respondJSON(w, http.StatusOK, map[string]interface{}{
		"success": true,
		"keyId":   req.KeyID,
	})
}

// handleRotateKey implements zero-downtime key rotation with grace period.
func (a *API) handleRotateKey(w http.ResponseWriter, r *http.Request) {
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
		KeyID            string `json:"keyId"`
		GracePeriodHours int    `json:"gracePeriodHours"`
	}
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil || req.KeyID == "" {
		a.respondError(w, http.StatusBadRequest, "invalid_request", "KeyID is required")
		return
	}

	if req.GracePeriodHours <= 0 {
		req.GracePeriodHours = 24 // default 24h grace window
	}

	oldKey, err := a.db.GetKeyByID(r.Context(), req.KeyID)
	if err != nil || oldKey == nil {
		a.respondError(w, http.StatusNotFound, "not_found", "Key not found")
		return
	}

	// 1. Generate new replacement key
	gen, err := keys.Generate(keys.KeyGenerationOptions{
		Prefix:      oldKey.Prefix,
		WorkspaceID: oldKey.WorkspaceID,
	})
	if err != nil {
		a.respondError(w, http.StatusInternalServerError, "generation_error", err.Error())
		return
	}

	salt, _ := a.db.GetWorkspaceSalt(r.Context(), oldKey.WorkspaceID)
	keyHash := keys.HashKey(gen.RawKey, salt)
	now := time.Now().UTC()

	newKey := &storage.APIKey{
		ID:           gen.KeyID,
		KeyHash:      keyHash,
		Prefix:       gen.Prefix,
		Name:         oldKey.Name + " (Rotated)",
		OwnerID:      oldKey.OwnerID,
		WorkspaceID:  oldKey.WorkspaceID,
		Tier:         oldKey.Tier,
		RateLimit:    oldKey.RateLimit,
		RateDuration: oldKey.RateDuration,
		Burst:        oldKey.Burst,
		MaxUses:      oldKey.MaxUses,
		Scopes:       oldKey.Scopes,
		IPAllowlist:  oldKey.IPAllowlist,
		Metadata:     oldKey.Metadata,
		Enabled:      true,
		CreatedAt:    now,
		UpdatedAt:    now,
	}

	if err := a.db.CreateKey(r.Context(), newKey); err != nil {
		a.respondError(w, http.StatusInternalServerError, "storage_error", err.Error())
		return
	}

	// 2. Schedule old key expiry after grace period
	graceExpiry := now.Add(time.Duration(req.GracePeriodHours) * time.Hour)
	_ = a.db.UpdateKey(r.Context(), oldKey.ID, storage.UpdateKeyOptions{
		ExpiresAt: &graceExpiry,
	})
	a.validator.InvalidateByID(oldKey.ID)

	// 3. Audit log
	_ = a.db.RecordAuditLog(r.Context(), &storage.AuditLog{
		WorkspaceID: oldKey.WorkspaceID,
		Actor:       proxy.ExtractClientIP(r),
		Action:      "key.rotated",
		TargetID:    oldKey.ID,
		Details: map[string]string{
			"oldKeyId":         oldKey.ID,
			"newKeyId":         newKey.ID,
			"gracePeriodHours": strconv.Itoa(req.GracePeriodHours),
			"oldExpiresAt":     graceExpiry.Format(time.RFC3339),
		},
		Timestamp: now,
	})

	a.respondJSON(w, http.StatusOK, map[string]interface{}{
		"success":          true,
		"oldKeyId":         oldKey.ID,
		"newKeyId":         newKey.ID,
		"newKey":           gen.RawKey,
		"gracePeriodHours": req.GracePeriodHours,
		"oldKeyExpiresAt":  graceExpiry,
	})
}

// handleExportKeys outputs CSV or JSON dumps of workspace keys.
func (a *API) handleExportKeys(w http.ResponseWriter, r *http.Request) {
	wsID := r.URL.Query().Get("workspaceId")
	format := strings.ToLower(r.URL.Query().Get("format"))

	keysList, _, err := a.db.ListKeys(r.Context(), storage.ListKeysOptions{
		WorkspaceID: wsID,
		Limit:       500,
	})
	if err != nil {
		a.respondError(w, http.StatusInternalServerError, "export_error", err.Error())
		return
	}

	if format == "csv" {
		w.Header().Set("Content-Type", "text/csv")
		w.Header().Set("Content-Disposition", "attachment; filename=\"keygate-keys.csv\"")

		var buf bytes.Buffer
		writer := csv.NewWriter(&buf)
		_ = writer.Write([]string{"ID", "Name", "Prefix", "Workspace", "Tier", "RateLimit", "MaxUses", "Requests", "Status", "CreatedAt"})

		for _, k := range keysList {
			status := "ACTIVE"
			if k.RevokedAt != nil {
				status = "REVOKED"
			} else if !k.Enabled {
				status = "DISABLED"
			}
			_ = writer.Write([]string{
				k.ID, k.Name, k.Prefix, k.WorkspaceID, k.Tier,
				strconv.Itoa(k.RateLimit), strconv.Itoa(k.MaxUses),
				strconv.FormatInt(k.TotalRequests, 10), status, k.CreatedAt.Format(time.RFC3339),
			})
		}
		writer.Flush()
		_, _ = w.Write(buf.Bytes())
		return
	}

	w.Header().Set("Content-Type", "application/json")
	w.Header().Set("Content-Disposition", "attachment; filename=\"keygate-keys.json\"")
	_ = json.NewEncoder(w).Encode(keysList)
}

// handleListWorkspaces returns all configured workspaces.
func (a *API) handleListWorkspaces(w http.ResponseWriter, r *http.Request) {
	workspaces, err := a.db.ListWorkspaces(r.Context())
	if err != nil {
		a.respondError(w, http.StatusInternalServerError, "db_error", err.Error())
		return
	}
	a.respondJSON(w, http.StatusOK, map[string]interface{}{
		"workspaces": workspaces,
	})
}

// handleCreateWorkspace registers a new tenant environment.
func (a *API) handleCreateWorkspace(w http.ResponseWriter, r *http.Request) {
	var req struct {
		ID   string `json:"id"`
		Name string `json:"name"`
	}
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil || req.ID == "" {
		a.respondError(w, http.StatusBadRequest, "invalid_request", "Workspace ID is required")
		return
	}

	ws, err := a.db.CreateWorkspace(r.Context(), req.ID, req.Name)
	if err != nil {
		a.respondError(w, http.StatusInternalServerError, "creation_failed", err.Error())
		return
	}

	_ = a.db.RecordAuditLog(r.Context(), &storage.AuditLog{
		WorkspaceID: ws.ID,
		Actor:       proxy.ExtractClientIP(r),
		Action:      "workspace.created",
		TargetID:    ws.ID,
		Details:     map[string]string{"name": ws.Name},
		Timestamp:   time.Now().UTC(),
	})

	a.respondJSON(w, http.StatusCreated, ws)
}

// handleListAuditLogs returns administrative audit trail.
func (a *API) handleListAuditLogs(w http.ResponseWriter, r *http.Request) {
	wsID := r.URL.Query().Get("workspaceId")
	limit, _ := strconv.Atoi(r.URL.Query().Get("limit"))
	offset, _ := strconv.Atoi(r.URL.Query().Get("offset"))

	logs, total, err := a.db.ListAuditLogs(r.Context(), wsID, limit, offset)
	if err != nil {
		a.respondError(w, http.StatusInternalServerError, "audit_error", err.Error())
		return
	}

	a.respondJSON(w, http.StatusOK, map[string]interface{}{
		"logs":   logs,
		"total":  total,
		"limit":  limit,
		"offset": offset,
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
