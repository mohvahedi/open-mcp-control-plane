package main

import (
	"context"
	"errors"
	"log"
	"net/http"
	"os"
	"os/signal"
	"syscall"
	"time"

	"github.com/mohvahedi/open-mcp-control-plane/internal/catalog"
	"github.com/mohvahedi/open-mcp-control-plane/internal/config"
	"github.com/mohvahedi/open-mcp-control-plane/internal/runtime"
	"github.com/mohvahedi/open-mcp-control-plane/internal/security"
	"github.com/mohvahedi/open-mcp-control-plane/internal/server"
	"github.com/mohvahedi/open-mcp-control-plane/internal/store"
)

func main() {
	cfg := config.Load()
	if err := cfg.Validate(); err != nil {
		log.Fatalf("invalid configuration: %v", err)
	}

	repo, err := store.OpenSQLite(cfg.DatabasePath)
	if err != nil {
		log.Fatal(err)
	}
	defer repo.Close()

	if err := bootstrapAdminToken(context.Background(), repo, cfg.AdminBootstrapToken); err != nil {
		log.Fatalf("admin bootstrap failed: %v", err)
	}

	catalogService := catalog.NewService(
		catalog.NewStaticSource("bootstrap", []catalog.Package{{
			ID:          "example-mcp-server",
			Name:        "Example MCP Server",
			Description: "Local bootstrap package for tests",
			Source:      "bootstrap",
			Transport:   "streamable-http",
			Tags:        []string{"test"},
		}}),
		catalog.NewRegistrySource("mcp-registry", catalog.RegistryConfig{
			BaseURL:   cfg.RegistryBaseURL,
			Timeout:   cfg.RegistryTimeout,
			PageLimit: cfg.RegistryPageLimit,
		}),
	)

	var rt runtime.Runtime
	if cfg.UseFakeRuntime {
		rt = runtime.NewFakeRuntime()
	} else {
		rt = runtime.NewDockerCLI(cfg.DockerBinary, cfg.RequestTimeout)
	}

	handler := server.New(cfg, catalogService, repo, rt)
	httpServer := &http.Server{
		Addr:         cfg.Address(),
		Handler:      handler,
		ReadTimeout:  10 * time.Second,
		WriteTimeout: 20 * time.Second,
		IdleTimeout:  60 * time.Second,
	}

	log.Printf("{\"msg\":\"open-mcp-control-plane listening\",\"version\":%q,\"addr\":%q}", cfg.Version, cfg.Address())
	go func() {
		if err := httpServer.ListenAndServe(); err != nil && !errors.Is(err, http.ErrServerClosed) {
			log.Fatal(err)
		}
	}()

	sigCh := make(chan os.Signal, 1)
	signal.Notify(sigCh, syscall.SIGINT, syscall.SIGTERM)
	<-sigCh
	ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
	defer cancel()
	_ = httpServer.Shutdown(ctx)
}

func bootstrapAdminToken(ctx context.Context, repo *store.SQLiteRepository, configuredToken string) error {
	_, err := repo.GetAdminHash(ctx)
	if err == nil {
		return nil
	}
	if !errors.Is(err, store.ErrNotFound) {
		return err
	}
	token := configuredToken
	if token == "" {
		generated, err := security.NewToken()
		if err != nil {
			return err
		}
		token = generated
		log.Printf("{\"msg\":\"bootstrap admin token generated\",\"token\":%q,\"note\":\"store this token securely; it is shown only once\"}", token)
	}
	hash, err := security.HashToken(token)
	if err != nil {
		return err
	}
	return repo.SetAdminHash(ctx, hash)
}
