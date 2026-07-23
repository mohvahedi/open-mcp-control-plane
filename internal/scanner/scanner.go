package scanner

import (
	"context"
	"fmt"
	"strings"
	"time"
)

// Severity levels used across findings.
const (
	SeverityCritical = "critical"
	SeverityHigh     = "high"
	SeverityMedium   = "medium"
	SeverityLow      = "low"
	SeverityInfo     = "info"
)

// Finding is a single image or package risk signal.
type Finding struct {
	Code        string `json:"code"`
	Severity    string `json:"severity"`
	Message     string `json:"message"`
	Recommended string `json:"recommended,omitempty"`
	Source      string `json:"source,omitempty"`
}

// ImageReport is the result of scanning a container image reference.
type ImageReport struct {
	Image        string    `json:"image"`
	Digest       string    `json:"digest,omitempty"`
	Tag          string    `json:"tag,omitempty"`
	Pinned       bool      `json:"pinned"`
	Score        int       `json:"score"` // 0 (safe) .. 100 (critical)
	Grade        string    `json:"grade"` // A-F
	Findings     []Finding `json:"findings"`
	ScannedAt    time.Time `json:"scanned_at"`
	Scanner      string    `json:"scanner"`
	RequiresGate bool      `json:"requires_gate"`
}

// PackageReport scores a catalog package for operational risk.
type PackageReport struct {
	PackageID    string    `json:"package_id"`
	Name         string    `json:"name"`
	Score        int       `json:"score"`
	Grade        string    `json:"grade"`
	Findings     []Finding `json:"findings"`
	ScannedAt    time.Time `json:"scanned_at"`
	RequiresGate bool      `json:"requires_gate"`
}

// Scanner performs static/heuristic image and package risk analysis.
// When a real CVE scanner (e.g. Trivy) is unavailable, HeuristicScanner still
// produces actionable supply-chain signals without external dependencies.
type Scanner interface {
	ScanImage(ctx context.Context, image string) (ImageReport, error)
	ScorePackage(ctx context.Context, pkg PackageInput) (PackageReport, error)
}

type PackageInput struct {
	ID          string
	Name        string
	Description string
	Version     string
	Source      string
	Runtime     string
	License     string
	Tags        []string
	Tools       []ToolInput
	Image       string
	Provenance  map[string]any
}

type ToolInput struct {
	Name        string
	Description string
	Operations  []string
}

// HeuristicScanner is the default offline scanner.
type HeuristicScanner struct{}

func NewHeuristic() *HeuristicScanner { return &HeuristicScanner{} }

func (h *HeuristicScanner) ScanImage(_ context.Context, image string) (ImageReport, error) {
	image = strings.TrimSpace(image)
	if image == "" {
		return ImageReport{}, fmt.Errorf("image is required")
	}
	report := ImageReport{
		Image:     image,
		ScannedAt: time.Now().UTC(),
		Scanner:   "heuristic-v1",
	}

	// Parse digest / tag.
	if i := strings.Index(image, "@sha256:"); i >= 0 {
		report.Pinned = true
		report.Digest = image[i+1:] // sha256:...
		base := image[:i]
		if j := strings.LastIndex(base, ":"); j > strings.LastIndex(base, "/") {
			report.Tag = base[j+1:]
		}
	} else if j := strings.LastIndex(image, ":"); j > strings.LastIndex(image, "/") {
		report.Tag = image[j+1:]
	} else {
		report.Tag = "latest" // implicit
	}

	add := func(code, sev, msg, rec string) {
		report.Findings = append(report.Findings, Finding{
			Code: code, Severity: sev, Message: msg, Recommended: rec, Source: "image",
		})
	}

	if !report.Pinned {
		sev := SeverityMedium
		if report.Tag == "" || report.Tag == "latest" || report.Tag == "main" || report.Tag == "master" {
			sev = SeverityHigh
		}
		add("unpinned_image", sev, "Image reference is not pinned by immutable digest", "Pin with image@sha256:...")
	}
	if !report.Pinned && (report.Tag == "latest" || report.Tag == "") {
		add("mutable_tag", SeverityHigh, "Using mutable tag 'latest' (or implicit latest)", "Use a version tag and digest pin")
	}
	lower := strings.ToLower(image)
	if strings.HasPrefix(lower, "http://") || strings.Contains(lower, "://") {
		add("non_registry_ref", SeverityHigh, "Image does not look like a registry reference", "Use registry/repo:tag or @digest")
	}
	// Public / untrusted registries heuristics
	if strings.HasPrefix(lower, "docker.io/") || (!strings.Contains(image, "/") && !strings.Contains(image, ".")) {
		add("docker_hub_default", SeverityLow, "Image resolves via Docker Hub default namespace", "Prefer an explicit org/registry path")
	}
	if strings.Contains(lower, "localhost") || strings.Contains(lower, "127.0.0.1") {
		add("local_registry", SeverityMedium, "Image points at a local registry", "Ensure the local registry is trusted and scanned")
	}
	// Suspicious path segments
	for _, bad := range []string{"/scratch/", "/temp/", "/tmp/", "unofficial", "unknown"} {
		if strings.Contains(lower, bad) {
			add("suspicious_path", SeverityMedium, "Image path contains suspicious segment: "+bad, "Verify publisher reputation")
			break
		}
	}
	// Privileged-sounding image names
	for _, bad := range []string{"dind", "docker-in-docker", "privileged", "rootful"} {
		if strings.Contains(lower, bad) {
			add("privileged_image_name", SeverityHigh, "Image name suggests elevated privileges: "+bad, "Avoid privileged runtimes for MCP tools")
			break
		}
	}

	report.Score = scoreFindings(report.Findings)
	report.Grade = gradeForScore(report.Score)
	report.RequiresGate = report.Score >= 40 || hasSeverity(report.Findings, SeverityHigh, SeverityCritical)
	return report, nil
}

func (h *HeuristicScanner) ScorePackage(_ context.Context, pkg PackageInput) (PackageReport, error) {
	if pkg.ID == "" && pkg.Name == "" {
		return PackageReport{}, fmt.Errorf("package id or name is required")
	}
	report := PackageReport{
		PackageID: pkg.ID,
		Name:      pkg.Name,
		ScannedAt: time.Now().UTC(),
	}
	add := func(code, sev, msg, rec string) {
		report.Findings = append(report.Findings, Finding{
			Code: code, Severity: sev, Message: msg, Recommended: rec, Source: "package",
		})
	}

	if pkg.License == "" {
		add("missing_license", SeverityMedium, "Package has no declared license", "Prefer packages with an OSI-approved license")
	} else {
		lic := strings.ToLower(pkg.License)
		if strings.Contains(lic, "proprietary") || strings.Contains(lic, "unknown") || lic == "none" {
			add("restrictive_license", SeverityMedium, "License may restrict redistribution: "+pkg.License, "Review license before enterprise use")
		}
	}
	if pkg.Version == "" || pkg.Version == "latest" || pkg.Version == "dev" {
		add("unstable_version", SeverityMedium, "Package version is missing or unstable", "Pin a released semantic version")
	}
	if pkg.Source == "" {
		add("unknown_source", SeverityHigh, "Package source/publisher is unknown", "Only install from trusted catalog sources")
	}
	// Tool risk
	for _, tool := range pkg.Tools {
		name := strings.ToLower(tool.Name + " " + tool.Description + " " + strings.Join(tool.Operations, " "))
		for _, kw := range []string{"shell", "exec", "delete", "remove", "write", "sudo", "root", "filesystem", "ssh", "credential", "secret", "token"} {
			if strings.Contains(name, kw) {
				add("sensitive_tool", SeverityHigh, "Tool may perform sensitive operations ("+kw+"): "+tool.Name, "Allowlist only required tools in profiles")
				break
			}
		}
	}
	// Tags
	for _, tag := range pkg.Tags {
		t := strings.ToLower(tag)
		if t == "experimental" || t == "alpha" || t == "beta" || t == "unofficial" {
			add("maturity_tag", SeverityMedium, "Package tagged as "+tag, "Prefer stable packages for production profiles")
		}
	}
	// Runtime hints
	rt := strings.ToLower(pkg.Runtime)
	if strings.Contains(rt, "host") || strings.Contains(rt, "privileged") {
		add("host_runtime", SeverityHigh, "Runtime suggests host-level access", "Run inside isolated containers")
	}
	// Provenance
	if len(pkg.Provenance) == 0 {
		add("missing_provenance", SeverityLow, "No provenance metadata attached", "Record publisher, repo URL, and signature when available")
	} else {
		if _, ok := pkg.Provenance["repository"]; !ok {
			if _, ok2 := pkg.Provenance["repo"]; !ok2 {
				add("weak_provenance", SeverityLow, "Provenance lacks repository URL", "Add source repository to provenance")
			}
		}
	}

	// Optional nested image scan
	if pkg.Image != "" {
		img, err := h.ScanImage(context.Background(), pkg.Image)
		if err == nil {
			for _, f := range img.Findings {
				report.Findings = append(report.Findings, f)
			}
		}
	}

	report.Score = scoreFindings(report.Findings)
	report.Grade = gradeForScore(report.Score)
	report.RequiresGate = report.Score >= 40 || hasSeverity(report.Findings, SeverityHigh, SeverityCritical)
	return report, nil
}

func scoreFindings(findings []Finding) int {
	score := 0
	for _, f := range findings {
		switch f.Severity {
		case SeverityCritical:
			score += 40
		case SeverityHigh:
			score += 25
		case SeverityMedium:
			score += 12
		case SeverityLow:
			score += 5
		case SeverityInfo:
			score += 1
		}
	}
	if score > 100 {
		score = 100
	}
	return score
}

func gradeForScore(score int) string {
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

func hasSeverity(findings []Finding, levels ...string) bool {
	set := map[string]struct{}{}
	for _, l := range levels {
		set[l] = struct{}{}
	}
	for _, f := range findings {
		if _, ok := set[f.Severity]; ok {
			return true
		}
	}
	return false
}
