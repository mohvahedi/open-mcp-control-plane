package security

import (
	"crypto/aes"
	"crypto/cipher"
	"crypto/rand"
	"crypto/sha256"
	"encoding/base64"
	"errors"
	"fmt"
	"io"
	"strings"
)

// DeriveKey turns an arbitrary master secret into a 32-byte AES key.
func DeriveKey(master string) ([32]byte, error) {
	if strings.TrimSpace(master) == "" {
		return [32]byte{}, errors.New("master key required")
	}
	return sha256.Sum256([]byte(master)), nil
}

// EncryptSecret seals plaintext with AES-GCM and returns base64(nonce|ciphertext).
func EncryptSecret(masterKey [32]byte, plaintext string) (string, error) {
	block, err := aes.NewCipher(masterKey[:])
	if err != nil {
		return "", err
	}
	gcm, err := cipher.NewGCM(block)
	if err != nil {
		return "", err
	}
	nonce := make([]byte, gcm.NonceSize())
	if _, err := io.ReadFull(rand.Reader, nonce); err != nil {
		return "", err
	}
	sealed := gcm.Seal(nonce, nonce, []byte(plaintext), nil)
	return base64.StdEncoding.EncodeToString(sealed), nil
}

// DecryptSecret opens a value produced by EncryptSecret.
func DecryptSecret(masterKey [32]byte, encoded string) (string, error) {
	raw, err := base64.StdEncoding.DecodeString(encoded)
	if err != nil {
		return "", fmt.Errorf("invalid ciphertext encoding: %w", err)
	}
	block, err := aes.NewCipher(masterKey[:])
	if err != nil {
		return "", err
	}
	gcm, err := cipher.NewGCM(block)
	if err != nil {
		return "", err
	}
	if len(raw) < gcm.NonceSize() {
		return "", errors.New("ciphertext too short")
	}
	nonce, ciphertext := raw[:gcm.NonceSize()], raw[gcm.NonceSize():]
	plain, err := gcm.Open(nil, nonce, ciphertext, nil)
	if err != nil {
		return "", err
	}
	return string(plain), nil
}
