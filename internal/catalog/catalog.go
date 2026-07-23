package catalog

import (
	"context"
	"errors"
	"sort"
	"strings"
	"time"
)

var ErrPackageNotFound = errors.New("package not found")

type ToolMetadata struct {
	Name        string   `json:"name"`
	Description string   `json:"description,omitempty"`
	Operations  []string `json:"operations,omitempty"`
}

type Package struct {
	ID          string         `json:"id"`
	Name        string         `json:"name"`
	Description string         `json:"description,omitempty"`
	Version     string         `json:"version,omitempty"`
	Source      string         `json:"source"`
	Runtime     string         `json:"runtime,omitempty"`
	Transport   string         `json:"transport,omitempty"`
	License     string         `json:"license,omitempty"`
	Tags        []string       `json:"tags,omitempty"`
	Tools       []ToolMetadata `json:"tools,omitempty"`
	Provenance  map[string]any `json:"provenance,omitempty"`
}

type SourceStatus struct {
	Source       string    `json:"source"`
	Healthy      bool      `json:"healthy"`
	LastError    string    `json:"last_error,omitempty"`
	LastChecked  time.Time `json:"last_checked"`
	PackageCount int       `json:"package_count"`
}

type Source interface {
	Name() string
	Search(context.Context, string) ([]Package, error)
	GetPackage(context.Context, string) (Package, error)
	Status(context.Context) SourceStatus
}

type Service struct{ sources []Source }

func NewService(sources ...Source) *Service { return &Service{sources: sources} }

func (s *Service) Search(ctx context.Context, query string) ([]Package, error) {
	query = strings.ToLower(strings.TrimSpace(query))
	seen := make(map[string]Package)
	for _, source := range s.sources {
		packages, err := source.Search(ctx, query)
		if err != nil {
			return nil, err
		}
		for _, pkg := range packages {
			if pkg.ID == "" {
				continue
			}
			if _, exists := seen[pkg.ID]; !exists {
				seen[pkg.ID] = pkg
			}
		}
	}
	result := make([]Package, 0, len(seen))
	for _, pkg := range seen {
		result = append(result, pkg)
	}
	sort.Slice(result, func(i, j int) bool { return result[i].Name < result[j].Name })
	return result, nil
}

func (s *Service) GetPackage(ctx context.Context, id string) (Package, error) {
	for _, source := range s.sources {
		pkg, err := source.GetPackage(ctx, id)
		if err == nil {
			return pkg, nil
		}
		if !errors.Is(err, ErrPackageNotFound) {
			return Package{}, err
		}
	}
	return Package{}, ErrPackageNotFound
}

func (s *Service) SourceStatus(ctx context.Context) []SourceStatus {
	statuses := make([]SourceStatus, 0, len(s.sources))
	for _, source := range s.sources {
		statuses = append(statuses, source.Status(ctx))
	}
	return statuses
}

type StaticSource struct {
	name     string
	packages []Package
}

func NewStaticSource(name string, packages []Package) *StaticSource {
	return &StaticSource{name: name, packages: packages}
}
func (s *StaticSource) Name() string { return s.name }
func (s *StaticSource) Search(_ context.Context, query string) ([]Package, error) {
	if query == "" {
		return append([]Package(nil), s.packages...), nil
	}
	var result []Package
	for _, pkg := range s.packages {
		haystack := strings.ToLower(pkg.Name + " " + pkg.Description + " " + strings.Join(pkg.Tags, " "))
		if strings.Contains(haystack, query) {
			result = append(result, pkg)
		}
	}
	return result, nil
}

func (s *StaticSource) GetPackage(_ context.Context, id string) (Package, error) {
	for _, pkg := range s.packages {
		if pkg.ID == id {
			return pkg, nil
		}
	}
	return Package{}, ErrPackageNotFound
}

func (s *StaticSource) Status(_ context.Context) SourceStatus {
	return SourceStatus{Source: s.name, Healthy: true, LastChecked: time.Now().UTC(), PackageCount: len(s.packages)}
}
