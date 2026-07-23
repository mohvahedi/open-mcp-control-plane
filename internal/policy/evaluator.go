package policy

import (
	"strings"

	"github.com/mohvahedi/open-mcp-control-plane/internal/domain"
)

type Decision struct {
	Findings         []domain.PolicyFinding `json:"findings"`
	RequiresApproval bool                   `json:"requires_approval"`
	Allowed          bool                   `json:"allowed"`
}

func Evaluate(plan domain.DeploymentPlan) Decision {
	var findings []domain.PolicyFinding
	add := func(code, severity, msg, recommendation string) {
		findings = append(findings, domain.PolicyFinding{Code: code, Severity: severity, Message: msg, Recommended: recommendation})
	}

	if plan.Privileged {
		add("privileged_container", "high", "Privileged container requested", "Run as non-root with dropped capabilities")
	}
	if plan.HostNetwork {
		add("host_network", "high", "Host network requested", "Use an isolated bridge network")
	}
	if len(plan.HostMounts) > 0 {
		add("host_mounts", "high", "Host mounts requested", "Use named volumes or no host mounts")
	}
	if plan.Image != "" && !strings.Contains(plan.Image, "@sha256:") {
		add("unpinned_image", "medium", "Image is not pinned by immutable digest", "Use image@sha256:digest")
	}
	if plan.BroadEgress {
		add("broad_egress", "high", "Broad egress access requested", "Restrict egress to required destinations")
	}
	if plan.ExposePublic {
		add("public_exposure", "high", "Public exposure requested", "Keep service private and front with a controlled gateway")
	}
	for _, tool := range plan.RequestedTools {
		name := strings.ToLower(tool.Name)
		if strings.Contains(name, "write") || strings.Contains(name, "delete") || strings.Contains(name, "remove") || strings.Contains(name, "exec") || strings.Contains(name, "shell") {
			add("destructive_tool", "high", "Potentially destructive tool requested: "+tool.Name, "Use read-only tooling where possible")
		}
	}

	requiresApproval := false
	for _, f := range findings {
		if f.Severity == "high" {
			requiresApproval = true
			break
		}
	}

	return Decision{
		Findings:         findings,
		RequiresApproval: requiresApproval,
		Allowed:          !requiresApproval,
	}
}
