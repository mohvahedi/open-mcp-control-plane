package server

import (
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	"github.com/mohvahedi/open-mcp-control-plane/internal/catalog"
	"github.com/mohvahedi/open-mcp-control-plane/internal/config"
)

func TestHealth(t *testing.T) {
	handler := New(config.Config{Version: "test"}, catalog.NewService())
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
	source := catalog.NewStaticSource("test", []catalog.Package{{ID: "postgres", Name: "Postgres MCP"}})
	handler := New(config.Config{Version: "test"}, catalog.NewService(source))
	request := httptest.NewRequest(http.MethodGet, "/v1/catalog/search?q=postgres", nil)
	response := httptest.NewRecorder()
	handler.ServeHTTP(response, request)

	if response.Code != http.StatusOK || !strings.Contains(response.Body.String(), "postgres") {
		t.Fatalf("unexpected response: %d %s", response.Code, response.Body.String())
	}
}
