package catalog

import (
	"context"
	"sort"
	"strings"
)

type Package struct {
	ID          string   `json:"id"`
	Name        string   `json:"name"`
	Description string   `json:"description,omitempty"`
	Version     string   `json:"version,omitempty"`
	Source      string   `json:"source"`
	Runtime     string   `json:"runtime,omitempty"`
	Transport   string   `json:"transport,omitempty"`
	License     string   `json:"license,omitempty"`
	Tags        []string `json:"tags,omitempty"`
}

type Source interface {
	Name() string
	Search(context.Context, string) ([]Package, error)
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
