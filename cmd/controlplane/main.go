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
	"github.com/mohvahedi/open-mcp-control-plane/internal/domain"
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
		catalog.NewToolHiveSource("toolhive", catalog.ToolHiveConfig{
			Timeout: cfg.RegistryTimeout,
		}, catalog.Package{
			ID:          "toolhive-filesystem",
			Name:        "Filesystem MCP (ToolHive seed)",
			Description: "Seed package representing a ToolHive-federated MCP server",
			Source:      "toolhive",
			Tags:        []string{"toolhive", "filesystem"},
			Transport:   "stdio",
		}),
	)

	if skills, err := repo.ListSkills(context.Background()); err == nil && len(skills) == 0 {
		_, _ = repo.CreateSkill(context.Background(), domain.Skill{
			ID:          "skill-safe-ops",
			Name:        "Safe Operations Checklist",
			Description: "Bootstrap skill: prefer read-only tools and require approval for destructive actions",
			Version:     "0.1.0",
			Source:      "bootstrap",
			Kind:        "prompt",
			License:     "Apache-2.0",
			Tags:        []string{"safety", "bootstrap"},
			Content:     "# Safe Operations\n\n1. Prefer read-only tools.\n2. Require approval for write/delete/exec.\n3. Never exfiltrate secrets.\n",
			Provenance:  map[string]any{"repository": "https://github.com/mohvahedi/open-mcp-control-plane"},
		})
	}

	var rt runtime.Runtime
	if cfg.UseFakeRuntime {
		rt = runtime.NewFakeRuntime()
	} else {
		rt = runtime.NewDockerCLI(cfg.DockerBinary, cfg.RequestTimeout)
	}

	secretsBackend, err := security.NewSecretBackend(cfg.SecretsBackend, cfg.SecretsMasterKey, cfg.SecretsFileDir)
	if err != nil {
		log.Fatalf("secrets backend: %v", err)
	}
	oidcAuth, err := security.NewOIDCAuthenticator(context.Background(), cfg.OIDC())
	if err != nil {
		log.Fatalf("oidc setup: %v", err)
	}
	handler := server.NewWithOptions(cfg, catalogService, repo, rt, secretsBackend, oidcAuth)
	httpServer := &http.Server{
		Addr:         cfg.Address(),
		Handler:      handler,
		ReadTimeout:  15 * time.Second,
		// WriteTimeout 0 allows long-lived SSE streams; request deadlines still apply elsewhere.
		WriteTimeout: 0,
		IdleTimeout:  120 * time.Second,
	}

	log.Printf("{\"msg\":\"open-mcp-control-plane listening\",\"version\":%q,\"addr\":%q,\"secrets_backend\":%q,\"oidc\":%v}", cfg.Version, cfg.Address(), secretsBackend.Name(), oidcAuth.Enabled())
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
