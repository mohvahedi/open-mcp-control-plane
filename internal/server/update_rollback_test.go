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

func TestInstallationUpdateAndRollback(t *testing.T) {
	repo, err := store.OpenSQLite(filepath.Join(t.TempDir(), "upd.db"))
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { _ = repo.Close() })

	admin := "admin-upd"
	hash, _ := security.HashToken(admin)
	_ = repo.SetAdminHash(context.Background(), hash)

	now := time.Now().UTC()
	plan := domain.DeploymentPlan{
		ID: "plan-upd", Name: "svc", Image: "example/mcp@sha256:aaa", Transport: "streamable-http",
		EnvVarNames: []string{}, SecretReferences: []string{}, HostMounts: []string{},
		RequestedTools: nil, RiskFindings: nil, Labels: map[string]string{}, CreatedAt: now,
	}
	if _, err := repo.CreateDeploymentPlan(context.Background(), plan); err != nil {
		t.Fatal(err)
	}

	rt := runtime.NewFakeRuntime()
	ref, err := rt.Install(context.Background(), runtime.InstallSpec{Name: "svc", Image: plan.Image})
	if err != nil {
		t.Fatal(err)
	}
	inst := domain.Installation{
		ID: "inst-upd", PlanID: plan.ID, RuntimeRef: ref, State: "running",
		ImageDigest: "aaa", RollbackMetadata: map[string]string{}, CreatedAt: now, UpdatedAt: now,
	}
	if _, err := repo.CreateInstallation(context.Background(), inst); err != nil {
		t.Fatal(err)
	}

	handler := New(config.Config{
		Version: "test", RequestTimeout: 2 * time.Second, SecretsMasterKey: "unit-test-master-key", UseFakeRuntime: true,
	}, catalog.NewService(), repo, rt)

	// update to new image
	body := `{"image":"example/mcp@sha256:bbb"}`
	req := httptest.NewRequest(http.MethodPost, "/v1/admin/installations/inst-upd/update", strings.NewReader(body))
	req.Header.Set("Authorization", "Bearer "+admin)
	req.Header.Set("Content-Type", "application/json")
	rec := httptest.NewRecorder()
	handler.ServeHTTP(rec, req)
	if rec.Code != http.StatusOK {
		t.Fatalf("update failed: %d %s", rec.Code, rec.Body.String())
	}
	var updated domain.Installation
	if err := json.Unmarshal(rec.Body.Bytes(), &updated); err != nil {
		t.Fatal(err)
	}
	if updated.RuntimeRef == ref {
		t.Fatalf("expected new runtime ref, still %s", updated.RuntimeRef)
	}
	if updated.RollbackMetadata["previous_runtime_ref"] != ref {
		t.Fatalf("missing previous_runtime_ref: %+v", updated.RollbackMetadata)
	}
	if updated.ImageDigest != "bbb" {
		t.Fatalf("digest not updated: %q", updated.ImageDigest)
	}

	// rollback
	req = httptest.NewRequest(http.MethodPost, "/v1/admin/installations/inst-upd/rollback", nil)
	req.Header.Set("Authorization", "Bearer "+admin)
	rec = httptest.NewRecorder()
	handler.ServeHTTP(rec, req)
	if rec.Code != http.StatusOK {
		t.Fatalf("rollback failed: %d %s", rec.Code, rec.Body.String())
	}
	var rolled domain.Installation
	if err := json.Unmarshal(rec.Body.Bytes(), &rolled); err != nil {
		t.Fatal(err)
	}
	if rolled.RuntimeRef != ref {
		t.Fatalf("expected rollback to %s, got %s", ref, rolled.RuntimeRef)
	}
	if rolled.State != "running" {
		t.Fatalf("state=%s", rolled.State)
	}
}

func TestInstallationRollbackWithoutTarget(t *testing.T) {
	repo, err := store.OpenSQLite(filepath.Join(t.TempDir(), "rb.db"))
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { _ = repo.Close() })
	admin := "admin-rb"
	hash, _ := security.HashToken(admin)
	_ = repo.SetAdminHash(context.Background(), hash)
	now := time.Now().UTC()
	_, _ = repo.CreateInstallation(context.Background(), domain.Installation{
		ID: "inst-x", PlanID: "p", RuntimeRef: "fake-x", State: "running",
		RollbackMetadata: map[string]string{}, CreatedAt: now, UpdatedAt: now,
	})
	handler := New(config.Config{Version: "test", RequestTimeout: 2 * time.Second, SecretsMasterKey: "k"}, catalog.NewService(), repo, runtime.NewFakeRuntime())
	req := httptest.NewRequest(http.MethodPost, "/v1/admin/installations/inst-x/rollback", nil)
	req.Header.Set("Authorization", "Bearer "+admin)
	rec := httptest.NewRecorder()
	handler.ServeHTTP(rec, req)
	if rec.Code != http.StatusBadRequest {
		t.Fatalf("expected 400, got %d %s", rec.Code, rec.Body.String())
	}
}
