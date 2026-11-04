package server

import (
	"bytes"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/alexandrmotologa/keygate/internal/keys"
	"github.com/alexandrmotologa/keygate/internal/ratelimit"
	"github.com/alexandrmotologa/keygate/internal/storage"
)

func setupTestServer(t *testing.T) (*API, *storage.DB, *keys.Validator, *http.ServeMux, func()) {
	tempDir, err := os.MkdirTemp("", "keygate-srv-*")
	if err != nil {
		t.Fatalf("failed to create temp dir: %v", err)
	}

	dbPath := filepath.Join(tempDir, "test_srv.db")
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
	broker := NewEventBroker()

	api := NewAPI(APIConfig{
		DB:        db,
		Validator: validator,
		Limiter:   limiter,
		Broker:    broker,
	})

	mux := http.NewServeMux()
	api.RegisterRoutes(mux)
	_ = RegisterStaticFiles(mux)

	cleanup := func() {
		validator.Close()
		limiter.Close()
		db.Close()
		os.RemoveAll(tempDir)
	}

	return api, db, validator, mux, cleanup
}

func TestServerAPI(t *testing.T) {
	_, _, _, mux, cleanup := setupTestServer(t)
	defer cleanup()

	// 1. Healthcheck
	reqHealth := httptest.NewRequest("GET", "/healthz", nil)
	recHealth := httptest.NewRecorder()
	mux.ServeHTTP(recHealth, reqHealth)

	if recHealth.Code != http.StatusOK {
		t.Errorf("expected 200 for /healthz, got %d", recHealth.Code)
	}

	// 2. Create Key with MaxUses
	createPayload := map[string]interface{}{
		"name":        "Test API Key",
		"workspaceId": "ws_default",
		"prefix":      "sk_live",
		"ownerId":     "usr_test",
		"tier":        "pro",
		"rateLimit":   100,
		"maxUses":     10,
		"scopes":      []string{"read:users", "write:users"},
	}
	body, _ := json.Marshal(createPayload)
	reqCreate := httptest.NewRequest("POST", "/v1/keys.createKey", bytes.NewReader(body))
	recCreate := httptest.NewRecorder()
	mux.ServeHTTP(recCreate, reqCreate)

	if recCreate.Code != http.StatusCreated {
		t.Fatalf("expected 201 for createKey, got %d, body: %s", recCreate.Code, recCreate.Body.String())
	}

	var createRes struct {
		KeyID   string `json:"keyId"`
		Key     string `json:"key"`
		Name    string `json:"name"`
		MaxUses int    `json:"maxUses"`
	}
	_ = json.Unmarshal(recCreate.Body.Bytes(), &createRes)
	if !strings.HasPrefix(createRes.Key, "sk_live_ws_default_") || createRes.MaxUses != 10 {
		t.Errorf("unexpected create key response: %+v", createRes)
	}

	// 3. Verify Key
	verifyPayload := map[string]interface{}{
		"key":            createRes.Key,
		"workspaceId":    "ws_default",
		"requiredScopes": []string{"read:users"},
	}
	vBody, _ := json.Marshal(verifyPayload)
	reqVerify := httptest.NewRequest("POST", "/v1/keys.verifyKey", bytes.NewReader(vBody))
	recVerify := httptest.NewRecorder()
	mux.ServeHTTP(recVerify, reqVerify)

	if recVerify.Code != http.StatusOK {
		t.Fatalf("expected 200 for verifyKey, got %d", recVerify.Code)
	}

	// 4. Get Single Key Details
	reqGet := httptest.NewRequest("GET", "/v1/keys.getKey?id="+createRes.KeyID, nil)
	recGet := httptest.NewRecorder()
	mux.ServeHTTP(recGet, reqGet)

	if recGet.Code != http.StatusOK {
		t.Fatalf("expected 200 for getKey, got %d", recGet.Code)
	}

	// 5. Rotate Key
	rotatePayload := map[string]interface{}{
		"keyId":            createRes.KeyID,
		"gracePeriodHours": 48,
	}
	rotBody, _ := json.Marshal(rotatePayload)
	reqRotate := httptest.NewRequest("POST", "/v1/keys.rotateKey", bytes.NewReader(rotBody))
	recRotate := httptest.NewRecorder()
	mux.ServeHTTP(recRotate, reqRotate)

	if recRotate.Code != http.StatusOK {
		t.Fatalf("expected 200 for rotateKey, got %d, body: %s", recRotate.Code, recRotate.Body.String())
	}

	// 6. Workspaces API
	wsCreatePayload := map[string]interface{}{
		"id":   "ws_staging",
		"name": "Staging Environment",
	}
	wsBody, _ := json.Marshal(wsCreatePayload)
	reqWsCreate := httptest.NewRequest("POST", "/v1/workspaces.create", bytes.NewReader(wsBody))
	recWsCreate := httptest.NewRecorder()
	mux.ServeHTTP(recWsCreate, reqWsCreate)

	if recWsCreate.Code != http.StatusCreated {
		t.Fatalf("expected 201 for workspace create, got %d", recWsCreate.Code)
	}

	reqWsList := httptest.NewRequest("GET", "/v1/workspaces.list", nil)
	recWsList := httptest.NewRecorder()
	mux.ServeHTTP(recWsList, reqWsList)

	if recWsList.Code != http.StatusOK {
		t.Fatalf("expected 200 for workspace list, got %d", recWsList.Code)
	}

	// 7. Audit Logs API
	reqAudit := httptest.NewRequest("GET", "/v1/audit.list", nil)
	recAudit := httptest.NewRecorder()
	mux.ServeHTTP(recAudit, reqAudit)

	if recAudit.Code != http.StatusOK {
		t.Fatalf("expected 200 for audit list, got %d", recAudit.Code)
	}

	// 8. Prometheus /metrics
	reqMetrics := httptest.NewRequest("GET", "/metrics", nil)
	recMetrics := httptest.NewRecorder()
	mux.ServeHTTP(recMetrics, reqMetrics)

	if recMetrics.Code != http.StatusOK {
		t.Fatalf("expected 200 for /metrics, got %d", recMetrics.Code)
	}
	if !strings.Contains(recMetrics.Body.String(), "keygate_uptime_seconds") {
		t.Errorf("expected Prometheus metrics output in /metrics")
	}

	// 9. Export CSV
	reqExportCSV := httptest.NewRequest("GET", "/v1/keys.export?format=csv", nil)
	recExportCSV := httptest.NewRecorder()
	mux.ServeHTTP(recExportCSV, reqExportCSV)

	if recExportCSV.Code != http.StatusOK || !strings.Contains(recExportCSV.Body.String(), "RateLimit") {
		t.Errorf("expected CSV export response, got code %d", recExportCSV.Code)
	}
}
