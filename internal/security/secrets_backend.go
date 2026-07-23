package security

import (
	"context"
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"sync"
)

// SecretBackend stores and retrieves secret plaintext by opaque reference ID.
// The control plane persists only metadata + backend-specific ciphertext/locator.
type SecretBackend interface {
	Name() string
	Put(ctx context.Context, id, plaintext string) (locator string, err error)
	Get(ctx context.Context, locator string) (plaintext string, err error)
}

// LocalAESBackend keeps secrets encrypted with the process master key (AES-GCM).
type LocalAESBackend struct {
	master [32]byte
}

func NewLocalAESBackend(masterKey string) (*LocalAESBackend, error) {
	key, err := DeriveKey(masterKey)
	if err != nil {
		return nil, err
	}
	return &LocalAESBackend{master: key}, nil
}

func (b *LocalAESBackend) Name() string { return "local" }

func (b *LocalAESBackend) Put(_ context.Context, _, plaintext string) (string, error) {
	return EncryptSecret(b.master, plaintext)
}

func (b *LocalAESBackend) Get(_ context.Context, locator string) (string, error) {
	return DecryptSecret(b.master, locator)
}

// EnvBackend reads secrets from environment variables.
// Put stores the value in-process only (for tests/dev); production should inject env vars externally.
type EnvBackend struct {
	mu   sync.RWMutex
	vals map[string]string
}

func NewEnvBackend() *EnvBackend {
	return &EnvBackend{vals: map[string]string{}}
}

func (b *EnvBackend) Name() string { return "env" }

func (b *EnvBackend) Put(_ context.Context, id, plaintext string) (string, error) {
	key := "OPENMCP_SECRET_" + strings.ToUpper(strings.ReplaceAll(id, "-", "_"))
	b.mu.Lock()
	b.vals[key] = plaintext
	b.mu.Unlock()
	return "env:" + key, nil
}

func (b *EnvBackend) Get(_ context.Context, locator string) (string, error) {
	if !strings.HasPrefix(locator, "env:") {
		return "", errors.New("invalid env locator")
	}
	key := strings.TrimPrefix(locator, "env:")
	b.mu.RLock()
	v, ok := b.vals[key]
	b.mu.RUnlock()
	if ok {
		return v, nil
	}
	if v := os.Getenv(key); v != "" {
		return v, nil
	}
	return "", fmt.Errorf("env secret %s not found", key)
}

// FileBackend stores one file per secret under a directory (mode 0600).
type FileBackend struct {
	dir string
}

func NewFileBackend(dir string) (*FileBackend, error) {
	if strings.TrimSpace(dir) == "" {
		return nil, errors.New("file backend directory required")
	}
	if err := os.MkdirAll(dir, 0o700); err != nil {
		return nil, err
	}
	return &FileBackend{dir: dir}, nil
}

func (b *FileBackend) Name() string { return "file" }

func (b *FileBackend) Put(_ context.Context, id, plaintext string) (string, error) {
	path := filepath.Join(b.dir, id+".secret")
	if err := os.WriteFile(path, []byte(plaintext), 0o600); err != nil {
		return "", err
	}
	return "file:" + path, nil
}

func (b *FileBackend) Get(_ context.Context, locator string) (string, error) {
	if !strings.HasPrefix(locator, "file:") {
		return "", errors.New("invalid file locator")
	}
	path := strings.TrimPrefix(locator, "file:")
	bts, err := os.ReadFile(path)
	if err != nil {
		return "", err
	}
	return string(bts), nil
}

// NewSecretBackend selects a backend by name.
func NewSecretBackend(kind, masterKey, fileDir string) (SecretBackend, error) {
	switch strings.ToLower(strings.TrimSpace(kind)) {
	case "", "local", "aes", "aes-gcm":
		return NewLocalAESBackend(masterKey)
	case "env":
		return NewEnvBackend(), nil
	case "file":
		return NewFileBackend(fileDir)
	default:
		return nil, fmt.Errorf("unknown secrets backend %q (local|env|file)", kind)
	}
}
