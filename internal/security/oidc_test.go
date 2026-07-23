package security

import (
	"testing"
	"time"
)

func TestSessionTokenRoundTrip(t *testing.T) {
	id := OIDCIdentity{Subject: "sub-1", Email: "a@example.com", Name: "A"}
	tok, err := MintSessionToken("sess-secret", id, time.Hour)
	if err != nil {
		t.Fatal(err)
	}
	got, err := VerifySessionToken("sess-secret", tok)
	if err != nil {
		t.Fatal(err)
	}
	if got.Subject != id.Subject || got.Email != id.Email || got.Name != id.Name {
		t.Fatalf("got %+v", got)
	}
	if _, err := VerifySessionToken("wrong", tok); err == nil {
		t.Fatal("expected bad signature")
	}
}

func TestSessionTokenExpiry(t *testing.T) {
	id := OIDCIdentity{Subject: "s", Email: "e@x.com"}
	tok, err := MintSessionToken("k", id, -time.Hour)
	if err != nil {
		t.Fatal(err)
	}
	if _, err := VerifySessionToken("k", tok); err == nil {
		t.Fatal("expected expired")
	}
}

func TestParseAllowedCSV(t *testing.T) {
	got := ParseAllowedCSV(" a@x.com, b@y.com , ")
	if len(got) != 2 || got[0] != "a@x.com" || got[1] != "b@y.com" {
		t.Fatalf("%v", got)
	}
}

func TestRedirectSafe(t *testing.T) {
	if !RedirectSafe("/dashboard") || RedirectSafe("//evil.com") || RedirectSafe("http://evil.com") {
		t.Fatal("redirect safety checks failed")
	}
}
