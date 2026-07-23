package store

import (
	"context"
	"path/filepath"
	"testing"
	"time"

	"github.com/mohvahedi/open-mcp-control-plane/internal/domain"
)

func TestSQLiteRepositoryMigrationAndCRUD(t *testing.T) {
	repo, err := OpenSQLite(filepath.Join(t.TempDir(), "controlplane.db"))
	if err != nil {
		t.Fatal(err)
	}
	defer repo.Close()

	ctx := context.Background()
	if err := repo.SetAdminHash(ctx, "hash"); err != nil {
		t.Fatal(err)
	}
	hash, err := repo.GetAdminHash(ctx)
	if err != nil || hash != "hash" {
		t.Fatalf("hash lookup failed: %v %q", err, hash)
	}

	plan := domain.DeploymentPlan{ID: "plan-1", Name: "safe", Image: "repo/img@sha256:abc", Transport: "streamable-http", EnvVarNames: []string{"A"}, CreatedAt: time.Now().UTC()}
	if _, err := repo.CreateDeploymentPlan(ctx, plan); err != nil {
		t.Fatal(err)
	}
	storedPlan, err := repo.GetDeploymentPlan(ctx, "plan-1")
	if err != nil {
		t.Fatal(err)
	}
	if storedPlan.Name != "safe" {
		t.Fatalf("unexpected plan: %+v", storedPlan)
	}

	if _, err := repo.CreateApproval(ctx, domain.Approval{ID: "ap-1", PlanID: "plan-1", Reason: "approved", CreatedAt: time.Now().UTC()}); err != nil {
		t.Fatal(err)
	}
	if _, ok, err := repo.GetApprovalByPlanID(ctx, "plan-1"); err != nil || !ok {
		t.Fatalf("approval missing: %v %v", err, ok)
	}

	if _, err := repo.CreateSecretReference(ctx, domain.SecretReference{ID: "sec-1", Name: "api-token", CreatedAt: time.Now().UTC()}, "opaque-value"); err != nil {
		t.Fatal(err)
	}
	refs, err := repo.ListSecretReferences(ctx)
	if err != nil {
		t.Fatal(err)
	}
	if len(refs) != 1 || refs[0].Name != "api-token" {
		t.Fatalf("unexpected refs: %+v", refs)
	}
}

func TestSecretValuesAreNotReturned(t *testing.T) {
	repo, err := OpenSQLite(filepath.Join(t.TempDir(), "controlplane.db"))
	if err != nil {
		t.Fatal(err)
	}
	defer repo.Close()

	ctx := context.Background()
	if _, err := repo.CreateSecretReference(ctx, domain.SecretReference{ID: "sec-1", Name: "db-pass", CreatedAt: time.Now().UTC()}, "super-secret"); err != nil {
		t.Fatal(err)
	}
	refs, err := repo.ListSecretReferences(ctx)
	if err != nil {
		t.Fatal(err)
	}
	if len(refs) != 1 {
		t.Fatalf("expected one ref, got %d", len(refs))
	}
	if refs[0].Description == "super-secret" {
		t.Fatal("secret value leaked through API")
	}
}
