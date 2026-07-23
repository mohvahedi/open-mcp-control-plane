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

func startMockDownstream(t *testing.T, tools []map[string]any, invoke map[string]any) *httptest.Server {
	t.Helper()
	mux := http.NewServeMux()
	mux.HandleFunc("/tools", func(w http.ResponseWriter, r *http.Request) {
		writeJSON(w, http.StatusOK, map[string]any{"tools": tools})
	})
	mux.HandleFunc("/invoke", func(w http.ResponseWriter, r *http.Request) {
		var req struct {
			Tool string         `json:"tool"`
			Args map[string]any `json:"args"`
		}
		_ = json.NewDecoder(r.Body).Decode(&req)
		if out, ok := invoke[req.Tool]; ok {
			writeJSON(w, http.StatusOK, out)
			return
		}
		writeJSON(w, http.StatusNotFound, map[string]string{"error": "unknown tool"})
	})
	srv := httptest.NewServer(mux)
	t.Cleanup(srv.Close)
	return srv
}

func TestGatewayAggregatesTwoDownstreamServers(t *testing.T) {
	downA := startMockDownstream(t,
		[]map[string]any{{"name": "query", "description": "run sql"}},
		map[string]any{"query": map[string]any{"rows": []string{"ok-a"}}},
	)
	downB := startMockDownstream(t,
		[]map[string]any{{"name": "fetch", "description": "fetch url"}},
		map[string]any{"fetch": map[string]any{"status": 200}},
	)

	repo, err := store.OpenSQLite(filepath.Join(t.TempDir(), "gw.db"))
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { _ = repo.Close() })

	adminToken := "admin-test-token"
	hash, err := security.HashToken(adminToken)
	if err != nil {
		t.Fatal(err)
	}
	if err := repo.SetAdminHash(context.Background(), hash); err != nil {
		t.Fatal(err)
	}

	now := time.Now().UTC()
	instA := domain.Installation{ID: "inst-a", PlanID: "plan-a", RuntimeRef: downA.URL, State: "running", RollbackMetadata: map[string]string{}, CreatedAt: now, UpdatedAt: now}
	instB := domain.Installation{ID: "inst-b", PlanID: "plan-b", RuntimeRef: downB.URL, State: "running", RollbackMetadata: map[string]string{}, CreatedAt: now, UpdatedAt: now}
	if _, err := repo.CreateInstallation(context.Background(), instA); err != nil {
		t.Fatal(err)
	}
	if _, err := repo.CreateInstallation(context.Background(), instB); err != nil {
		t.Fatal(err)
	}

	profile := domain.Profile{
		ID:              "profile-1",
		Name:            "notion",
		InstallationIDs: []string{"inst-a", "inst-b"},
		ToolAllowlist:   []string{"inst-a.query", "inst-b.fetch"},
		CreatedAt:       now,
	}
	if _, err := repo.CreateProfile(context.Background(), profile); err != nil {
		t.Fatal(err)
	}

	rawClientToken := "client-secret-token"
	clientHash, err := security.HashToken(rawClientToken)
	if err != nil {
		t.Fatal(err)
	}
	client := domain.Client{ID: "client-1", ProfileID: "profile-1", Name: "notion", CreatedAt: now}
	if _, err := repo.CreateClient(context.Background(), client, clientHash); err != nil {
		t.Fatal(err)
	}

	handler := New(config.Config{
		Version:         "test",
		RequestTimeout:  2 * time.Second,
		GatewayBindPath: "/gateway",
	}, catalog.NewService(), repo, runtime.NewFakeRuntime())

	// list tools through gateway
	req := httptest.NewRequest(http.MethodGet, "/gateway/tools", nil)
	req.Header.Set("Authorization", "Bearer client-1."+rawClientToken)
	rec := httptest.NewRecorder()
	handler.ServeHTTP(rec, req)
	if rec.Code != http.StatusOK {
		t.Fatalf("tools status=%d body=%s", rec.Code, rec.Body.String())
	}
	body := rec.Body.String()
	if !strings.Contains(body, "inst-a.query") || !strings.Contains(body, "inst-b.fetch") {
		t.Fatalf("expected namespaced tools, got %s", body)
	}

	// invoke allowed tool
	invokeBody := `{"tool":"inst-a.query","args":{"q":"select 1"}}`
	req = httptest.NewRequest(http.MethodPost, "/gateway/invoke", strings.NewReader(invokeBody))
	req.Header.Set("Authorization", "Bearer client-1."+rawClientToken)
	req.Header.Set("Content-Type", "application/json")
	rec = httptest.NewRecorder()
	handler.ServeHTTP(rec, req)
	if rec.Code != http.StatusOK || !strings.Contains(rec.Body.String(), "ok-a") {
		t.Fatalf("invoke failed: %d %s", rec.Code, rec.Body.String())
	}

	// deny unlisted tool
	req = httptest.NewRequest(http.MethodPost, "/gateway/invoke", strings.NewReader(`{"tool":"inst-b.secret","args":{}}`))
	req.Header.Set("Authorization", "Bearer client-1."+rawClientToken)
	req.Header.Set("Content-Type", "application/json")
	rec = httptest.NewRecorder()
	handler.ServeHTTP(rec, req)
	if rec.Code != http.StatusForbidden {
		t.Fatalf("expected forbidden, got %d %s", rec.Code, rec.Body.String())
	}
}

func TestHighRiskPlanRequiresApprovalBeforeApply(t *testing.T) {
	repo, err := store.OpenSQLite(filepath.Join(t.TempDir(), "policy.db"))
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { _ = repo.Close() })

	adminToken := "admin-token"
	hash, _ := security.HashToken(adminToken)
	_ = repo.SetAdminHash(context.Background(), hash)

	handler := New(config.Config{Version: "test", RequestTimeout: 2 * time.Second}, catalog.NewService(), repo, runtime.NewFakeRuntime())

	planBody := `{"name":"risky","image":"example/img:latest","privileged":true,"host_network":true}`
	req := httptest.NewRequest(http.MethodPost, "/v1/admin/plans", strings.NewReader(planBody))
	req.Header.Set("Authorization", "Bearer "+adminToken)
	req.Header.Set("Content-Type", "application/json")
	rec := httptest.NewRecorder()
	handler.ServeHTTP(rec, req)
	if rec.Code != http.StatusCreated {
		t.Fatalf("create plan failed: %d %s", rec.Code, rec.Body.String())
	}
	var plan domain.DeploymentPlan
	if err := json.Unmarshal(rec.Body.Bytes(), &plan); err != nil {
		t.Fatal(err)
	}
	if !plan.RequiresApproval {
		t.Fatalf("expected requires_approval true: %+v", plan)
	}

	applyBody := `{"plan_id":"` + plan.ID + `"}`
	req = httptest.NewRequest(http.MethodPost, "/v1/admin/installations/apply", strings.NewReader(applyBody))
	req.Header.Set("Authorization", "Bearer "+adminToken)
	req.Header.Set("Content-Type", "application/json")
	rec = httptest.NewRecorder()
	handler.ServeHTTP(rec, req)
	if rec.Code != http.StatusForbidden {
		t.Fatalf("expected apply blocked without approval, got %d %s", rec.Code, rec.Body.String())
	}
}
