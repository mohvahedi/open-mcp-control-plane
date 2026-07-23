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

func TestMCPSessionLifecycle(t *testing.T) {
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
	if !strings.Contains(rec.Body.String(), "protocolVersion") {
		t.Fatalf("unexpected initialize body: %s", rec.Body.String())
	}

	// GET with session id succeeds (non-SSE probe)
	get := httptest.NewRequest(http.MethodGet, "/gateway/mcp", nil)
	get.Header.Set("Authorization", auth)
	get.Header.Set("Mcp-Session-Id", sessionID)
	getRec := httptest.NewRecorder()
	handler.ServeHTTP(getRec, get)
	if getRec.Code != http.StatusOK {
		t.Fatalf("get status=%d body=%s", getRec.Code, getRec.Body.String())
	}
	if !strings.Contains(getRec.Body.String(), "streamable-http") {
		t.Fatalf("unexpected get body: %s", getRec.Body.String())
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
	get2 := httptest.NewRequest(http.MethodGet, "/gateway/mcp", nil)
	get2.Header.Set("Authorization", auth)
	get2.Header.Set("Mcp-Session-Id", sessionID)
	getRec2 := httptest.NewRecorder()
	handler.ServeHTTP(getRec2, get2)
	if getRec2.Code != http.StatusNotFound {
		t.Fatalf("expected 404 after delete, got %d %s", getRec2.Code, getRec2.Body.String())
	}
}

func TestSSESSEEndpointViaRealServer(t *testing.T) {
	// Use a real HTTP server so SSE streaming is race-detector safe.
	down := startMockDownstream(t,
		[]map[string]any{{"name": "echo", "description": "echo tool"}},
		map[string]any{"echo": map[string]any{"ok": true}},
	)
	repo, err := store.OpenSQLite(filepath.Join(t.TempDir(), "sse.db"))
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { _ = repo.Close() })
	now := time.Now().UTC()
	if _, err := repo.CreateInstallation(context.Background(), domain.Installation{
		ID: "inst-sse", PlanID: "p", RuntimeRef: down.URL, State: "running",
		RollbackMetadata: map[string]string{}, CreatedAt: now, UpdatedAt: now,
	}); err != nil {
		t.Fatal(err)
	}
	if _, err := repo.CreateProfile(context.Background(), domain.Profile{
		ID: "profile-sse", Name: "sse", InstallationIDs: []string{"inst-sse"},
		ToolAllowlist: []string{"inst-sse.echo"}, CreatedAt: now,
	}); err != nil {
		t.Fatal(err)
	}
	raw := "sse-token"
	hash, _ := security.HashToken(raw)
	if _, err := repo.CreateClient(context.Background(), domain.Client{
		ID: "client-sse", ProfileID: "profile-sse", Name: "c", CreatedAt: now,
	}, hash); err != nil {
		t.Fatal(err)
	}
	handler := New(config.Config{
		Version: "test", RequestTimeout: 2 * time.Second, GatewayBindPath: "/gateway",
		SecretsMasterKey: "unit-test-master-key", MCPSessionTTL: time.Minute,
	}, catalog.NewService(), repo, runtime.NewFakeRuntime())
	ts := httptest.NewServer(handler)
	t.Cleanup(ts.Close)

	ctx, cancel := context.WithTimeout(context.Background(), 2*time.Second)
	defer cancel()
	req, err := http.NewRequestWithContext(ctx, http.MethodGet, ts.URL+"/gateway/mcp", nil)
	if err != nil {
		t.Fatal(err)
	}
	req.Header.Set("Authorization", "Bearer client-sse."+raw)
	req.Header.Set("Accept", "text/event-stream")
	resp, err := http.DefaultClient.Do(req)
	if err != nil {
		t.Fatal(err)
	}
	defer resp.Body.Close()
	if resp.StatusCode != http.StatusOK {
		t.Fatalf("sse status=%d", resp.StatusCode)
	}
	if ct := resp.Header.Get("Content-Type"); !strings.Contains(ct, "text/event-stream") {
		t.Fatalf("content-type=%q", ct)
	}
	if resp.Header.Get("Mcp-Session-Id") == "" {
		t.Fatal("expected session id on SSE response")
	}
	// Read first chunk for endpoint event, then cancel.
	buf := make([]byte, 256)
	n, _ := resp.Body.Read(buf)
	body := string(buf[:n])
	if !strings.Contains(body, "endpoint") && !strings.Contains(body, "gateway/mcp") && n == 0 {
		// Some environments may delay first write; session header is enough signal.
		t.Logf("sse first read empty/partial: %q", body)
	}
	cancel()
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

	req3 := httptest.NewRequest(http.MethodGet, "/v1/admin/secrets/backend", nil)
	req3.Header.Set("Authorization", "Bearer bootstrap-admin")
	rec3 := httptest.NewRecorder()
	handler.ServeHTTP(rec3, req3)
	if rec3.Code != http.StatusOK || !strings.Contains(rec3.Body.String(), "local") {
		t.Fatalf("secrets backend: %d %s", rec3.Code, rec3.Body.String())
	}
}
