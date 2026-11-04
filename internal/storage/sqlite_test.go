package storage

import (
	"context"
	"os"
	"path/filepath"
	"testing"
	"time"
)

func TestSQLiteStorage(t *testing.T) {
	tempDir, err := os.MkdirTemp("", "keygate-test-*")
	if err != nil {
		t.Fatalf("failed to create temp dir: %v", err)
	}
	defer os.RemoveAll(tempDir)

	dbPath := filepath.Join(tempDir, "test.db")
	db, err := Open(dbPath)
	if err != nil {
		t.Fatalf("failed to open test sqlite db: %v", err)
	}
	defer db.Close()

	ctx := context.Background()

	// Test 1: Workspace Salt creation
	salt, err := db.GetWorkspaceSalt(ctx, "ws_default")
	if err != nil {
		t.Fatalf("failed to get default workspace salt: %v", err)
	}
	if len(salt) == 0 {
		t.Errorf("expected non-empty salt")
	}

	// Test 2: Insert API Key
	now := time.Now().UTC()
	key := &APIKey{
		ID:           "key_test_01",
		KeyHash:      "hash_1234567890abcdef",
		Prefix:       "sk_live",
		Name:         "Integration Test Key",
		OwnerID:      "usr_test",
		WorkspaceID:  "ws_default",
		Tier:         "pro",
		RateLimit:    500,
		RateDuration: 60,
		Burst:        20,
		MaxUses:      50,
		Scopes:       []string{"read:users", "write:keys"},
		IPAllowlist:  []string{"127.0.0.1"},
		Metadata:     map[string]string{"env": "testing"},
		Enabled:      true,
		CreatedAt:    now,
		UpdatedAt:    now,
	}

	if err := db.CreateKey(ctx, key); err != nil {
		t.Fatalf("failed to create key: %v", err)
	}

	// Test 3: Get Key By Hash
	fetched, err := db.GetKeyByHash(ctx, "hash_1234567890abcdef")
	if err != nil {
		t.Fatalf("failed to get key by hash: %v", err)
	}
	if fetched == nil {
		t.Fatalf("expected key to be found")
	}
	if fetched.ID != "key_test_01" || fetched.Tier != "pro" || len(fetched.Scopes) != 2 {
		t.Errorf("fetched key mismatch: %+v", fetched)
	}

	// Test 4: List Keys
	keys, total, err := db.ListKeys(ctx, ListKeysOptions{
		WorkspaceID: "ws_default",
		Status:      "active",
	})
	if err != nil {
		t.Fatalf("failed to list keys: %v", err)
	}
	if total != 1 || len(keys) != 1 {
		t.Errorf("expected 1 active key, got total=%d, len=%d", total, len(keys))
	}

	// Test 5: Update Key
	newName := "Updated Test Key"
	newLimit := 1000
	err = db.UpdateKey(ctx, "key_test_01", UpdateKeyOptions{
		Name:      &newName,
		RateLimit: &newLimit,
	})
	if err != nil {
		t.Fatalf("failed to update key: %v", err)
	}

	updated, err := db.GetKeyByID(ctx, "key_test_01")
	if err != nil || updated == nil {
		t.Fatalf("failed to fetch updated key: %v", err)
	}
	if updated.Name != newName || updated.RateLimit != 1000 {
		t.Errorf("updated fields mismatch: name=%s, limit=%d", updated.Name, updated.RateLimit)
	}

	// Test 6: Record Usage Events
	err = db.RecordUsageEvent(ctx, &UsageEvent{
		KeyID:       "key_test_01",
		WorkspaceID: "ws_default",
		StatusCode:  200,
		LatencyMs:   0.45,
		ClientIP:    "127.0.0.1",
		Path:        "/api/v1/test",
		Method:      "GET",
		Timestamp:   time.Now().UTC(),
	})
	if err != nil {
		t.Fatalf("failed to record usage event: %v", err)
	}

	// Test 7: Analytics Summary
	summary, err := db.GetAnalyticsSummary(ctx, "ws_default")
	if err != nil {
		t.Fatalf("failed to get analytics summary: %v", err)
	}
	if summary.TotalKeys != 1 || summary.TotalRequests != 1 || summary.SuccessRequests != 1 {
		t.Errorf("unexpected summary data: %+v", summary)
	}

	// Test 8: Revoke Key
	revokedAt, err := db.RevokeKey(ctx, "key_test_01")
	if err != nil {
		t.Fatalf("failed to revoke key: %v", err)
	}
	if revokedAt == nil {
		t.Errorf("expected revokedAt timestamp")
	}

	revokedKey, err := db.GetKeyByID(ctx, "key_test_01")
	if err != nil || revokedKey == nil {
		t.Fatalf("failed to fetch revoked key: %v", err)
	}
	if revokedKey.Enabled || revokedKey.RevokedAt == nil {
		t.Errorf("expected key to be revoked: enabled=%v, revokedAt=%v", revokedKey.Enabled, revokedKey.RevokedAt)
	}

	// Test 9: Workspace and Audit Logs
	ws, err := db.CreateWorkspace(ctx, "ws_beta", "Beta Workspace")
	if err != nil || ws.ID != "ws_beta" {
		t.Fatalf("failed to create workspace: %v", err)
	}

	workspaces, err := db.ListWorkspaces(ctx)
	if err != nil || len(workspaces) < 2 {
		t.Fatalf("expected at least 2 workspaces, got %d", len(workspaces))
	}

	err = db.RecordAuditLog(ctx, &AuditLog{
		WorkspaceID: "ws_default",
		Actor:       "admin_test",
		Action:      "key.revoked",
		TargetID:    "key_test_01",
		Timestamp:   time.Now().UTC(),
	})
	if err != nil {
		t.Fatalf("failed to record audit log: %v", err)
	}

	logs, totalLogs, err := db.ListAuditLogs(ctx, "ws_default", 10, 0)
	if err != nil || totalLogs < 1 || len(logs) < 1 {
		t.Fatalf("expected at least 1 audit log, got total=%d", totalLogs)
	}
}
