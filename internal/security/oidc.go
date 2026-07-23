package security

import (
	"context"
	"crypto/rand"
	"crypto/sha256"
	"encoding/base64"
	"encoding/hex"
	"errors"
	"fmt"
	"net/http"
	"net/url"
	"strings"
	"sync"
	"time"

	"github.com/coreos/go-oidc/v3/oidc"
	"golang.org/x/oauth2"
)

// OIDCConfig configures optional OpenID Connect admin authentication.
type OIDCConfig struct {
	Enabled      bool
	IssuerURL    string
	ClientID     string
	ClientSecret string
	RedirectURL  string
	Scopes       []string
	// AllowedEmails, if non-empty, restricts who may administer after OIDC login.
	AllowedEmails []string
	// AllowedSubjects, if non-empty, restricts by OIDC `sub` claim.
	AllowedSubjects []string
}

// OIDCIdentity is a verified admin principal from OIDC.
type OIDCIdentity struct {
	Subject string
	Email   string
	Name    string
}

// OIDCAuthenticator handles login redirect, callback, and bearer ID-token validation.
type OIDCAuthenticator struct {
	cfg      OIDCConfig
	provider *oidc.Provider
	verifier *oidc.IDTokenVerifier
	oauth    oauth2.Config

	mu     sync.Mutex
	states map[string]stateEntry
}

type stateEntry struct {
	Verifier  string
	ExpiresAt time.Time
}

func NewOIDCAuthenticator(ctx context.Context, cfg OIDCConfig) (*OIDCAuthenticator, error) {
	if !cfg.Enabled {
		return &OIDCAuthenticator{cfg: cfg, states: map[string]stateEntry{}}, nil
	}
	if cfg.IssuerURL == "" || cfg.ClientID == "" || cfg.RedirectURL == "" {
		return nil, errors.New("oidc issuer, client id, and redirect url are required when enabled")
	}
	provider, err := oidc.NewProvider(ctx, cfg.IssuerURL)
	if err != nil {
		return nil, fmt.Errorf("oidc provider: %w", err)
	}
	scopes := cfg.Scopes
	if len(scopes) == 0 {
		scopes = []string{oidc.ScopeOpenID, "profile", "email"}
	}
	a := &OIDCAuthenticator{
		cfg:      cfg,
		provider: provider,
		verifier: provider.Verifier(&oidc.Config{ClientID: cfg.ClientID}),
		states:   map[string]stateEntry{},
		oauth: oauth2.Config{
			ClientID:     cfg.ClientID,
			ClientSecret: cfg.ClientSecret,
			RedirectURL:  cfg.RedirectURL,
			Endpoint:     provider.Endpoint(),
			Scopes:       scopes,
		},
	}
	return a, nil
}

func (a *OIDCAuthenticator) Enabled() bool {
	return a != nil && a.cfg.Enabled && a.provider != nil
}

func (a *OIDCAuthenticator) AuthCodeURL() (authURL, state string, err error) {
	if !a.Enabled() {
		return "", "", errors.New("oidc disabled")
	}
	state, err = randomURLSafe(24)
	if err != nil {
		return "", "", err
	}
	verifier, err := randomURLSafe(32)
	if err != nil {
		return "", "", err
	}
	a.mu.Lock()
	a.gcLocked()
	a.states[state] = stateEntry{Verifier: verifier, ExpiresAt: time.Now().Add(10 * time.Minute)}
	a.mu.Unlock()
	// PKCE S256
	sum := sha256.Sum256([]byte(verifier))
	challenge := base64.RawURLEncoding.EncodeToString(sum[:])
	url := a.oauth.AuthCodeURL(state,
		oauth2.SetAuthURLParam("code_challenge", challenge),
		oauth2.SetAuthURLParam("code_challenge_method", "S256"),
	)
	return url, state, nil
}

func (a *OIDCAuthenticator) Exchange(ctx context.Context, code, state string) (OIDCIdentity, string, error) {
	if !a.Enabled() {
		return OIDCIdentity{}, "", errors.New("oidc disabled")
	}
	a.mu.Lock()
	a.gcLocked()
	entry, ok := a.states[state]
	delete(a.states, state)
	a.mu.Unlock()
	if !ok || time.Now().After(entry.ExpiresAt) {
		return OIDCIdentity{}, "", errors.New("invalid or expired oauth state")
	}
	tok, err := a.oauth.Exchange(ctx, code, oauth2.SetAuthURLParam("code_verifier", entry.Verifier))
	if err != nil {
		return OIDCIdentity{}, "", fmt.Errorf("token exchange: %w", err)
	}
	rawID, ok := tok.Extra("id_token").(string)
	if !ok || rawID == "" {
		return OIDCIdentity{}, "", errors.New("id_token missing from token response")
	}
	id, err := a.VerifyIDToken(ctx, rawID)
	if err != nil {
		return OIDCIdentity{}, "", err
	}
	return id, rawID, nil
}

func (a *OIDCAuthenticator) VerifyIDToken(ctx context.Context, raw string) (OIDCIdentity, error) {
	if !a.Enabled() {
		return OIDCIdentity{}, errors.New("oidc disabled")
	}
	token, err := a.verifier.Verify(ctx, raw)
	if err != nil {
		return OIDCIdentity{}, err
	}
	var claims struct {
		Email         string `json:"email"`
		EmailVerified *bool  `json:"email_verified"`
		Name          string `json:"name"`
		Preferred     string `json:"preferred_username"`
	}
	if err := token.Claims(&claims); err != nil {
		return OIDCIdentity{}, err
	}
	id := OIDCIdentity{
		Subject: token.Subject,
		Email:   strings.ToLower(strings.TrimSpace(claims.Email)),
		Name:    claims.Name,
	}
	if id.Name == "" {
		id.Name = claims.Preferred
	}
	if err := a.authorize(id); err != nil {
		return OIDCIdentity{}, err
	}
	return id, nil
}

func (a *OIDCAuthenticator) authorize(id OIDCIdentity) error {
	if len(a.cfg.AllowedSubjects) > 0 {
		ok := false
		for _, s := range a.cfg.AllowedSubjects {
			if s == id.Subject {
				ok = true
				break
			}
		}
		if !ok {
			return errors.New("oidc subject not allowed")
		}
	}
	if len(a.cfg.AllowedEmails) > 0 {
		ok := false
		for _, e := range a.cfg.AllowedEmails {
			if strings.EqualFold(e, id.Email) {
				ok = true
				break
			}
		}
		if !ok {
			return errors.New("oidc email not allowed")
		}
	}
	return nil
}

func (a *OIDCAuthenticator) gcLocked() {
	now := time.Now()
	for k, v := range a.states {
		if now.After(v.ExpiresAt) {
			delete(a.states, k)
		}
	}
}

func randomURLSafe(n int) (string, error) {
	b := make([]byte, n)
	if _, err := rand.Read(b); err != nil {
		return "", err
	}
	return base64.RawURLEncoding.EncodeToString(b), nil
}

// SessionToken is a short-lived signed admin session after OIDC login (HMAC hex).
// Format: expiryUnixHex.payloadHex.sigHex where payload is subject|email|name
func MintSessionToken(secret string, id OIDCIdentity, ttl time.Duration) (string, error) {
	if strings.TrimSpace(secret) == "" {
		return "", errors.New("session secret required")
	}
	if ttl == 0 {
		ttl = 8 * time.Hour
	}
	// negative ttl mints an already-expired token (useful in tests).

	exp := time.Now().Add(ttl).Unix()
	payload := fmt.Sprintf("%d|%s|%s|%s", exp, id.Subject, id.Email, id.Name)
	sum := sha256.Sum256(append([]byte(secret+"|"), []byte(payload)...))
	return fmt.Sprintf("%x.%s.%s",
		exp,
		hex.EncodeToString([]byte(payload)),
		hex.EncodeToString(sum[:]),
	), nil
}

func VerifySessionToken(secret, token string) (OIDCIdentity, error) {
	parts := strings.Split(token, ".")
	if len(parts) != 3 {
		return OIDCIdentity{}, errors.New("invalid session token")
	}
	payloadB, err := hex.DecodeString(parts[1])
	if err != nil {
		return OIDCIdentity{}, err
	}
	sigB, err := hex.DecodeString(parts[2])
	if err != nil {
		return OIDCIdentity{}, err
	}
	sum := sha256.Sum256(append([]byte(secret+"|"), payloadB...))
	if subtleConstantTimeCompare(sigB, sum[:]) != 1 {
		return OIDCIdentity{}, errors.New("bad session signature")
	}
	fields := strings.SplitN(string(payloadB), "|", 4)
	if len(fields) != 4 {
		return OIDCIdentity{}, errors.New("bad session payload")
	}
	var exp int64
	if _, err := fmt.Sscanf(fields[0], "%d", &exp); err != nil {
		return OIDCIdentity{}, err
	}
	if time.Now().Unix() > exp {
		return OIDCIdentity{}, errors.New("session expired")
	}
	return OIDCIdentity{Subject: fields[1], Email: fields[2], Name: fields[3]}, nil
}

func subtleConstantTimeCompare(a, b []byte) int {
	if len(a) != len(b) {
		return 0
	}
	var v byte
	for i := range a {
		v |= a[i] ^ b[i]
	}
	if v == 0 {
		return 1
	}
	return 0
}

// ParseAllowedCSV splits comma-separated allowlists.
func ParseAllowedCSV(v string) []string {
	if strings.TrimSpace(v) == "" {
		return nil
	}
	parts := strings.Split(v, ",")
	out := make([]string, 0, len(parts))
	for _, p := range parts {
		p = strings.TrimSpace(p)
		if p != "" {
			out = append(out, p)
		}
	}
	return out
}

// RedirectSafe returns true when u is a relative path or matches allowed host.
func RedirectSafe(raw string) bool {
	if raw == "" || strings.HasPrefix(raw, "/") && !strings.HasPrefix(raw, "//") {
		return true
	}
	u, err := url.Parse(raw)
	return err == nil && u.Host == "" && strings.HasPrefix(u.Path, "/")
}

// BearerToken extracts a bearer token from Authorization header.
func BearerToken(r *http.Request) string {
	h := r.Header.Get("Authorization")
	if strings.HasPrefix(strings.ToLower(h), "bearer ") {
		return strings.TrimSpace(h[7:])
	}
	return ""
}
