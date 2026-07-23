package server

import (
	"encoding/json"
	"net/http"
	"time"

	"github.com/mohvahedi/open-mcp-control-plane/internal/catalog"
	"github.com/mohvahedi/open-mcp-control-plane/internal/config"
)

type Server struct {
	config  config.Config
	catalog *catalog.Service
	mux     *http.ServeMux
}

func New(cfg config.Config, catalogService *catalog.Service) http.Handler {
	s := &Server{config: cfg, catalog: catalogService, mux: http.NewServeMux()}
	s.routes()
	return s.withDefaults(s.mux)
}

func (s *Server) routes() {
	s.mux.HandleFunc("GET /healthz", s.health)
	s.mux.HandleFunc("GET /v1/info", s.info)
	s.mux.HandleFunc("GET /v1/catalog/search", s.searchCatalog)
}

func (s *Server) health(w http.ResponseWriter, _ *http.Request) {
	writeJSON(w, http.StatusOK, map[string]any{"status": "ok", "time": time.Now().UTC()})
}

func (s *Server) info(w http.ResponseWriter, _ *http.Request) {
	writeJSON(w, http.StatusOK, map[string]any{
		"name":    "open-mcp-control-plane",
		"version": s.config.Version,
		"status":  "foundation",
	})
}

func (s *Server) searchCatalog(w http.ResponseWriter, r *http.Request) {
	packages, err := s.catalog.Search(r.Context(), r.URL.Query().Get("q"))
	if err != nil {
		writeJSON(w, http.StatusBadGateway, map[string]string{"error": "catalog search failed"})
		return
	}
	writeJSON(w, http.StatusOK, map[string]any{"items": packages, "count": len(packages)})
}

func (s *Server) withDefaults(next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("X-Content-Type-Options", "nosniff")
		w.Header().Set("X-Frame-Options", "DENY")
		w.Header().Set("Referrer-Policy", "no-referrer")
		next.ServeHTTP(w, r)
	})
}

func writeJSON(w http.ResponseWriter, status int, value any) {
	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(status)
	_ = json.NewEncoder(w).Encode(value)
}
