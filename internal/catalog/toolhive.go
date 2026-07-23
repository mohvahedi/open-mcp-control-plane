package catalog

import (
	"context"
	"encoding/json"
	"fmt"
	"net/http"
	"strings"
	"time"
)

// ToolHiveConfig configures a ToolHive / community catalog federation source.
type ToolHiveConfig struct {
	BaseURL    string
	Timeout    time.Duration
	HTTPClient *http.Client
}

// ToolHiveSource federates packages from a ToolHive-compatible catalog endpoint.
// When the remote is unavailable it degrades gracefully to the seed catalog.
type ToolHiveSource struct {
	name        string
	baseURL     string
	timeout     time.Duration
	httpClient  *http.Client
	lastErr     string
	lastChecked time.Time
	count       int
	seed        []Package
}

func NewToolHiveSource(name string, cfg ToolHiveConfig, seed ...Package) *ToolHiveSource {
	if name == "" {
		name = "toolhive"
	}
	if cfg.BaseURL == "" {
		cfg.BaseURL = "https://toolhive.dev/api/v1"
	}
	if cfg.Timeout <= 0 {
		cfg.Timeout = 5 * time.Second
	}
	if cfg.HTTPClient == nil {
		cfg.HTTPClient = &http.Client{Timeout: cfg.Timeout}
	}
	return &ToolHiveSource{
		name:       name,
		baseURL:    strings.TrimRight(cfg.BaseURL, "/"),
		timeout:    cfg.Timeout,
		httpClient: cfg.HTTPClient,
		seed:       append([]Package(nil), seed...),
		count:      len(seed),
	}
}

func (t *ToolHiveSource) Name() string { return t.name }

func (t *ToolHiveSource) Search(ctx context.Context, query string) ([]Package, error) {
	pkgs, _ := t.fetch(ctx)
	return filterPackages(pkgs, query), nil
}

func (t *ToolHiveSource) GetPackage(ctx context.Context, id string) (Package, error) {
	pkgs, _ := t.fetch(ctx)
	for _, p := range pkgs {
		if p.ID == id {
			return p, nil
		}
	}
	return Package{}, ErrPackageNotFound
}

func (t *ToolHiveSource) Status(_ context.Context) SourceStatus {
	return SourceStatus{
		Source:       t.name,
		Healthy:      t.lastErr == "",
		LastError:    t.lastErr,
		LastChecked:  t.lastChecked,
		PackageCount: t.count,
	}
}

func (t *ToolHiveSource) fetch(ctx context.Context) ([]Package, error) {
	ctx, cancel := context.WithTimeout(ctx, t.timeout)
	defer cancel()

	endpoints := []string{
		t.baseURL + "/catalog",
		t.baseURL + "/packages",
		t.baseURL + "/v1/catalog",
	}

	var last error
	for _, ep := range endpoints {
		req, err := http.NewRequestWithContext(ctx, http.MethodGet, ep, nil)
		if err != nil {
			last = err
			continue
		}
		resp, err := t.httpClient.Do(req)
		t.lastChecked = time.Now().UTC()
		if err != nil {
			last = err
			continue
		}

		pkgs, err := decodeToolHiveResponse(resp, t.name)
		_ = resp.Body.Close()
		if err != nil {
			last = err
			continue
		}
		if len(pkgs) == 0 && len(t.seed) > 0 {
			pkgs = append([]Package(nil), t.seed...)
		}
		t.seed = pkgs
		t.count = len(pkgs)
		t.lastErr = ""
		return append([]Package(nil), pkgs...), nil
	}

	if last != nil {
		t.lastErr = last.Error()
	}
	if len(t.seed) > 0 {
		t.count = len(t.seed)
		return append([]Package(nil), t.seed...), nil
	}
	if last == nil {
		last = fmt.Errorf("toolhive catalog unavailable")
	}
	return nil, last
}

func decodeToolHiveResponse(resp *http.Response, source string) ([]Package, error) {
	if resp.StatusCode < 200 || resp.StatusCode >= 300 {
		return nil, fmt.Errorf("toolhive status %d", resp.StatusCode)
	}
	var data any
	if err := json.NewDecoder(resp.Body).Decode(&data); err != nil {
		return nil, err
	}
	return normalizeToolHive(data, source), nil
}

func normalizeToolHive(data any, source string) []Package {
	switch v := data.(type) {
	case map[string]any:
		for _, key := range []string{"items", "packages", "results", "catalog", "data"} {
			if _, ok := v[key]; ok {
				return normalizePackageList(v, source)
			}
		}
		if p := normalizeOnePackage(v, source); p.ID != "" {
			return []Package{p}
		}
		return nil
	case []any:
		return normalizePackageList(map[string]any{"items": v}, source)
	default:
		return nil
	}
}

func filterPackages(pkgs []Package, query string) []Package {
	query = strings.ToLower(strings.TrimSpace(query))
	if query == "" {
		return append([]Package(nil), pkgs...)
	}
	var out []Package
	for _, p := range pkgs {
		hay := strings.ToLower(p.Name + " " + p.Description + " " + strings.Join(p.Tags, " ") + " " + p.ID)
		if strings.Contains(hay, query) {
			out = append(out, p)
		}
	}
	return out
}

var _ Source = (*ToolHiveSource)(nil)
