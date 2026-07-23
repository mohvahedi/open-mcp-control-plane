package policy

import (
	"strings"

	"github.com/mohvahedi/open-mcp-control-plane/internal/domain"
	"github.com/mohvahedi/open-mcp-control-plane/internal/scanner"
)

type Decision struct {
	Findings         []domain.PolicyFinding `json:"findings"`
	RequiresApproval bool                   `json:"requires_approval"`
	Allowed          bool                   `json:"allowed"`
	RiskScore        int                    `json:"risk_score"`
	RiskGrade        string                 `json:"risk_grade"`
	ImageReport      *scanner.ImageReport   `json:"image_report,omitempty"`
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

	var imgReport *scanner.ImageReport
	if plan.Image != "" {
		r, err := scanner.NewHeuristic().ScanImage(nil, plan.Image)
		if err == nil {
			imgReport = &r
			for _, f := range r.Findings {
				// Avoid duplicate unpinned_image from both policy and scanner.
				if f.Code == "unpinned_image" {
					continue
				}
				add(f.Code, f.Severity, f.Message, f.Recommended)
			}
		}
	}

	requiresApproval := false
	for _, f := range findings {
		if f.Severity == "high" || f.Severity == "critical" {
			requiresApproval = true
			break
		}
	}
	score := scorePolicyFindings(findings)
	if imgReport != nil && imgReport.Score > score {
		score = imgReport.Score
	}
	if imgReport != nil && imgReport.RequiresGate {
		requiresApproval = true
	}

	return Decision{
		Findings:         findings,
		RequiresApproval: requiresApproval,
		Allowed:          !requiresApproval,
		RiskScore:        score,
		RiskGrade:        grade(score),
		ImageReport:      imgReport,
	}
}

func scorePolicyFindings(findings []domain.PolicyFinding) int {
	score := 0
	for _, f := range findings {
		switch f.Severity {
		case "critical":
			score += 40
		case "high":
			score += 25
		case "medium":
			score += 12
		case "low":
			score += 5
		default:
			score += 1
		}
	}
	if score > 100 {
		return 100
	}
	return score
}

func grade(score int) string {
	switch {
	case score <= 10:
		return "A"
	case score <= 25:
		return "B"
	case score <= 40:
		return "C"
	case score <= 60:
		return "D"
	default:
		return "F"
	}
}
