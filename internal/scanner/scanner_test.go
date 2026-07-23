package scanner

import (
	"context"
	"testing"
)

func TestScanImagePinnedSafe(t *testing.T) {
	s := NewHeuristic()
	r, err := s.ScanImage(context.Background(), "ghcr.io/org/mcp@sha256:abcdef0123456789")
	if err != nil {
		t.Fatal(err)
	}
	if !r.Pinned {
		t.Fatal("expected pinned")
	}
	if r.Score > 25 {
		t.Fatalf("expected low score for pinned image, got %d findings=%v", r.Score, r.Findings)
	}
	if r.Grade == "F" {
		t.Fatalf("unexpected grade %s", r.Grade)
	}
}

func TestScanImageLatestHighRisk(t *testing.T) {
	s := NewHeuristic()
	r, err := s.ScanImage(context.Background(), "example/mcp:latest")
	if err != nil {
		t.Fatal(err)
	}
	if r.Pinned {
		t.Fatal("expected unpinned")
	}
	if !r.RequiresGate {
		t.Fatal("latest should require gate")
	}
	if r.Score < 40 {
		t.Fatalf("expected elevated score, got %d", r.Score)
	}
}

func TestScorePackageSensitiveTools(t *testing.T) {
	s := NewHeuristic()
	r, err := s.ScorePackage(context.Background(), PackageInput{
		ID: "pkg-1", Name: "shell-mcp", Source: "bootstrap", License: "MIT", Version: "1.0.0",
		Tools: []ToolInput{{Name: "run_shell", Description: "execute shell commands"}},
	})
	if err != nil {
		t.Fatal(err)
	}
	if !r.RequiresGate {
		t.Fatalf("sensitive tools should gate: %#v", r)
	}
	found := false
	for _, f := range r.Findings {
		if f.Code == "sensitive_tool" {
			found = true
		}
	}
	if !found {
		t.Fatal("expected sensitive_tool finding")
	}
}

func TestScorePackageClean(t *testing.T) {
	s := NewHeuristic()
	r, err := s.ScorePackage(context.Background(), PackageInput{
		ID: "pkg-2", Name: "read-only", Source: "official", License: "Apache-2.0", Version: "2.1.0",
		Tools:      []ToolInput{{Name: "list_items", Description: "list resources"}},
		Provenance: map[string]any{"repository": "https://github.com/example/read-only"},
		Image:      "ghcr.io/example/read-only@sha256:deadbeef",
	})
	if err != nil {
		t.Fatal(err)
	}
	if r.RequiresGate {
		t.Fatalf("clean package should not gate: score=%d findings=%v", r.Score, r.Findings)
	}
	if r.Grade != "A" && r.Grade != "B" {
		t.Fatalf("expected A/B grade, got %s score=%d", r.Grade, r.Score)
	}
}
