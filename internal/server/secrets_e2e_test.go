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

func TestSecretCreateListsOpaqueAndRoundTripsCipher(t *testing.T) {
	repo, err := store.OpenSQLite(filepath.Join(t.TempDir(), "sec.db"))
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { _ = repo.Close() })

	adminToken := "admin-secret-token"
	hash, err := security.HashToken(adminToken)
	if err != nil {
		t.Fatal(err)
	}
	if err := repo.SetAdminHash(context.Background(), hash); err != nil {
		t.Fatal(err)
	}

	handler := New(config.Config{
		Version:          "test",
		RequestTimeout:   2 * time.Second,
		SecretsMasterKey: "unit-test-master-key",
	}, catalog.NewService(), repo, runtime.NewFakeRuntime())

	body := `{"name":"api-token","description":"demo","value":"super-secret-value"}`
	req := httptest.NewRequest(http.MethodPost, "/v1/admin/secrets", strings.NewReader(body))
	req.Header.Set("Authorization", "Bearer "+adminToken)
	req.Header.Set("Content-Type", "application/json")
	rec := httptest.NewRecorder()
	handler.ServeHTTP(rec, req)
	if rec.Code != http.StatusCreated {
		t.Fatalf("create secret failed: %d %s", rec.Code, rec.Body.String())
	}
	if strings.Contains(rec.Body.String(), "super-secret-value") {
		t.Fatal("plaintext secret leaked in create response")
	}
	var created domain.SecretReference
	if err := json.Unmarshal(rec.Body.Bytes(), &created); err != nil {
		t.Fatal(err)
	}
	if created.ID == "" || created.Name != "api-token" {
		t.Fatalf("unexpected secret ref: %+v", created)
	}

	req = httptest.NewRequest(http.MethodGet, "/v1/admin/secrets", nil)
	req.Header.Set("Authorization", "Bearer "+adminToken)
	rec = httptest.NewRecorder()
	handler.ServeHTTP(rec, req)
	if rec.Code != http.StatusOK {
		t.Fatalf("list secrets failed: %d %s", rec.Code, rec.Body.String())
	}
	if strings.Contains(rec.Body.String(), "super-secret-value") {
		t.Fatal("plaintext secret leaked in list response")
	}

	cipher, err := repo.GetSecretCipher(context.Background(), created.ID)
	if err != nil {
		t.Fatal(err)
	}
	key, err := security.DeriveKey("unit-test-master-key")
	if err != nil {
		t.Fatal(err)
	}
	plain, err := security.DecryptSecret(key, cipher)
	if err != nil {
		t.Fatal(err)
	}
	if plain != "super-secret-value" {
		t.Fatalf("round-trip failed: %q", plain)
	}
}

func TestSafePlanApplyE2EWithFakeRuntime(t *testing.T) {
	repo, err := store.OpenSQLite(filepath.Join(t.TempDir(), "e2e.db"))
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { _ = repo.Close() })

	adminToken := "admin-e2e-token"
	hash, _ := security.HashToken(adminToken)
	_ = repo.SetAdminHash(context.Background(), hash)

	handler := New(config.Config{
		Version:          "test",
		RequestTimeout:   2 * time.Second,
		SecretsMasterKey: "unit-test-master-key",
		UseFakeRuntime:   true,
	}, catalog.NewService(), repo, runtime.NewFakeRuntime())

	planBody := `{"name":"safe-demo","image":"example/mcp@sha256:abc123","transport":"streamable-http"}`
	req := httptest.NewRequest(http.MethodPost, "/v1/admin/plans", strings.NewReader(planBody))
	req.Header.Set("Authorization", "Bearer "+adminToken)
	req.Header.Set("Content-Type", "application/json")
	rec := httptest.NewRecorder()
	handler.ServeHTTP(rec, req)
	if rec.Code != http.StatusCreated {
		t.Fatalf("create plan: %d %s", rec.Code, rec.Body.String())
	}
	var plan domain.DeploymentPlan
	if err := json.Unmarshal(rec.Body.Bytes(), &plan); err != nil {
		t.Fatal(err)
	}
	if plan.RequiresApproval {
		t.Fatalf("safe plan should not require approval: %+v", plan)
	}

	applyBody := `{"plan_id":"` + plan.ID + `"}`
	req = httptest.NewRequest(http.MethodPost, "/v1/admin/installations/apply", strings.NewReader(applyBody))
	req.Header.Set("Authorization", "Bearer "+adminToken)
	req.Header.Set("Content-Type", "application/json")
	rec = httptest.NewRecorder()
	handler.ServeHTTP(rec, req)
	if rec.Code != http.StatusCreated {
		t.Fatalf("apply plan: %d %s", rec.Code, rec.Body.String())
	}
	var inst domain.Installation
	if err := json.Unmarshal(rec.Body.Bytes(), &inst); err != nil {
		t.Fatal(err)
	}
	if inst.State != "running" || inst.RuntimeRef == "" {
		t.Fatalf("unexpected installation: %+v", inst)
	}

	req = httptest.NewRequest(http.MethodPost, "/v1/admin/profiles", strings.NewReader(`{"name":"e2e"}`))
	req.Header.Set("Authorization", "Bearer "+adminToken)
	req.Header.Set("Content-Type", "application/json")
	rec = httptest.NewRecorder()
	handler.ServeHTTP(rec, req)
	if rec.Code != http.StatusCreated {
		t.Fatalf("create profile: %d %s", rec.Code, rec.Body.String())
	}
	var profile domain.Profile
	_ = json.Unmarshal(rec.Body.Bytes(), &profile)

	body := `{"installation_ids":["` + inst.ID + `"]}`
	req = httptest.NewRequest(http.MethodPost, "/v1/admin/profiles/"+profile.ID+"/installations", strings.NewReader(body))
	req.Header.Set("Authorization", "Bearer "+adminToken)
	req.Header.Set("Content-Type", "application/json")
	rec = httptest.NewRecorder()
	handler.ServeHTTP(rec, req)
	if rec.Code != http.StatusOK {
		t.Fatalf("set installations: %d %s", rec.Code, rec.Body.String())
	}

	req = httptest.NewRequest(http.MethodGet, "/v1/admin/gateway/status", nil)
	req.Header.Set("Authorization", "Bearer "+adminToken)
	rec = httptest.NewRecorder()
	handler.ServeHTTP(rec, req)
	if rec.Code != http.StatusOK || !strings.Contains(rec.Body.String(), "installations") {
		t.Fatalf("gateway status: %d %s", rec.Code, rec.Body.String())
	}
}
