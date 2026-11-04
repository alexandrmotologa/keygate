package keys

import (
	"crypto/rand"
	"errors"
	"fmt"
	"strings"
)

// Base58 standard alphabet (avoids 0, O, I, l to prevent visual confusion)
const base58Alphabet = "123456789ABCDEFGHJKLMNPQRSTUVWXYZabcdefghijkmnopqrstuvwxyz"

// Base58Encode encodes a byte slice into a Base58 string.
func Base58Encode(input []byte) string {
	if len(input) == 0 {
		return ""
	}

	result := make([]byte, 0, len(input)*138/100+1)
	for _, b := range input {
		carry := int(b)
		for j := 0; j < len(result); j++ {
			carry += int(result[j]) << 8
			result[j] = byte(carry % 58)
			carry /= 58
		}
		for carry > 0 {
			result = append(result, byte(carry%58))
			carry /= 58
		}
	}

	// Preserve leading zeros
	for _, b := range input {
		if b == 0 {
			result = append(result, 0)
		} else {
			break
		}
	}

	// Reverse and map to alphabet
	output := make([]byte, len(result))
	for i, val := range result {
		output[len(result)-1-i] = base58Alphabet[val]
	}

	return string(output)
}

// GenerateRandomBytes returns cryptographically secure random bytes.
func GenerateRandomBytes(n int) ([]byte, error) {
	b := make([]byte, n)
	_, err := rand.Read(b)
	if err != nil {
		return nil, fmt.Errorf("crypto/rand error: %w", err)
	}
	return b, nil
}

// KeyGenerationOptions defines configurable parameters for key generation.
type KeyGenerationOptions struct {
	Prefix      string // e.g. "sk_live", "sk_test"
	WorkspaceID string // e.g. "ws_default"
	ByteLength  int    // Random bytes length, defaults to 24 (192 bits of entropy)
}

// GeneratedKey contains the plaintext key and generated key identifier.
type GeneratedKey struct {
	KeyID  string // e.g. "key_01jh..."
	RawKey string // e.g. "sk_live_ws_default_5K3mP8..." (only returned once)
	Prefix string // e.g. "sk_live"
}

// Generate creates a cryptographically secure, prefix-encoded API key.
func Generate(opts KeyGenerationOptions) (*GeneratedKey, error) {
	prefix := strings.TrimSpace(opts.Prefix)
	if prefix == "" {
		prefix = "sk_live"
	}
	prefix = strings.TrimSuffix(prefix, "_")

	byteLen := opts.ByteLength
	if byteLen <= 0 {
		byteLen = 24 // 192 bits entropy
	}

	randomBytes, err := GenerateRandomBytes(byteLen)
	if err != nil {
		return nil, err
	}
	secretEncoded := Base58Encode(randomBytes)

	// Generate key ID
	idBytes, err := GenerateRandomBytes(12)
	if err != nil {
		return nil, err
	}
	keyID := fmt.Sprintf("key_%s", Base58Encode(idBytes))

	ws := strings.TrimSpace(opts.WorkspaceID)
	if ws == "" {
		ws = "ws_default"
	}
	rawKey := fmt.Sprintf("%s_%s_%s", prefix, ws, secretEncoded)

	return &GeneratedKey{
		KeyID:  keyID,
		RawKey: rawKey,
		Prefix: prefix,
	}, nil
}

// ParseKey extracts prefix, workspace, and secret body from an API key.
// Format is: <prefix>_<workspace>_<secret> where secret is Base58 (no underscores).
// Example: sk_live_ws_default_9URWDxFZrU6m3TLunKTouAQ7VgoRk8YQc
func ParseKey(rawKey string) (prefix, workspaceID, secret string, err error) {
	rawKey = strings.TrimSpace(rawKey)
	if rawKey == "" {
		return "", "", "", errors.New("empty API key")
	}

	lastUnderscore := strings.LastIndex(rawKey, "_")
	if lastUnderscore == -1 || lastUnderscore == len(rawKey)-1 {
		return "", "", "", errors.New("invalid API key structure: missing secret suffix")
	}

	secret = rawKey[lastUnderscore+1:]
	prefixAndWs := rawKey[:lastUnderscore]

	if strings.HasPrefix(prefixAndWs, "sk_live_") {
		prefix = "sk_live"
		workspaceID = strings.TrimPrefix(prefixAndWs, "sk_live_")
	} else if strings.HasPrefix(prefixAndWs, "sk_test_") {
		prefix = "sk_test"
		workspaceID = strings.TrimPrefix(prefixAndWs, "sk_test_")
	} else {
		firstUnderscore := strings.Index(prefixAndWs, "_")
		if firstUnderscore != -1 {
			prefix = prefixAndWs[:firstUnderscore]
			workspaceID = prefixAndWs[firstUnderscore+1:]
		} else {
			prefix = prefixAndWs
			workspaceID = "ws_default"
		}
	}

	if workspaceID == "" {
		workspaceID = "ws_default"
	}

	return prefix, workspaceID, secret, nil
}
