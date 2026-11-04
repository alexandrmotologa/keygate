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

	// 2. Create Key
	createPayload := map[string]interface{}{
		"name":        "Test API Key",
		"workspaceId": "ws_default",
		"prefix":      "sk_live",
		"ownerId":     "usr_test",
		"tier":        "pro",
		"rateLimit":   100,
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
		KeyID string `json:"keyId"`
		Key   string `json:"key"`
		Name  string `json:"name"`
	}
	_ = json.Unmarshal(recCreate.Body.Bytes(), &createRes)
	if !strings.HasPrefix(createRes.Key, "sk_live_ws_default_") {
		t.Errorf("unexpected key format: %s", createRes.Key)
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

	var verifyRes struct {
		Valid     bool   `json:"valid"`
		Code      string `json:"code"`
		KeyID     string `json:"keyId"`
		RateLimit struct {
			Remaining int `json:"remaining"`
		} `json:"rateLimit"`
	}
	_ = json.Unmarshal(recVerify.Body.Bytes(), &verifyRes)
	if !verifyRes.Valid || verifyRes.Code != "VALID" || verifyRes.KeyID != createRes.KeyID {
		t.Errorf("verifyKey failed: %+v", verifyRes)
	}

	// 4. List Keys
	reqList := httptest.NewRequest("GET", "/v1/keys.listKeys?workspaceId=ws_default", nil)
	recList := httptest.NewRecorder()
	mux.ServeHTTP(recList, reqList)

	if recList.Code != http.StatusOK {
		t.Fatalf("expected 200 for listKeys, got %d", recList.Code)
	}
	var listRes struct {
		Total int `json:"total"`
	}
	_ = json.Unmarshal(recList.Body.Bytes(), &listRes)
	if listRes.Total != 1 {
		t.Errorf("expected 1 key in list, got %d", listRes.Total)
	}

	// 5. Revoke Key
	revokePayload := map[string]interface{}{
		"keyId": createRes.KeyID,
	}
	rBody, _ := json.Marshal(revokePayload)
	reqRevoke := httptest.NewRequest("POST", "/v1/keys.revokeKey", bytes.NewReader(rBody))
	recRevoke := httptest.NewRecorder()
	mux.ServeHTTP(recRevoke, reqRevoke)

	if recRevoke.Code != http.StatusOK {
		t.Fatalf("expected 200 for revokeKey, got %d", recRevoke.Code)
	}

	// 6. Verify again -> Should be invalid KEY_REVOKED
	reqVerify2 := httptest.NewRequest("POST", "/v1/keys.verifyKey", bytes.NewReader(vBody))
	recVerify2 := httptest.NewRecorder()
	mux.ServeHTTP(recVerify2, reqVerify2)

	var verifyRes2 struct {
		Valid bool   `json:"valid"`
		Code  string `json:"code"`
	}
	_ = json.Unmarshal(recVerify2.Body.Bytes(), &verifyRes2)
	if verifyRes2.Valid || verifyRes2.Code != "KEY_REVOKED" {
		t.Errorf("expected KEY_REVOKED, got: %+v", verifyRes2)
	}

	// 7. Embedded SPA Root
	reqRoot := httptest.NewRequest("GET", "/", nil)
	recRoot := httptest.NewRecorder()
	mux.ServeHTTP(recRoot, reqRoot)

	if recRoot.Code != http.StatusOK {
		t.Errorf("expected 200 for SPA root /, got %d", recRoot.Code)
	}
	if !strings.Contains(recRoot.Body.String(), "<div id=\"root\">") {
		t.Errorf("expected SPA index.html content in response")
	}
}
