package security

import "testing"

func TestEncryptDecryptSecretRoundTrip(t *testing.T) {
	key, err := DeriveKey("test-master-key")
	if err != nil {
		t.Fatal(err)
	}
	ciphertext, err := EncryptSecret(key, "super-secret-value")
	if err != nil {
		t.Fatal(err)
	}
	if ciphertext == "" || ciphertext == "super-secret-value" {
		t.Fatalf("ciphertext not sealed: %q", ciphertext)
	}
	plain, err := DecryptSecret(key, ciphertext)
	if err != nil {
		t.Fatal(err)
	}
	if plain != "super-secret-value" {
		t.Fatalf("got %q", plain)
	}
}

func TestDeriveKeyRequiresMaster(t *testing.T) {
	if _, err := DeriveKey(""); err == nil {
		t.Fatal("expected error")
	}
}
