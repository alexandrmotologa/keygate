package keys

import (
	"strings"
	"testing"
)

func TestKeyGenerationAndHashing(t *testing.T) {
	// Test 1: Standard key generation
	gen, err := Generate(KeyGenerationOptions{
		Prefix:      "sk_live",
		WorkspaceID: "ws_default",
	})
	if err != nil {
		t.Fatalf("failed to generate key: %v", err)
	}

	if !strings.HasPrefix(gen.RawKey, "sk_live_ws_default_") {
		t.Errorf("unexpected key format: %s", gen.RawKey)
	}
	if !strings.HasPrefix(gen.KeyID, "key_") {
		t.Errorf("unexpected key ID format: %s", gen.KeyID)
	}

	// Test 2: Parsing key components
	prefix, ws, secret, err := ParseKey(gen.RawKey)
	if err != nil {
		t.Fatalf("failed to parse key: %v", err)
	}
	if prefix != "sk_live" || ws != "ws_default" || len(secret) == 0 {
		t.Errorf("parsed key mismatch: prefix=%s, ws=%s, secret=%s", prefix, ws, secret)
	}

	// Test 3: Salted Hashing & Verification
	salt := "test_random_salt_12345"
	hash := HashKey(gen.RawKey, salt)
	if len(hash) != 64 {
		t.Errorf("expected 64-char hex SHA-256 digest, got length %d", len(hash))
	}

	if !VerifyHash(gen.RawKey, salt, hash) {
		t.Errorf("verification failed for valid key and salt")
	}

	// Test 4: Rejection on modified key
	if VerifyHash(gen.RawKey+"extra", salt, hash) {
		t.Errorf("verification should fail for modified key")
	}

	// Test 5: Rejection on wrong salt
	if VerifyHash(gen.RawKey, "wrong_salt", hash) {
		t.Errorf("verification should fail for wrong salt")
	}

	// Test 6: Collision test on 1,000 keys
	seen := make(map[string]bool)
	for i := 0; i < 1000; i++ {
		k, err := Generate(KeyGenerationOptions{
			Prefix:      "sk_live",
			WorkspaceID: "ws_prod",
		})
		if err != nil {
			t.Fatalf("generation failed: %v", err)
		}
		if seen[k.RawKey] {
			t.Fatalf("collision detected for key: %s", k.RawKey)
		}
		seen[k.RawKey] = true
	}
}
