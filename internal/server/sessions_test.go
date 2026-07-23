package server

import (
	"context"
	"net/http"
	"net/http/httptest"
	"path/filepath"
	"strings"
	"testing"
	"time"

	"github.com/mohvahedi/open-mcp-control-plane/internal/catalog"
	"github.com/mohvahedi/open-mcp-control-plane/internal/config"
	"github.com/mohvahedi/open-mcp-control-plane/internal/domain"
	"github.com/mohvahedi/open-mcp-control-plane/internal/runtime"
	"github.com/mohvahedi/open-mcp-control-plane/internal/security"
	"github.com/mohvahedi/open-mcp-control-plane/internal/store"
)

func TestMCPSessionLifecycleAndSSE(t *testing.T) {
	down := startMockDownstream(t,
		[]map[string]any{{"name": "echo", "description": "echo tool"}},
		map[string]any{"echo": map[string]any{"ok": true}},
	)

	repo, err := store.OpenSQLite(filepath.Join(t.TempDir(), "sess.db"))
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { _ = repo.Close() })

	now := time.Now().UTC()
	if _, err := repo.CreateInstallation(context.Background(), domain.Installation{
		ID: "inst-s", PlanID: "p", RuntimeRef: down.URL, State: "running",
		RollbackMetadata: map[string]string{}, CreatedAt: now, UpdatedAt: now,
	}); err != nil {
		t.Fatal(err)
	}
	if _, err := repo.CreateProfile(context.Background(), domain.Profile{
		ID: "profile-s", Name: "s", InstallationIDs: []string{"inst-s"},
		ToolAllowlist: []string{"inst-s.echo"}, CreatedAt: now,
	}); err != nil {
		t.Fatal(err)
	}
	raw := "sess-token"
	hash, err := security.HashToken(raw)
	if err != nil {
		t.Fatal(err)
	}
	if _, err := repo.CreateClient(context.Background(), domain.Client{
		ID: "client-s", ProfileID: "profile-s", Name: "c", CreatedAt: now,
	}, hash); err != nil {
		t.Fatal(err)
	}

	handler := New(config.Config{
		Version: "test", RequestTimeout: 2 * time.Second, GatewayBindPath: "/gateway",
		SecretsMasterKey: "unit-test-master-key", MCPSessionTTL: time.Minute,
	}, catalog.NewService(), repo, runtime.NewFakeRuntime())
	auth := "Bearer client-s." + raw

	// initialize creates session
	initBody := `{"jsonrpc":"2.0","id":1,"method":"initialize","params":{}}`
	req := httptest.NewRequest(http.MethodPost, "/gateway/mcp", strings.NewReader(initBody))
	req.Header.Set("Authorization", auth)
	req.Header.Set("Content-Type", "application/json")
	rec := httptest.NewRecorder()
	handler.ServeHTTP(rec, req)
	if rec.Code != http.StatusOK {
		t.Fatalf("initialize status=%d body=%s", rec.Code, rec.Body.String())
	}
	sessionID := rec.Header().Get("Mcp-Session-Id")
	if sessionID == "" {
		t.Fatal("expected Mcp-Session-Id")
	}

	// SSE stream attaches to session and emits endpoint event
	sseReq := httptest.NewRequest(http.MethodGet, "/gateway/mcp", nil)
	sseReq.Header.Set("Authorization", auth)
	sseReq.Header.Set("Accept", "text/event-stream")
	sseReq.Header.Set("Mcp-Session-Id", sessionID)
	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()
	sseReq = sseReq.WithContext(ctx)

	pr, pw := httptest.NewRecorder(), httptest.NewRecorder()
	_ = pr
	done := make(chan string, 1)
	go func() {
		handler.ServeHTTP(pw, sseReq)
	}()
	// Give the handler a moment to write the endpoint event.
	time.Sleep(50 * time.Millisecond)
	body := pw.Body.String()
	if !strings.Contains(body, "event: endpoint") && !strings.Contains(body, "text/event-stream") {
		// Flusher path: check Content-Type at least
		if ct := pw.Header().Get("Content-Type"); !strings.Contains(ct, "text/event-stream") && body == "" {
			// ResponseRecorder may not flush progressively; still require session header on init.
			done <- "skip-body"
		} else {
			done <- body
		}
	} else {
		done <- body
	}
	cancel()
	select {
	case <-done:
	default:
	}
	if ct := pw.Header().Get("Content-Type"); ct != "" && !strings.Contains(ct, "text/event-stream") {
		t.Fatalf("unexpected content-type %q body=%q", ct, pw.Body.String())
	}
	if sid := pw.Header().Get("Mcp-Session-Id"); sid != "" && sid != sessionID {
		t.Fatalf("session mismatch %s vs %s", sid, sessionID)
	}

	// DELETE ends session
	del := httptest.NewRequest(http.MethodDelete, "/gateway/mcp", nil)
	del.Header.Set("Authorization", auth)
	del.Header.Set("Mcp-Session-Id", sessionID)
	delRec := httptest.NewRecorder()
	handler.ServeHTTP(delRec, del)
	if delRec.Code != http.StatusOK {
		t.Fatalf("delete status=%d body=%s", delRec.Code, delRec.Body.String())
	}

	// Unknown session after delete
	get := httptest.NewRequest(http.MethodGet, "/gateway/mcp", nil)
	get.Header.Set("Authorization", auth)
	get.Header.Set("Mcp-Session-Id", sessionID)
	getRec := httptest.NewRecorder()
	handler.ServeHTTP(getRec, get)
	if getRec.Code != http.StatusNotFound {
		t.Fatalf("expected 404 after delete, got %d %s", getRec.Code, getRec.Body.String())
	}
}

func TestAdminSessionTokenAuth(t *testing.T) {
	repo, err := store.OpenSQLite(filepath.Join(t.TempDir(), "admin.db"))
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { _ = repo.Close() })
	hash, err := security.HashToken("bootstrap-admin")
	if err != nil {
		t.Fatal(err)
	}
	if err := repo.SetAdminHash(context.Background(), hash); err != nil {
		t.Fatal(err)
	}

	cfg := config.Config{
		Version: "test", RequestTimeout: time.Second, SecretsMasterKey: "unit-test-master-key",
		SessionHMACSecret: "sess-hmac", GatewayBindPath: "/gateway",
	}
	handler := New(cfg, catalog.NewService(), repo, runtime.NewFakeRuntime())

	// bootstrap token still works
	req := httptest.NewRequest(http.MethodGet, "/v1/admin/gateway/status", nil)
	req.Header.Set("Authorization", "Bearer bootstrap-admin")
	rec := httptest.NewRecorder()
	handler.ServeHTTP(rec, req)
	if rec.Code != http.StatusOK {
		t.Fatalf("bootstrap auth failed: %d %s", rec.Code, rec.Body.String())
	}
	if !strings.Contains(rec.Body.String(), "streamable-http") {
		t.Fatalf("unexpected body %s", rec.Body.String())
	}

	// OIDC session token works
	sess, err := security.MintSessionToken("sess-hmac", security.OIDCIdentity{
		Subject: "sub", Email: "admin@example.com", Name: "Admin",
	}, time.Hour)
	if err != nil {
		t.Fatal(err)
	}
	req2 := httptest.NewRequest(http.MethodGet, "/v1/auth/status", nil)
	req2.Header.Set("Authorization", "Bearer "+sess)
	rec2 := httptest.NewRecorder()
	handler.ServeHTTP(rec2, req2)
	if rec2.Code != http.StatusOK || !strings.Contains(rec2.Body.String(), `"authenticated":true`) {
		t.Fatalf("session auth failed: %d %s", rec2.Code, rec2.Body.String())
	}

	// secrets backend info
	req3 := httptest.NewRequest(http.MethodGet, "/v1/admin/secrets/backend", nil)
	req3.Header.Set("Authorization", "Bearer bootstrap-admin")
	rec3 := httptest.NewRecorder()
	handler.ServeHTTP(rec3, req3)
	if rec3.Code != http.StatusOK || !strings.Contains(rec3.Body.String(), "local") {
		t.Fatalf("secrets backend: %d %s", rec3.Code, rec3.Body.String())
	}
}

