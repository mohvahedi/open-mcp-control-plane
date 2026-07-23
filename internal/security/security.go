package security

import (
	"crypto/rand"
	"crypto/sha256"
	"crypto/subtle"
	"encoding/hex"
	"errors"
	"strings"
)

func NewToken() (string, error) {
	b := make([]byte, 24)
	if _, err := rand.Read(b); err != nil {
		return "", err
	}
	return hex.EncodeToString(b), nil
}

func HashToken(raw string) (string, error) {
	if strings.TrimSpace(raw) == "" {
		return "", errors.New("token required")
	}
	salt := make([]byte, 16)
	if _, err := rand.Read(salt); err != nil {
		return "", err
	}
	sum := sha256.Sum256(append(salt, []byte(raw)...))
	return hex.EncodeToString(salt) + ":" + hex.EncodeToString(sum[:]), nil
}

func VerifyToken(hashed, raw string) bool {
	parts := strings.Split(hashed, ":")
	if len(parts) != 2 {
		return false
	}
	salt, err := hex.DecodeString(parts[0])
	if err != nil {
		return false
	}
	expected, err := hex.DecodeString(parts[1])
	if err != nil {
		return false
	}
	sum := sha256.Sum256(append(salt, []byte(raw)...))
	if len(expected) != len(sum) {
		return false
	}
	return subtle.ConstantTimeCompare(expected, sum[:]) == 1
}

func RedactMap(input map[string]any) map[string]any {
	out := make(map[string]any, len(input))
	for k, v := range input {
		lk := strings.ToLower(k)
		if strings.Contains(lk, "secret") || strings.Contains(lk, "token") || strings.Contains(lk, "password") || strings.Contains(lk, "key") {
			out[k] = "[REDACTED]"
			continue
		}
		out[k] = v
	}
	return out
}
