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

func skillsTestHandler(t *testing.T) (http.Handler, string, store.Repository) {
	t.Helper()
	repo, err := store.OpenSQLite(filepath.Join(t.TempDir(), "skills.db"))
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { _ = repo.Close() })
	admin := "admin-skills-token"
	hash, err := security.HashToken(admin)
	if err != nil {
		t.Fatal(err)
	}
	if err := repo.SetAdminHash(context.Background(), hash); err != nil {
		t.Fatal(err)
	}
	h := New(config.Config{
		Version: "test", RequestTimeout: 2 * time.Second, SecretsMasterKey: "unit-test-master-key", UseFakeRuntime: true,
	}, catalog.NewService(), repo, runtime.NewFakeRuntime())
	return h, admin, repo
}

func TestSkillCreateListBind(t *testing.T) {
	handler, admin, repo := skillsTestHandler(t)
	now := time.Now().UTC()
	profile := domain.Profile{ID: "prof-1", Name: "ops", InstallationIDs: []string{}, ToolAllowlist: []string{}, CreatedAt: now}
	if _, err := repo.CreateProfile(context.Background(), profile); err != nil {
		t.Fatal(err)
	}

	body := `{"name":"Reviewer","kind":"prompt","content":"Be careful.","tags":["review"],"version":"1.0.0"}`
	req := httptest.NewRequest(http.MethodPost, "/v1/admin/skills", strings.NewReader(body))
	req.Header.Set("Authorization", "Bearer "+admin)
	req.Header.Set("Content-Type", "application/json")
	rr := httptest.NewRecorder()
	handler.ServeHTTP(rr, req)
	if rr.Code != http.StatusCreated {
		t.Fatalf("create skill: %d %s", rr.Code, rr.Body.String())
	}
	var skill domain.Skill
	if err := json.Unmarshal(rr.Body.Bytes(), &skill); err != nil {
		t.Fatal(err)
	}
	if skill.ID == "" || skill.Source != "local" {
		t.Fatalf("unexpected skill: %+v", skill)
	}

	req = httptest.NewRequest(http.MethodGet, "/v1/admin/skills", nil)
	req.Header.Set("Authorization", "Bearer "+admin)
	rr = httptest.NewRecorder()
	handler.ServeHTTP(rr, req)
	if rr.Code != http.StatusOK || !strings.Contains(rr.Body.String(), skill.ID) {
		t.Fatalf("list skills: %d %s", rr.Code, rr.Body.String())
	}

	bindBody := `{"profile_id":"prof-1","skill_id":"` + skill.ID + `"}`
	req = httptest.NewRequest(http.MethodPost, "/v1/admin/skill-bindings", strings.NewReader(bindBody))
	req.Header.Set("Authorization", "Bearer "+admin)
	req.Header.Set("Content-Type", "application/json")
	rr = httptest.NewRecorder()
	handler.ServeHTTP(rr, req)
	if rr.Code != http.StatusCreated {
		t.Fatalf("bind: %d %s", rr.Code, rr.Body.String())
	}
	var binding domain.SkillBinding
	if err := json.Unmarshal(rr.Body.Bytes(), &binding); err != nil {
		t.Fatal(err)
	}
	if !binding.Enabled || binding.ProfileID != "prof-1" {
		t.Fatalf("unexpected binding: %+v", binding)
	}

	req = httptest.NewRequest(http.MethodGet, "/v1/admin/skill-bindings?profile_id=prof-1", nil)
	req.Header.Set("Authorization", "Bearer "+admin)
	rr = httptest.NewRecorder()
	handler.ServeHTTP(rr, req)
	if rr.Code != http.StatusOK || !strings.Contains(rr.Body.String(), binding.ID) {
		t.Fatalf("list bindings: %d %s", rr.Code, rr.Body.String())
	}
}
