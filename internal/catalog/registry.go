package catalog

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"net/http"
	"net/url"
	"strings"
	"time"
)

type RegistryConfig struct {
	BaseURL    string
	Timeout    time.Duration
	PageLimit  int
	HTTPClient *http.Client
}

type RegistrySource struct {
	name         string
	baseURL      string
	timeout      time.Duration
	pageLimit    int
	httpClient   *http.Client
	lastErr      string
	lastChecked  time.Time
	packageCount int
}

func NewRegistrySource(name string, cfg RegistryConfig) *RegistrySource {
	if cfg.BaseURL == "" {
		cfg.BaseURL = "https://registry.modelcontextprotocol.io"
	}
	if cfg.Timeout <= 0 {
		cfg.Timeout = 5 * time.Second
	}
	if cfg.PageLimit <= 0 || cfg.PageLimit > 100 {
		cfg.PageLimit = 25
	}
	if cfg.HTTPClient == nil {
		cfg.HTTPClient = &http.Client{Timeout: cfg.Timeout}
	}
	if name == "" {
		name = "mcp-registry"
	}
	return &RegistrySource{name: name, baseURL: strings.TrimRight(cfg.BaseURL, "/"), timeout: cfg.Timeout, pageLimit: cfg.PageLimit, httpClient: cfg.HTTPClient}
}

func (r *RegistrySource) Name() string { return r.name }

func (r *RegistrySource) Search(ctx context.Context, query string) ([]Package, error) {
	endpoint, err := url.Parse(r.baseURL + "/v0/packages")
	if err != nil {
		return nil, err
	}
	q := endpoint.Query()
	if query != "" {
		q.Set("q", query)
	}
	q.Set("limit", fmt.Sprint(r.pageLimit))
	endpoint.RawQuery = q.Encode()

	payload, err := r.getJSON(ctx, endpoint.String())
	if err != nil {
		return nil, err
	}
	pkgs := normalizePackageList(payload, r.name)
	r.packageCount = len(pkgs)
	return pkgs, nil
}

func (r *RegistrySource) GetPackage(ctx context.Context, id string) (Package, error) {
	if id == "" {
		return Package{}, ErrPackageNotFound
	}
	payload, err := r.getJSON(ctx, r.baseURL+"/v0/packages/"+url.PathEscape(id))
	if err != nil {
		if strings.Contains(err.Error(), "404") {
			return Package{}, ErrPackageNotFound
		}
		return Package{}, err
	}
	pkg := normalizeOnePackage(payload, r.name)
	if pkg.ID == "" {
		return Package{}, ErrPackageNotFound
	}
	return pkg, nil
}

func (r *RegistrySource) Status(_ context.Context) SourceStatus {
	healthy := r.lastErr == ""
	return SourceStatus{Source: r.name, Healthy: healthy, LastError: r.lastErr, LastChecked: r.lastChecked, PackageCount: r.packageCount}
}

func (r *RegistrySource) getJSON(ctx context.Context, endpoint string) (map[string]any, error) {
	ctx, cancel := context.WithTimeout(ctx, r.timeout)
	defer cancel()
	req, err := http.NewRequestWithContext(ctx, http.MethodGet, endpoint, nil)
	if err != nil {
		return nil, err
	}
	resp, err := r.httpClient.Do(req)
	r.lastChecked = time.Now().UTC()
	if err != nil {
		r.lastErr = err.Error()
		return nil, fmt.Errorf("registry request failed: %w", err)
	}
	defer resp.Body.Close()
	if resp.StatusCode < 200 || resp.StatusCode >= 300 {
		r.lastErr = fmt.Sprintf("status %d", resp.StatusCode)
		return nil, fmt.Errorf("registry response status %d", resp.StatusCode)
	}
	var data map[string]any
	if err := json.NewDecoder(resp.Body).Decode(&data); err != nil {
		r.lastErr = err.Error()
		return nil, fmt.Errorf("registry invalid json: %w", err)
	}
	r.lastErr = ""
	return data, nil
}

func normalizePackageList(payload map[string]any, source string) []Package {
	items, ok := payload["items"].([]any)
	if !ok {
		if packages, ok := payload["packages"].([]any); ok {
			items = packages
		}
	}
	out := make([]Package, 0, len(items))
	for _, raw := range items {
		obj, ok := raw.(map[string]any)
		if !ok {
			continue
		}
		pkg := normalizeOnePackage(obj, source)
		if pkg.ID != "" {
			out = append(out, pkg)
		}
	}
	return out
}

func normalizeOnePackage(payload map[string]any, source string) Package {
	get := func(keys ...string) string {
		for _, key := range keys {
			if v, ok := payload[key].(string); ok && v != "" {
				return v
			}
		}
		return ""
	}
	pkg := Package{
		ID:          get("id", "slug", "name"),
		Name:        get("name", "id"),
		Description: get("description", "summary"),
		Version:     get("version", "latestVersion"),
		Source:      source,
		Runtime:     get("runtime"),
		Transport:   get("transport"),
		License:     get("license"),
		Provenance: map[string]any{
			"source":     source,
			"retrievedAt": time.Now().UTC().Format(time.RFC3339Nano),
			"upstream":   payload,
		},
	}
	if tags, ok := payload["tags"].([]any); ok {
		for _, t := range tags {
			if s, ok := t.(string); ok && s != "" {
				pkg.Tags = append(pkg.Tags, s)
			}
		}
	}
	if tools, ok := payload["tools"].([]any); ok {
		for _, t := range tools {
			obj, ok := t.(map[string]any)
			if !ok {
				continue
			}
			meta := ToolMetadata{Name: stringOr(obj, "name", "id"), Description: stringOr(obj, "description")}
			if ops, ok := obj["operations"].([]any); ok {
				for _, op := range ops {
					if s, ok := op.(string); ok {
						meta.Operations = append(meta.Operations, s)
					}
				}
			}
			if meta.Name != "" {
				pkg.Tools = append(pkg.Tools, meta)
			}
		}
	}
	return pkg
}

func stringOr(m map[string]any, keys ...string) string {
	for _, key := range keys {
		if v, ok := m[key].(string); ok {
			return v
		}
	}
	return ""
}

var _ Source = (*RegistrySource)(nil)

func IsSafeMetadataKey(key string) bool {
	k := strings.ToLower(key)
	return !strings.Contains(k, "instruction") && !strings.Contains(k, "execute")
}

func FilterUnsafeMetadata(m map[string]any) map[string]any {
	out := make(map[string]any, len(m))
	for k, v := range m {
		if IsSafeMetadataKey(k) {
			out[k] = v
		}
	}
	return out
}

func NormalizeWithSafety(raw map[string]any, source string) (Package, error) {
	if raw == nil {
		return Package{}, errors.New("empty payload")
	}
	safe := FilterUnsafeMetadata(raw)
	return normalizeOnePackage(safe, source), nil
}
