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
	"github.com/mohvahedi/open-mcp-control-plane/internal/runtime"
	"github.com/mohvahedi/open-mcp-control-plane/internal/security"
	"github.com/mohvahedi/open-mcp-control-plane/internal/store"
)

func scanTestHandler(t *testing.T) (http.Handler, string) {
	t.Helper()
	repo, err := store.OpenSQLite(filepath.Join(t.TempDir(), "scan.db"))
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { _ = repo.Close() })
	admin := "admin-scan-token"
	hash, err := security.HashToken(admin)
	if err != nil {
		t.Fatal(err)
	}
	if err := repo.SetAdminHash(context.Background(), hash); err != nil {
		t.Fatal(err)
	}
	pkg := catalog.Package{
		ID: "pkg-readonly", Name: "Read Only MCP", Source: "test", License: "Apache-2.0", Version: "1.0.0",
		Tools:      []catalog.ToolMetadata{{Name: "list_items", Description: "list resources"}},
		Provenance: map[string]any{"repository": "https://github.com/example/readonly", "image": "ghcr.io/example/ro@sha256:abc"},
	}
	h := New(config.Config{
		Version: "test", RequestTimeout: 2 * time.Second, SecretsMasterKey: "unit-test-master-key", UseFakeRuntime: true,
	}, catalog.NewService(catalog.NewStaticSource("test", []catalog.Package{pkg})), repo, runtime.NewFakeRuntime())
	return h, admin
}

func TestScanImageEndpoint(t *testing.T) {
	handler, admin := scanTestHandler(t)
	body := `{"image":"example/mcp:latest"}`
	req := httptest.NewRequest(http.MethodPost, "/v1/admin/scan/image", strings.NewReader(body))
	req.Header.Set("Authorization", "Bearer "+admin)
	req.Header.Set("Content-Type", "application/json")
	rr := httptest.NewRecorder()
	handler.ServeHTTP(rr, req)
	if rr.Code != http.StatusOK {
		t.Fatalf("status %d body %s", rr.Code, rr.Body.String())
	}
	var report map[string]any
	if err := json.Unmarshal(rr.Body.Bytes(), &report); err != nil {
		t.Fatal(err)
	}
	if report["image"] != "example/mcp:latest" {
		t.Fatalf("unexpected report: %#v", report)
	}
	score, _ := report["score"].(float64)
	if score < 40 {
		t.Fatalf("expected elevated score for latest tag, got %v", report["score"])
	}
}

func TestPackageRiskEndpoint(t *testing.T) {
	handler, _ := scanTestHandler(t)
	req := httptest.NewRequest(http.MethodGet, "/v1/catalog/packages/pkg-readonly/risk", nil)
	rr := httptest.NewRecorder()
	handler.ServeHTTP(rr, req)
	if rr.Code != http.StatusOK {
		t.Fatalf("risk status %d body %s", rr.Code, rr.Body.String())
	}
	var out map[string]any
	if err := json.Unmarshal(rr.Body.Bytes(), &out); err != nil {
		t.Fatal(err)
	}
	if _, ok := out["risk"]; !ok {
		t.Fatalf("missing risk: %#v", out)
	}
}

func TestCreatePlanAttachesScannerFindings(t *testing.T) {
	handler, admin := scanTestHandler(t)
	body := `{"name":"scan-plan","image":"example/dind:latest","privileged":false}`
	req := httptest.NewRequest(http.MethodPost, "/v1/admin/plans", strings.NewReader(body))
	req.Header.Set("Authorization", "Bearer "+admin)
	req.Header.Set("Content-Type", "application/json")
	rr := httptest.NewRecorder()
	handler.ServeHTTP(rr, req)
	if rr.Code != http.StatusCreated {
		t.Fatalf("status %d body %s", rr.Code, rr.Body.String())
	}
	var plan map[string]any
	if err := json.Unmarshal(rr.Body.Bytes(), &plan); err != nil {
		t.Fatal(err)
	}
	if plan["requires_approval"] != true {
		t.Fatalf("dind latest should require approval: %#v", plan)
	}
	findings, _ := plan["risk_findings"].([]any)
	if len(findings) == 0 {
		t.Fatalf("expected scanner/policy findings: %#v", plan)
	}
}
