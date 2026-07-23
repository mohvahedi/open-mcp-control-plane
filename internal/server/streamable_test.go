package server

import (
	"context"
	"encoding/json"
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

func TestStreamableHTTPInitializeToolsListAndCall(t *testing.T) {
	down := startMockDownstream(t,
		[]map[string]any{{"name": "echo", "description": "echo tool"}},
		map[string]any{"echo": map[string]any{"ok": true, "msg": "hi"}},
	)

	repo, err := store.OpenSQLite(filepath.Join(t.TempDir(), "stream.db"))
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { _ = repo.Close() })

	now := time.Now().UTC()
	inst := domain.Installation{
		ID: "inst-stream", PlanID: "plan-s", RuntimeRef: down.URL, State: "running",
		RollbackMetadata: map[string]string{}, CreatedAt: now, UpdatedAt: now,
	}
	if _, err := repo.CreateInstallation(context.Background(), inst); err != nil {
		t.Fatal(err)
	}
	profile := domain.Profile{
		ID: "profile-s", Name: "stream", InstallationIDs: []string{"inst-stream"},
		ToolAllowlist: []string{"inst-stream.echo"}, CreatedAt: now,
	}
	if _, err := repo.CreateProfile(context.Background(), profile); err != nil {
		t.Fatal(err)
	}
	raw := "stream-client-token"
	hash, err := security.HashToken(raw)
	if err != nil {
		t.Fatal(err)
	}
	if _, err := repo.CreateClient(context.Background(), domain.Client{
		ID: "client-s", ProfileID: "profile-s", Name: "stream-client", CreatedAt: now,
	}, hash); err != nil {
		t.Fatal(err)
	}

	handler := New(config.Config{
		Version: "test", RequestTimeout: 2 * time.Second, GatewayBindPath: "/gateway",
		SecretsMasterKey: "unit-test-master-key",
	}, catalog.NewService(), repo, runtime.NewFakeRuntime())

	auth := "Bearer client-s." + raw

	// initialize
	initBody := `{"jsonrpc":"2.0","id":1,"method":"initialize","params":{"protocolVersion":"2024-11-05","capabilities":{},"clientInfo":{"name":"test","version":"0"}}}`
	req := httptest.NewRequest(http.MethodPost, "/gateway/mcp", strings.NewReader(initBody))
	req.Header.Set("Authorization", auth)
	req.Header.Set("Content-Type", "application/json")
	rec := httptest.NewRecorder()
	handler.ServeHTTP(rec, req)
	if rec.Code != http.StatusOK {
		t.Fatalf("initialize status=%d body=%s", rec.Code, rec.Body.String())
	}
	if !strings.Contains(rec.Body.String(), "protocolVersion") || !strings.Contains(rec.Body.String(), "open-mcp-control-plane-gateway") {
		t.Fatalf("unexpected initialize body: %s", rec.Body.String())
	}

	// tools/list
	listBody := `{"jsonrpc":"2.0","id":2,"method":"tools/list","params":{}}`
	req = httptest.NewRequest(http.MethodPost, "/gateway/mcp", strings.NewReader(listBody))
	req.Header.Set("Authorization", auth)
	req.Header.Set("Content-Type", "application/json")
	rec = httptest.NewRecorder()
	handler.ServeHTTP(rec, req)
	if rec.Code != http.StatusOK || !strings.Contains(rec.Body.String(), "inst-stream.echo") {
		t.Fatalf("tools/list failed: %d %s", rec.Code, rec.Body.String())
	}

	// tools/call
	callBody := `{"jsonrpc":"2.0","id":3,"method":"tools/call","params":{"name":"inst-stream.echo","arguments":{"x":1}}}`
	req = httptest.NewRequest(http.MethodPost, "/gateway/mcp", strings.NewReader(callBody))
	req.Header.Set("Authorization", auth)
	req.Header.Set("Content-Type", "application/json")
	rec = httptest.NewRecorder()
	handler.ServeHTTP(rec, req)
	if rec.Code != http.StatusOK || !strings.Contains(rec.Body.String(), `"isError":false`) {
		t.Fatalf("tools/call failed: %d %s", rec.Code, rec.Body.String())
	}
	var rpc map[string]any
	if err := json.Unmarshal(rec.Body.Bytes(), &rpc); err != nil {
		t.Fatal(err)
	}
	if rpc["error"] != nil {
		t.Fatalf("unexpected rpc error: %v", rpc["error"])
	}

	// deny unlisted tool
	denyBody := `{"jsonrpc":"2.0","id":4,"method":"tools/call","params":{"name":"inst-stream.secret","arguments":{}}}`
	req = httptest.NewRequest(http.MethodPost, "/gateway/mcp", strings.NewReader(denyBody))
	req.Header.Set("Authorization", auth)
	req.Header.Set("Content-Type", "application/json")
	rec = httptest.NewRecorder()
	handler.ServeHTTP(rec, req)
	if rec.Code != http.StatusOK || !strings.Contains(rec.Body.String(), "tool not allowed") {
		t.Fatalf("expected tool not allowed, got %d %s", rec.Code, rec.Body.String())
	}

	// profile path alias
	req = httptest.NewRequest(http.MethodGet, "/mcp/profiles/stream", nil)
	req.Header.Set("Authorization", auth)
	rec = httptest.NewRecorder()
	handler.ServeHTTP(rec, req)
	if rec.Code != http.StatusOK || !strings.Contains(rec.Body.String(), "streamable-http") {
		t.Fatalf("profile get failed: %d %s", rec.Code, rec.Body.String())
	}
}

func TestStreamableHTTPRequiresClientAuth(t *testing.T) {
	handler := testServer(t)
	req := httptest.NewRequest(http.MethodPost, "/gateway/mcp", strings.NewReader(`{"jsonrpc":"2.0","id":1,"method":"ping"}`))
	req.Header.Set("Content-Type", "application/json")
	rec := httptest.NewRecorder()
	handler.ServeHTTP(rec, req)
	if rec.Code != http.StatusUnauthorized {
		t.Fatalf("expected 401, got %d %s", rec.Code, rec.Body.String())
	}
}
