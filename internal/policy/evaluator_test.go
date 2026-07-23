package policy

import (
	"testing"

	"github.com/mohvahedi/open-mcp-control-plane/internal/domain"
)

func TestEvaluateRequiresApprovalForHighRisk(t *testing.T) {
	plan := domain.DeploymentPlan{
		Name:        "risky",
		Image:       "example/image:latest",
		Privileged:  true,
		HostNetwork: true,
	}
	decision := Evaluate(plan)
	if !decision.RequiresApproval {
		t.Fatalf("expected approval requirement, got %#v", decision)
	}
	if len(decision.Findings) == 0 {
		t.Fatal("expected findings")
	}
}

func TestEvaluateSafePlan(t *testing.T) {
	plan := domain.DeploymentPlan{
		Name:  "safe",
		Image: "example/image@sha256:abc",
	}
	decision := Evaluate(plan)
	if decision.RequiresApproval {
		t.Fatalf("safe plan should not require approval: %#v", decision)
	}
}
