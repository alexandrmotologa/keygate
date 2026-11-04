package keys

import (
	"context"
	"os"
	"path/filepath"
	"testing"
	"time"

	"github.com/alexandrmotologa/keygate/internal/storage"
)

func setupTestValidator(t *testing.T) (*Validator, *storage.DB, func()) {
	tempDir, err := os.MkdirTemp("", "keygate-val-*")
	if err != nil {
		t.Fatalf("failed to create temp dir: %v", err)
	}

	dbPath := filepath.Join(tempDir, "test.db")
	db, err := storage.Open(dbPath)
	if err != nil {
		os.RemoveAll(tempDir)
		t.Fatalf("failed to open storage: %v", err)
	}

	validator, err := NewValidator(db)
	if err != nil {
		db.Close()
		os.RemoveAll(tempDir)
		t.Fatalf("failed to create validator: %v", err)
	}

	cleanup := func() {
		validator.Close()
		db.Close()
		os.RemoveAll(tempDir)
	}

	return validator, db, cleanup
}

func TestValidatorComprehensive(t *testing.T) {
	val, db, cleanup := setupTestValidator(t)
	defer cleanup()

	ctx := context.Background()

	// 1. Generate and insert key
	gen, err := Generate(KeyGenerationOptions{
		Prefix:      "sk_live",
		WorkspaceID: "ws_default",
	})
	if err != nil {
		t.Fatalf("generation failed: %v", err)
	}

	salt, _ := db.GetWorkspaceSalt(ctx, "ws_default")
	keyHash := HashKey(gen.RawKey, salt)

	now := time.Now().UTC()
	future := now.Add(24 * time.Hour)
	past := now.Add(-1 * time.Hour)

	err = db.CreateKey(ctx, &storage.APIKey{
		ID:           gen.KeyID,
		KeyHash:      keyHash,
		Prefix:       gen.Prefix,
		Name:         "Prod Gateway Key",
		OwnerID:      "usr_admin",
		WorkspaceID:  "ws_default",
		Tier:         "pro",
		RateLimit:    500,
		RateDuration: 60,
		Burst:        20,
		Scopes:       []string{"read:users", "write:users"},
		IPAllowlist:  []string{"127.0.0.1", "10.0.0.0/16"},
		ExpiresAt:    &future,
		Enabled:      true,
		CreatedAt:    now,
		UpdatedAt:    now,
	})
	if err != nil {
		t.Fatalf("failed to insert key: %v", err)
	}

	// 2. First verify (cache miss)
	res1 := val.Verify(ctx, ValidationRequest{
		RawKey:         gen.RawKey,
		RequiredScopes: []string{"read:users"},
		ClientIP:       "127.0.0.1",
	})
	if !res1.Valid || res1.Code != CodeValid {
		t.Fatalf("first verification failed: %s (%s)", res1.Message, res1.Code)
	}
	if res1.Cached {
		t.Errorf("expected first call to be cache miss")
	}

	// Wait for Ristretto internal buffer processing
	time.Sleep(10 * time.Millisecond)

	// 3. Second verify (cache hit)
	res2 := val.Verify(ctx, ValidationRequest{
		RawKey:         gen.RawKey,
		RequiredScopes: []string{"read:users"},
		ClientIP:       "127.0.0.1",
	})
	if !res2.Valid || res2.Code != CodeValid {
		t.Fatalf("second verification failed: %s", res2.Message)
	}
	if !res2.Cached {
		t.Errorf("expected second call to be cached")
	}

	// 4. IP Allowlist rejection
	resIP := val.Verify(ctx, ValidationRequest{
		RawKey:   gen.RawKey,
		ClientIP: "192.168.1.100", // not in allowlist
	})
	if resIP.Valid || resIP.Code != CodeIPForbidden {
		t.Errorf("expected IP_FORBIDDEN, got: %s", resIP.Code)
	}

	// 5. Missing required scope rejection
	resScope := val.Verify(ctx, ValidationRequest{
		RawKey:         gen.RawKey,
		RequiredScopes: []string{"admin:billing"}, // key only has read:users, write:users
		ClientIP:       "127.0.0.1",
	})
	if resScope.Valid || resScope.Code != CodeInsufficientScopes {
		t.Errorf("expected INSUFFICIENT_SCOPES, got: %s", resScope.Code)
	}

	// 6. Cache Invalidation on Revocation
	val.InvalidateByID(gen.KeyID)
	_, err = db.RevokeKey(ctx, gen.KeyID)
	if err != nil {
		t.Fatalf("failed to revoke key: %v", err)
	}

	resRevoked := val.Verify(ctx, ValidationRequest{
		RawKey: gen.RawKey,
	})
	if resRevoked.Valid || resRevoked.Code != CodeKeyRevoked {
		t.Errorf("expected KEY_REVOKED, got: %s", resRevoked.Code)
	}

	// 7. Expired key test
	genExp, _ := Generate(KeyGenerationOptions{
		Prefix:      "sk_live",
		WorkspaceID: "ws_default",
	})
	hashExp := HashKey(genExp.RawKey, salt)
	_ = db.CreateKey(ctx, &storage.APIKey{
		ID:          genExp.KeyID,
		KeyHash:     hashExp,
		Prefix:      genExp.Prefix,
		Name:        "Expired Key",
		OwnerID:     "usr_test",
		WorkspaceID: "ws_default",
		Tier:        "free",
		ExpiresAt:   &past,
		Enabled:     true,
		CreatedAt:   past,
		UpdatedAt:   past,
	})

	resExp := val.Verify(ctx, ValidationRequest{
		RawKey: genExp.RawKey,
	})
	if resExp.Valid || resExp.Code != CodeKeyExpired {
		t.Errorf("expected KEY_EXPIRED, got: %s", resExp.Code)
	}
}

func BenchmarkValidatorCacheHit(b *testing.B) {
	tempDir, err := os.MkdirTemp("", "keygate-bench-*")
	if err != nil {
		b.Fatalf("failed to create temp dir: %v", err)
	}
	defer os.RemoveAll(tempDir)

	dbPath := filepath.Join(tempDir, "bench.db")
	db, _ := storage.Open(dbPath)
	defer db.Close()

	validator, _ := NewValidator(db)
	defer validator.Close()

	ctx := context.Background()
	gen, _ := Generate(KeyGenerationOptions{
		Prefix:      "sk_live",
		WorkspaceID: "ws_default",
	})
	salt, _ := db.GetWorkspaceSalt(ctx, "ws_default")
	keyHash := HashKey(gen.RawKey, salt)

	_ = db.CreateKey(ctx, &storage.APIKey{
		ID:           gen.KeyID,
		KeyHash:      keyHash,
		Prefix:       gen.Prefix,
		Name:         "Benchmark Key",
		OwnerID:      "usr_bench",
		WorkspaceID:  "ws_default",
		Tier:         "enterprise",
		RateLimit:    10000,
		RateDuration: 60,
		Burst:        500,
		Enabled:      true,
		CreatedAt:    time.Now().UTC(),
		UpdatedAt:    time.Now().UTC(),
	})

	// Warmup cache
	validator.Verify(ctx, ValidationRequest{RawKey: gen.RawKey})
	time.Sleep(20 * time.Millisecond)

	req := ValidationRequest{RawKey: gen.RawKey}

	b.ResetTimer()
	b.RunParallel(func(pb *testing.PB) {
		for pb.Next() {
			res := validator.Verify(ctx, req)
			if !res.Valid {
				b.Errorf("verification failed: %s", res.Message)
			}
		}
	})
}
