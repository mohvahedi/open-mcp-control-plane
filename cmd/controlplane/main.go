package main

import (
	"log"
	"net/http"

	"github.com/mohvahedi/open-mcp-control-plane/internal/catalog"
	"github.com/mohvahedi/open-mcp-control-plane/internal/config"
	"github.com/mohvahedi/open-mcp-control-plane/internal/server"
)

func main() {
	cfg := config.Load()
	catalogService := catalog.NewService(catalog.NewStaticSource("bootstrap", nil))
	handler := server.New(cfg, catalogService)

	log.Printf("open-mcp-control-plane %s listening on %s", cfg.Version, cfg.Address())
	if err := http.ListenAndServe(cfg.Address(), handler); err != nil {
		log.Fatal(err)
	}
}
