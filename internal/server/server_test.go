package server

import (
	"net/http"
	"net/http/httptest"
	"path/filepath"
	"strings"
	"testing"
	"time"

	"github.com/mohvahedi/open-mcp-control-plane/internal/catalog"
	"github.com/mohvahedi/open-mcp-control-plane/internal/config"
	"github.com/mohvahedi/open-mcp-control-plane/internal/runtime"
	"github.com/mohvahedi/open-mcp-control-plane/internal/store"
)

func testServer(t *testing.T, packages ...catalog.Package) http.Handler {
	t.Helper()
	repo, err := store.OpenSQLite(filepath.Join(t.TempDir(), "test.db"))
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { _ = repo.Close() })
	source := catalog.NewStaticSource("test", packages)
	cfg := config.Config{
		Version:          "test",
		RequestTimeout:   2 * time.Second,
		GatewayBindPath:  "/gateway",
		SecretsMasterKey: "unit-test-master-key",
	}
	return New(cfg, catalog.NewService(source), repo, runtime.NewFakeRuntime())
}

func TestHealth(t *testing.T) {
	handler := testServer(t)
	request := httptest.NewRequest(http.MethodGet, "/healthz", nil)
	response := httptest.NewRecorder()
	handler.ServeHTTP(response, request)

	if response.Code != http.StatusOK {
		t.Fatalf("expected 200, got %d", response.Code)
	}
	if !strings.Contains(response.Body.String(), `"status":"ok"`) {
		t.Fatalf("unexpected body: %s", response.Body.String())
	}
}

func TestCatalogSearch(t *testing.T) {
	handler := testServer(t, catalog.Package{ID: "postgres", Name: "Postgres MCP", Source: "test"})
	request := httptest.NewRequest(http.MethodGet, "/v1/catalog/search?q=postgres", nil)
	response := httptest.NewRecorder()
	handler.ServeHTTP(response, request)

	if response.Code != http.StatusOK || !strings.Contains(response.Body.String(), "postgres") {
		t.Fatalf("unexpected response: %d %s", response.Code, response.Body.String())
	}
}

func TestReadyz(t *testing.T) {
	handler := testServer(t)
	request := httptest.NewRequest(http.MethodGet, "/readyz", nil)
	response := httptest.NewRecorder()
	handler.ServeHTTP(response, request)
	if response.Code != http.StatusOK {
		t.Fatalf("expected 200, got %d body=%s", response.Code, response.Body.String())
	}
}
