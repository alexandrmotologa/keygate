package proxy

import (
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"testing"
	"time"

	"github.com/alexandrmotologa/keygate/internal/keys"
	"github.com/alexandrmotologa/keygate/internal/ratelimit"
	"github.com/alexandrmotologa/keygate/internal/storage"
)

type mockSink struct {
	events []*storage.UsageEvent
}

func (m *mockSink) RecordEvent(event *storage.UsageEvent) {
	m.events = append(m.events, event)
}

func setupProxyTest(t *testing.T) (*Proxy, *keys.Validator, *storage.DB, *MockUpstream, *mockSink, func()) {
	tempDir, err := os.MkdirTemp("", "keygate-proxy-*")
	if err != nil {
		t.Fatalf("failed to create temp dir: %v", err)
	}

	dbPath := filepath.Join(tempDir, "proxy_test.db")
	db, err := storage.Open(dbPath)
	if err != nil {
		os.RemoveAll(tempDir)
		t.Fatalf("failed to open storage: %v", err)
	}

	validator, err := keys.NewValidator(db)
	if err != nil {
		db.Close()
		os.RemoveAll(tempDir)
		t.Fatalf("failed to create validator: %v", err)
	}

	limiter := ratelimit.NewLimiter()
	sink := &mockSink{}

	// Start real mock upstream
	mockUpstream, err := StartMockUpstream("")
	if err != nil {
		validator.Close()
		limiter.Close()
		db.Close()
		os.RemoveAll(tempDir)
		t.Fatalf("failed to start mock upstream: %v", err)
	}

	p, err := NewProxy(ProxyConfig{
		TargetURL: mockUpstream.Addr,
		Validator: validator,
		Limiter:   limiter,
		DB:        db,
		Sink:      sink,
	})
	if err != nil {
		mockUpstream.Close()
		validator.Close()
		limiter.Close()
		db.Close()
		os.RemoveAll(tempDir)
		t.Fatalf("failed to create proxy: %v", err)
	}

	cleanup := func() {
		mockUpstream.Close()
		validator.Close()
		limiter.Close()
		db.Close()
		os.RemoveAll(tempDir)
	}

	return p, validator, db, mockUpstream, sink, cleanup
}

func TestProxyPipeline(t *testing.T) {
	proxy, _, db, _, sink, cleanup := setupProxyTest(t)
	defer cleanup()

	ctx := context.Background()

	// 1. Create a valid test key with a limit of 2 requests
	gen, _ := keys.Generate(keys.KeyGenerationOptions{
		Prefix:      "sk_live",
		WorkspaceID: "ws_default",
	})
	salt, _ := db.GetWorkspaceSalt(ctx, "ws_default")
	keyHash := keys.HashKey(gen.RawKey, salt)

	_ = db.CreateKey(ctx, &storage.APIKey{
		ID:           gen.KeyID,
		KeyHash:      keyHash,
		Prefix:       gen.Prefix,
		Name:         "Proxy Test Key",
		OwnerID:      "usr_proxy",
		WorkspaceID:  "ws_default",
		Tier:         "free",
		RateLimit:    2,
		RateDuration: 60,
		Burst:        0,
		Scopes:       []string{"read:items"},
		Enabled:      true,
		CreatedAt:    time.Now().UTC(),
		UpdatedAt:    time.Now().UTC(),
	})

	// Case 1: Missing credentials -> 401
	req1 := httptest.NewRequest("GET", "/test/resource", nil)
	rec1 := httptest.NewRecorder()
	proxy.ServeHTTP(rec1, req1)

	if rec1.Code != http.StatusUnauthorized {
		t.Errorf("expected 401 for missing key, got %d", rec1.Code)
	}

	// Case 2: First valid request -> 200 forwarded upstream
	req2 := httptest.NewRequest("GET", "/api/data?q=search", nil)
	req2.Header.Set("Authorization", "Bearer "+gen.RawKey)
	rec2 := httptest.NewRecorder()
	proxy.ServeHTTP(rec2, req2)

	if rec2.Code != http.StatusOK {
		t.Fatalf("expected 200 for valid key, got %d, body: %s", rec2.Code, rec2.Body.String())
	}

	// Verify IETF RateLimit headers
	if rec2.Header().Get("RateLimit-Limit") != "2" {
		t.Errorf("expected RateLimit-Limit=2, got %s", rec2.Header().Get("RateLimit-Limit"))
	}
	if rec2.Header().Get("RateLimit-Remaining") != "1" {
		t.Errorf("expected RateLimit-Remaining=1, got %s", rec2.Header().Get("RateLimit-Remaining"))
	}

	// Parse upstream echo body to verify injected headers and stripped Authorization
	var echoBody struct {
		Status          string            `json:"status"`
		ReceivedHeaders map[string]string `json:"receivedHeaders"`
	}
	if err := json.Unmarshal(rec2.Body.Bytes(), &echoBody); err != nil {
		t.Fatalf("failed to parse upstream echo json: %v", err)
	}
	if echoBody.ReceivedHeaders["X-Keygate-Key-Id"] != gen.KeyID {
		t.Errorf("expected X-Keygate-Key-Id=%s, got %s", gen.KeyID, echoBody.ReceivedHeaders["X-Keygate-Key-Id"])
	}
	if echoBody.ReceivedHeaders["X-Keygate-Owner-Id"] != "usr_proxy" {
		t.Errorf("expected X-Keygate-Owner-Id=usr_proxy, got %s", echoBody.ReceivedHeaders["X-Keygate-Owner-Id"])
	}
	if _, hasAuth := echoBody.ReceivedHeaders["Authorization"]; hasAuth {
		t.Errorf("upstream should NOT receive raw Authorization header (must be stripped)")
	}

	// Case 3: Second valid request -> 200 (Remaining: 0)
	req3 := httptest.NewRequest("GET", "/api/data", nil)
	req3.Header.Set("X-API-Key", gen.RawKey)
	rec3 := httptest.NewRecorder()
	proxy.ServeHTTP(rec3, req3)

	if rec3.Code != http.StatusOK {
		t.Fatalf("expected 200 for second request, got %d", rec3.Code)
	}
	if rec3.Header().Get("RateLimit-Remaining") != "0" {
		t.Errorf("expected RateLimit-Remaining=0, got %s", rec3.Header().Get("RateLimit-Remaining"))
	}

	// Case 4: Third request -> 429 Too Many Requests
	req4 := httptest.NewRequest("GET", "/api/data", nil)
	req4.Header.Set("Authorization", "Bearer "+gen.RawKey)
	rec4 := httptest.NewRecorder()
	proxy.ServeHTTP(rec4, req4)

	if rec4.Code != http.StatusTooManyRequests {
		t.Errorf("expected 429 for rate limited request, got %d", rec4.Code)
	}
	if rec4.Header().Get("Retry-After") == "" {
		t.Errorf("expected Retry-After header on 429 response")
	}

	// Verify telemetry sink captured events
	if len(sink.events) < 4 {
		t.Errorf("expected at least 4 telemetry events captured, got %d", len(sink.events))
	}
}
