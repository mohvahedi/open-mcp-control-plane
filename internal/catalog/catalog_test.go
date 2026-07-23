package catalog

import (
	"context"
	"testing"
)

func TestServiceSearchFiltersAndDeduplicates(t *testing.T) {
	first := Package{ID: "postgres", Name: "Postgres", Description: "Database tools", Tags: []string{"sql"}}
	duplicate := Package{ID: "postgres", Name: "Postgres duplicate"}
	github := Package{ID: "github", Name: "GitHub", Description: "Repository tools"}

	service := NewService(
		NewStaticSource("one", []Package{first, github}),
		NewStaticSource("two", []Package{duplicate}),
	)

	packages, err := service.Search(context.Background(), "sql")
	if err != nil {
		t.Fatal(err)
	}
	if len(packages) != 1 || packages[0].ID != "postgres" {
		t.Fatalf("unexpected result: %#v", packages)
	}
}
