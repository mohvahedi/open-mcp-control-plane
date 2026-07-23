package catalog

import (
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"testing"
	"time"
)

func TestToolHiveSourceUsesSeedWhenRemoteDown(t *testing.T) {
	seed := Package{ID: "th-postgres", Name: "Postgres via ToolHive", Source: "toolhive", Tags: []string{"db"}}
	src := NewToolHiveSource("toolhive", ToolHiveConfig{
		BaseURL: "http://127.0.0.1:1", Timeout: 50 * time.Millisecond,
	}, seed)
	got, err := src.Search(context.Background(), "postgres")
	if err != nil {
		t.Fatal(err)
	}
	if len(got) != 1 || got[0].ID != "th-postgres" {
		t.Fatalf("expected seed package, got %#v", got)
	}
	st := src.Status(context.Background())
	if st.PackageCount != 1 {
		t.Fatalf("count=%d", st.PackageCount)
	}
}

func TestToolHiveSourceParsesRemoteCatalog(t *testing.T) {
	mux := http.NewServeMux()
	mux.HandleFunc("/catalog", func(w http.ResponseWriter, r *http.Request) {
		_ = json.NewEncoder(w).Encode(map[string]any{
			"items": []map[string]any{{
				"id": "remote-skill-mcp", "name": "Remote Skill MCP", "description": "from toolhive",
				"tags": []string{"remote"},
			}},
		})
	})
	srv := httptest.NewServer(mux)
	defer srv.Close()

	src := NewToolHiveSource("toolhive", ToolHiveConfig{BaseURL: srv.URL, Timeout: time.Second})
	got, err := src.Search(context.Background(), "remote")
	if err != nil {
		t.Fatal(err)
	}
	if len(got) != 1 || got[0].ID != "remote-skill-mcp" {
		t.Fatalf("unexpected %#v", got)
	}
	if src.Status(context.Background()).Healthy != true {
		t.Fatal("expected healthy")
	}
}
