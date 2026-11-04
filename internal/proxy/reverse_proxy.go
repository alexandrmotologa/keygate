package proxy

import (
	"context"
	"encoding/json"
	"fmt"
	"net"
	"net/http"
	"net/http/httputil"
	"net/url"
	"strings"
	"time"

	"github.com/alexandrmotologa/keygate/internal/keys"
	"github.com/alexandrmotologa/keygate/internal/ratelimit"
	"github.com/alexandrmotologa/keygate/internal/storage"
)

// TelemetrySink receives live request execution telemetry.
type TelemetrySink interface {
	RecordEvent(event *storage.UsageEvent)
}

// Proxy routes incoming client traffic through authentication and rate limiting to upstream targets.
type Proxy struct {
	targetURL    *url.URL
	reverseProxy *httputil.ReverseProxy
	validator    *keys.Validator
	limiter      *ratelimit.Limiter
	db           *storage.DB
	sink         TelemetrySink
}

// ProxyConfig holds construction parameters.
type ProxyConfig struct {
	TargetURL string
	Validator *keys.Validator
	Limiter   *ratelimit.Limiter
	DB        *storage.DB
	Sink      TelemetrySink
}

// NewProxy creates and initializes the reverse proxy handler.
func NewProxy(cfg ProxyConfig) (*Proxy, error) {
	parsedURL, err := url.Parse(cfg.TargetURL)
	if err != nil {
		return nil, fmt.Errorf("invalid proxy target URL: %w", err)
	}

	rp := httputil.NewSingleHostReverseProxy(parsedURL)

	// Customize director to retain original host or set target host
	originalDirector := rp.Director
	rp.Director = func(req *http.Request) {
		originalDirector(req)
		req.Host = parsedURL.Host
	}

	p := &Proxy{
		targetURL:    parsedURL,
		reverseProxy: rp,
		validator:    cfg.Validator,
		limiter:      cfg.Limiter,
		db:           cfg.DB,
		sink:         cfg.Sink,
	}

	return p, nil
}

// ExtractClientIP retrieves client IP from RemoteAddr or proxy headers.
func ExtractClientIP(r *http.Request) string {
	if xff := r.Header.Get("X-Forwarded-For"); xff != "" {
		parts := strings.Split(xff, ",")
		return strings.TrimSpace(parts[0])
	}
	if xri := r.Header.Get("X-Real-IP"); xri != "" {
		return strings.TrimSpace(xri)
	}
	ip, _, err := net.SplitHostPort(r.RemoteAddr)
	if err == nil {
		return ip
	}
	return r.RemoteAddr
}

// ExtractAPIKey extracts the raw key from Authorization Bearer or X-API-Key header.
func ExtractAPIKey(r *http.Request) string {
	auth := r.Header.Get("Authorization")
	if auth != "" {
		parts := strings.SplitN(auth, " ", 2)
		if len(parts) == 2 && strings.EqualFold(parts[0], "bearer") {
			return strings.TrimSpace(parts[1])
		}
	}

	if xKey := r.Header.Get("X-API-Key"); xKey != "" {
		return strings.TrimSpace(xKey)
	}

	return ""
}

// ServeHTTP inspects credentials, throttles, and forwards requests.
func (p *Proxy) ServeHTTP(w http.ResponseWriter, r *http.Request) {
	start := time.Now()
	clientIP := ExtractClientIP(r)
	rawKey := ExtractAPIKey(r)

	// 1. Missing credentials
	if rawKey == "" {
		p.respondError(w, http.StatusUnauthorized, "unauthorized", "Missing API key in Authorization (Bearer) or X-API-Key header.")
		p.emitEvent("", "", http.StatusUnauthorized, time.Since(start), clientIP, r.URL.Path, r.Method)
		return
	}

	// 2. Validate key
	ctx := r.Context()
	valRes := p.validator.Verify(ctx, keys.ValidationRequest{
		RawKey:   rawKey,
		ClientIP: clientIP,
	})

	if !valRes.Valid {
		status := http.StatusUnauthorized
		if valRes.Code == keys.CodeIPForbidden || valRes.Code == keys.CodeInsufficientScopes {
			status = http.StatusForbidden
		}
		p.respondError(w, status, string(valRes.Code), valRes.Message)
		keyID := ""
		wsID := ""
		if valRes.Key != nil {
			keyID = valRes.Key.ID
			wsID = valRes.Key.WorkspaceID
		}
		p.emitEvent(keyID, wsID, status, time.Since(start), clientIP, r.URL.Path, r.Method)
		return
	}

	apiKey := valRes.Key

	// 3. Rate Limiting Check
	duration := time.Duration(apiKey.RateDuration) * time.Second
	if duration <= 0 {
		duration = 60 * time.Second
	}
	rateRes := p.limiter.Allow(ratelimit.LimitOptions{
		Key:      apiKey.ID,
		Limit:    apiKey.RateLimit,
		Duration: duration,
		Burst:    apiKey.Burst,
	})

	// Inject IETF headers onto response
	for k, v := range rateRes.FormatHeaders() {
		w.Header().Set(k, v)
	}

	if !rateRes.Allowed {
		p.respondError(w, http.StatusTooManyRequests, "rate_limit_exceeded",
			fmt.Sprintf("Rate limit of %d requests exceeded. Retry after %d seconds.", apiKey.RateLimit, rateRes.RetryAfter))
		p.emitEvent(apiKey.ID, apiKey.WorkspaceID, http.StatusTooManyRequests, time.Since(start), clientIP, r.URL.Path, r.Method)
		return
	}

	// 4. Strip client credentials and inject trusted upstream identity headers
	r.Header.Del("Authorization")
	r.Header.Del("X-API-Key")

	r.Header.Set("X-KeyGate-Key-Id", apiKey.ID)
	r.Header.Set("X-KeyGate-Owner-Id", apiKey.OwnerID)
	r.Header.Set("X-KeyGate-Workspace", apiKey.WorkspaceID)
	r.Header.Set("X-KeyGate-Scopes", strings.Join(apiKey.Scopes, ","))
	r.Header.Set("X-KeyGate-Tier", apiKey.Tier)

	// Forward client IP
	if r.Header.Get("X-Forwarded-For") == "" {
		r.Header.Set("X-Forwarded-For", clientIP)
	}

	// 5. Wrap response writer to capture upstream status
	rw := &responseCaptureWriter{ResponseWriter: w, statusCode: http.StatusOK}
	p.reverseProxy.ServeHTTP(rw, r)

	// 6. Record usage metrics
	latency := time.Since(start)
	p.emitEvent(apiKey.ID, apiKey.WorkspaceID, rw.statusCode, latency, clientIP, r.URL.Path, r.Method)
	p.validator.RecordSuccessUsage(r.Context(), apiKey)
}

func (p *Proxy) emitEvent(keyID, workspaceID string, statusCode int, duration time.Duration, clientIP, path, method string) {
	if workspaceID == "" {
		workspaceID = "ws_default"
	}
	latencyMs := float64(duration.Microseconds()) / 1000.0
	event := &storage.UsageEvent{
		KeyID:       keyID,
		WorkspaceID: workspaceID,
		StatusCode:  statusCode,
		LatencyMs:   latencyMs,
		ClientIP:    clientIP,
		Path:        path,
		Method:      method,
		Timestamp:   time.Now().UTC(),
	}

	if p.sink != nil {
		p.sink.RecordEvent(event)
	}

	// Async insert to SQLite
	go func(ev *storage.UsageEvent) {
		bgCtx, cancel := context.WithTimeout(context.Background(), 3*time.Second)
		defer cancel()
		_ = p.db.RecordUsageEvent(bgCtx, ev)
	}(event)
}

func (p *Proxy) respondError(w http.ResponseWriter, status int, code, message string) {
	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(status)
	_ = json.NewEncoder(w).Encode(map[string]interface{}{
		"error":   code,
		"message": message,
	})
}

type responseCaptureWriter struct {
	http.ResponseWriter
	statusCode int
}

func (w *responseCaptureWriter) WriteHeader(code int) {
	w.statusCode = code
	w.ResponseWriter.WriteHeader(code)
}
