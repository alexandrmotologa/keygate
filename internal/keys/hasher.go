package keys

import (
	"crypto/sha256"
	"crypto/subtle"
	"encoding/hex"
)

// HashKey calculates the irreversible salted SHA-256 hash of a plaintext API key.
// Format: SHA-256(salt + ":" + rawKey)
func HashKey(rawKey, salt string) string {
	hasher := sha256.New()
	hasher.Write([]byte(salt))
	hasher.Write([]byte(":"))
	hasher.Write([]byte(rawKey))
	digest := hasher.Sum(nil)
	return hex.EncodeToString(digest)
}

// VerifyHash compares a plaintext key against an expected salted hash using constant-time comparison.
func VerifyHash(rawKey, salt, expectedHash string) bool {
	computedHash := HashKey(rawKey, salt)
	if len(computedHash) != len(expectedHash) {
		return false
	}
	return subtle.ConstantTimeCompare([]byte(computedHash), []byte(expectedHash)) == 1
}
