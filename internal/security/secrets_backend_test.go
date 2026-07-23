package security

import (
	"context"
	"os"
	"path/filepath"
	"testing"
)

func TestLocalAESBackendRoundTrip(t *testing.T) {
	b, err := NewLocalAESBackend("unit-test-master-key")
	if err != nil {
		t.Fatal(err)
	}
	loc, err := b.Put(context.Background(), "sec-1", "super-secret")
	if err != nil {
		t.Fatal(err)
	}
	got, err := b.Get(context.Background(), loc)
	if err != nil {
		t.Fatal(err)
	}
	if got != "super-secret" {
		t.Fatalf("got %q", got)
	}
}

func TestEnvBackendRoundTrip(t *testing.T) {
	b := NewEnvBackend()
	loc, err := b.Put(context.Background(), "abc-def", "from-env")
	if err != nil {
		t.Fatal(err)
	}
	got, err := b.Get(context.Background(), loc)
	if err != nil || got != "from-env" {
		t.Fatalf("got %q err=%v", got, err)
	}
}

func TestFileBackendRoundTrip(t *testing.T) {
	dir := t.TempDir()
	b, err := NewFileBackend(dir)
	if err != nil {
		t.Fatal(err)
	}
	loc, err := b.Put(context.Background(), "file1", "disk-secret")
	if err != nil {
		t.Fatal(err)
	}
	got, err := b.Get(context.Background(), loc)
	if err != nil || got != "disk-secret" {
		t.Fatalf("got %q err=%v", got, err)
	}
	if _, err := os.Stat(filepath.Join(dir, "file1.secret")); err != nil {
		t.Fatal(err)
	}
}

func TestNewSecretBackendKinds(t *testing.T) {
	if _, err := NewSecretBackend("local", "k", ""); err != nil {
		t.Fatal(err)
	}
	if _, err := NewSecretBackend("env", "", ""); err != nil {
		t.Fatal(err)
	}
	if _, err := NewSecretBackend("file", "", t.TempDir()); err != nil {
		t.Fatal(err)
	}
	if _, err := NewSecretBackend("vault", "", ""); err == nil {
		t.Fatal("expected unknown backend error")
	}
}
