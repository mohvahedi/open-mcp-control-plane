package server

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"log"
	"net/http"
	"sort"
	"strings"
	"time"

	"github.com/mohvahedi/open-mcp-control-plane/internal/catalog"
	"github.com/mohvahedi/open-mcp-control-plane/internal/config"
	"github.com/mohvahedi/open-mcp-control-plane/internal/domain"
	"github.com/mohvahedi/open-mcp-control-plane/internal/policy"
	"github.com/mohvahedi/open-mcp-control-plane/internal/runtime"
	"github.com/mohvahedi/open-mcp-control-plane/internal/security"
	"github.com/mohvahedi/open-mcp-control-plane/internal/store"
)

type Server struct {
	config     config.Config
	catalog    *catalog.Service
	repo       store.Repository
	runtime    runtime.Runtime
	mux        *http.ServeMux
	httpClient *http.Client
}

func New(cfg config.Config, catalogService *catalog.Service, repo store.Repository, rt runtime.Runtime) http.Handler {
	s := &Server{
		config:     cfg,
		catalog:    catalogService,
		repo:       repo,
		runtime:    rt,
		mux:        http.NewServeMux(),
		httpClient: &http.Client{Timeout: cfg.RequestTimeout},
	}
	s.routes()
	return s.withDefaults(s.mux)
}

func (s *Server) routes() {
	s.mux.HandleFunc("GET /", s.gui)
	s.mux.HandleFunc("GET /healthz", s.health)
	s.mux.HandleFunc("GET /readyz", s.ready)
	s.mux.HandleFunc("GET /v1/info", s.info)
	s.mux.HandleFunc("GET /v1/csrf", s.csrf)
	// catalog discovery
	s.mux.HandleFunc("GET /v1/catalog/search", s.searchCatalog)
	s.mux.HandleFunc("GET /v1/catalog/packages/{id}", s.inspectPackage)
	s.mux.HandleFunc("GET /v1/catalog/sources/status", s.catalogSourceStatus)

	// admin management API
	s.mux.HandleFunc("GET /v1/admin/plans", s.withAdminAuth(s.listPlans))
	s.mux.HandleFunc("POST /v1/admin/plans", s.withAdminAuth(s.createPlan))
	s.mux.HandleFunc("GET /v1/admin/plans/{id}", s.withAdminAuth(s.getPlan))
	s.mux.HandleFunc("GET /v1/admin/approvals", s.withAdminAuth(s.listApprovals))
	s.mux.HandleFunc("POST /v1/admin/approvals", s.withAdminAuth(s.createApproval))
	s.mux.HandleFunc("GET /v1/admin/installations", s.withAdminAuth(s.listInstallations))
	s.mux.HandleFunc("POST /v1/admin/installations/apply", s.withAdminAuth(s.applyInstallation))
	s.mux.HandleFunc("GET /v1/admin/installations/{id}/health", s.withAdminAuth(s.installationHealth))
	s.mux.HandleFunc("GET /v1/admin/installations/{id}/logs", s.withAdminAuth(s.installationLogs))
	s.mux.HandleFunc("POST /v1/admin/installations/{id}/start", s.withAdminAuth(s.installationStart))
	s.mux.HandleFunc("POST /v1/admin/installations/{id}/stop", s.withAdminAuth(s.installationStop))
	s.mux.HandleFunc("POST /v1/admin/installations/{id}/restart", s.withAdminAuth(s.installationRestart))
	s.mux.HandleFunc("POST /v1/admin/installations/{id}/disable", s.withAdminAuth(s.installationDisable))
	s.mux.HandleFunc("POST /v1/admin/installations/{id}/uninstall", s.withAdminAuth(s.installationUninstall))
	s.mux.HandleFunc("GET /v1/admin/profiles", s.withAdminAuth(s.listProfiles))
	s.mux.HandleFunc("POST /v1/admin/profiles", s.withAdminAuth(s.createProfile))
	s.mux.HandleFunc("POST /v1/admin/profiles/{id}/installations", s.withAdminAuth(s.profileInstallations))
	s.mux.HandleFunc("POST /v1/admin/profiles/{id}/tools", s.withAdminAuth(s.profileTools))
	s.mux.HandleFunc("GET /v1/admin/clients", s.withAdminAuth(s.listClients))
	s.mux.HandleFunc("POST /v1/admin/clients", s.withAdminAuth(s.createClient))
	s.mux.HandleFunc("POST /v1/admin/clients/{id}/revoke", s.withAdminAuth(s.revokeClient))
	s.mux.HandleFunc("GET /v1/admin/audit", s.withAdminAuth(s.listAudit))
	s.mux.HandleFunc("GET /v1/admin/gateway/status", s.withAdminAuth(s.gatewayStatus))

	// management MCP
	s.mux.HandleFunc("POST /mcp/management", s.withAdminAuth(s.managementMCP))

	// remote MCP gateway
	s.mux.HandleFunc("GET /gateway/tools", s.withClientAuth(s.gatewayTools))
	s.mux.HandleFunc("POST /gateway/invoke", s.withClientAuth(s.gatewayInvoke))
}

func (s *Server) health(w http.ResponseWriter, _ *http.Request) {
	writeJSON(w, http.StatusOK, map[string]any{"status": "ok", "time": time.Now().UTC()})
}

func (s *Server) ready(w http.ResponseWriter, r *http.Request) {
	if err := s.repo.Ping(r.Context()); err != nil {
		writeJSON(w, http.StatusServiceUnavailable, map[string]string{"status": "not_ready"})
		return
	}
	writeJSON(w, http.StatusOK, map[string]string{"status": "ready"})
}

func (s *Server) info(w http.ResponseWriter, _ *http.Request) {
	writeJSON(w, http.StatusOK, map[string]any{
		"name":    "open-mcp-control-plane",
		"version": s.config.Version,
		"status":  "v0.1-mvp",
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

func (s *Server) inspectPackage(w http.ResponseWriter, r *http.Request) {
	pkg, err := s.catalog.GetPackage(r.Context(), r.PathValue("id"))
	if err != nil {
		if errors.Is(err, catalog.ErrPackageNotFound) {
			writeJSON(w, http.StatusNotFound, map[string]string{"error": "package not found"})
			return
		}
		writeJSON(w, http.StatusBadGateway, map[string]string{"error": "catalog package lookup failed"})
		return
	}
	writeJSON(w, http.StatusOK, pkg)
}

func (s *Server) catalogSourceStatus(w http.ResponseWriter, r *http.Request) {
	writeJSON(w, http.StatusOK, map[string]any{"sources": s.catalog.SourceStatus(r.Context())})
}

func (s *Server) createPlan(w http.ResponseWriter, r *http.Request) {
	var plan domain.DeploymentPlan
	if !decodeJSON(w, r, &plan) {
		return
	}
	if plan.Name == "" {
		writeJSON(w, http.StatusBadRequest, map[string]string{"error": "name is required"})
		return
	}
	if plan.Image == "" && plan.RemoteEndpoint == "" {
		writeJSON(w, http.StatusBadRequest, map[string]string{"error": "image or remote_endpoint is required"})
		return
	}
	plan.ID = newID("plan")
	plan.CreatedAt = time.Now().UTC()
	decision := policy.Evaluate(plan)
	plan.RiskFindings = decision.Findings
	plan.RequiresApproval = decision.RequiresApproval
	stored, err := s.repo.CreateDeploymentPlan(r.Context(), plan)
	if err != nil {
		writeJSON(w, http.StatusInternalServerError, map[string]string{"error": "failed to store plan"})
		return
	}
	s.audit(r.Context(), "admin", "create_plan", stored.ID, "ok", map[string]any{"requires_approval": stored.RequiresApproval})
	writeJSON(w, http.StatusCreated, stored)
}

func (s *Server) listPlans(w http.ResponseWriter, r *http.Request) {
	plans, err := s.repo.ListDeploymentPlans(r.Context())
	if err != nil {
		writeJSON(w, http.StatusInternalServerError, map[string]string{"error": "failed to list plans"})
		return
	}
	writeJSON(w, http.StatusOK, map[string]any{"items": plans})
}

func (s *Server) getPlan(w http.ResponseWriter, r *http.Request) {
	plan, err := s.repo.GetDeploymentPlan(r.Context(), r.PathValue("id"))
	if err != nil {
		if errors.Is(err, store.ErrNotFound) {
			writeJSON(w, http.StatusNotFound, map[string]string{"error": "plan not found"})
			return
		}
		writeJSON(w, http.StatusInternalServerError, map[string]string{"error": "failed to get plan"})
		return
	}
	writeJSON(w, http.StatusOK, plan)
}

func (s *Server) createApproval(w http.ResponseWriter, r *http.Request) {
	var req struct {
		PlanID string `json:"plan_id"`
		Reason string `json:"reason"`
	}
	if !decodeJSON(w, r, &req) {
		return
	}
	if req.PlanID == "" || req.Reason == "" {
		writeJSON(w, http.StatusBadRequest, map[string]string{"error": "plan_id and reason are required"})
		return
	}
	approval := domain.Approval{ID: newID("approval"), PlanID: req.PlanID, Reason: req.Reason, CreatedAt: time.Now().UTC()}
	created, err := s.repo.CreateApproval(r.Context(), approval)
	if err != nil {
		writeJSON(w, http.StatusBadRequest, map[string]string{"error": "failed to create approval"})
		return
	}
	s.audit(r.Context(), "admin", "approve_plan", req.PlanID, "ok", map[string]any{"approval_id": created.ID})
	writeJSON(w, http.StatusCreated, created)
}

func (s *Server) listApprovals(w http.ResponseWriter, r *http.Request) {
	items, err := s.repo.ListApprovals(r.Context())
	if err != nil {
		writeJSON(w, http.StatusInternalServerError, map[string]string{"error": "failed to list approvals"})
		return
	}
	writeJSON(w, http.StatusOK, map[string]any{"items": items})
}

func (s *Server) applyInstallation(w http.ResponseWriter, r *http.Request) {
	var req struct {
		PlanID string `json:"plan_id"`
	}
	if !decodeJSON(w, r, &req) {
		return
	}
	plan, err := s.repo.GetDeploymentPlan(r.Context(), req.PlanID)
	if err != nil {
		writeJSON(w, http.StatusNotFound, map[string]string{"error": "plan not found"})
		return
	}
	if plan.RequiresApproval {
		if _, ok, err := s.repo.GetApprovalByPlanID(r.Context(), plan.ID); err != nil || !ok {
			writeJSON(w, http.StatusForbidden, map[string]string{"error": "approval required before apply"})
			return
		}
	}

	runtimeRef := plan.RemoteEndpoint
	state := "configured"
	imageDigest := ""
	rollback := map[string]string{"type": "reapply_previous_plan", "plan_id": plan.ID}
	if plan.Image != "" {
		ref, err := s.runtime.Install(r.Context(), runtime.InstallSpec{
			Name:     sanitizeName(plan.Name),
			Image:    plan.Image,
			EnvNames: plan.EnvVarNames,
			CPU:      plan.CPULimit,
			Memory:   plan.MemoryLimit,
			PIDs:     max(20, plan.PIDLimit),
			Network:  "bridge",
			Labels: map[string]string{
				"openmcp.managed": "true",
				"openmcp.plan_id": plan.ID,
			},
			ReadOnlyFS: true,
		})
		if err != nil {
			writeJSON(w, http.StatusBadGateway, map[string]string{"error": "runtime install failed"})
			return
		}
		runtimeRef = ref
		state = "running"
		if strings.Contains(plan.Image, "@sha256:") {
			imageDigest = strings.Split(plan.Image, "@sha256:")[1]
		}
	}

	installation := domain.Installation{
		ID:               newID("inst"),
		PlanID:           plan.ID,
		PackageID:        plan.PackageID,
		RuntimeRef:       runtimeRef,
		State:            state,
		ImageDigest:      imageDigest,
		RollbackMetadata: rollback,
		CreatedAt:        time.Now().UTC(),
		UpdatedAt:        time.Now().UTC(),
	}
	stored, err := s.repo.CreateInstallation(r.Context(), installation)
	if err != nil {
		writeJSON(w, http.StatusInternalServerError, map[string]string{"error": "failed to store installation"})
		return
	}
	s.audit(r.Context(), "admin", "apply_plan", stored.ID, "ok", map[string]any{"plan_id": stored.PlanID})
	writeJSON(w, http.StatusCreated, stored)
}

func (s *Server) listInstallations(w http.ResponseWriter, r *http.Request) {
	items, err := s.repo.ListInstallations(r.Context())
	if err != nil {
		writeJSON(w, http.StatusInternalServerError, map[string]string{"error": "failed to list installations"})
		return
	}
	writeJSON(w, http.StatusOK, map[string]any{"items": items})
}

func (s *Server) installationHealth(w http.ResponseWriter, r *http.Request) {
	inst, err := s.repo.GetInstallation(r.Context(), r.PathValue("id"))
	if err != nil {
		writeJSON(w, http.StatusNotFound, map[string]string{"error": "installation not found"})
		return
	}
	status, err := s.runtime.Health(r.Context(), inst.RuntimeRef)
	if err != nil {
		writeJSON(w, http.StatusBadGateway, map[string]string{"error": "runtime health failed"})
		return
	}
	writeJSON(w, http.StatusOK, status)
}

func (s *Server) installationLogs(w http.ResponseWriter, r *http.Request) {
	inst, err := s.repo.GetInstallation(r.Context(), r.PathValue("id"))
	if err != nil {
		writeJSON(w, http.StatusNotFound, map[string]string{"error": "installation not found"})
		return
	}
	logs, err := s.runtime.Logs(r.Context(), inst.RuntimeRef, 200)
	if err != nil {
		writeJSON(w, http.StatusBadGateway, map[string]string{"error": "runtime logs failed"})
		return
	}
	writeJSON(w, http.StatusOK, map[string]any{"logs": logs})
}

func (s *Server) installationStart(w http.ResponseWriter, r *http.Request) {
	s.installationControl(w, r, "start")
}
func (s *Server) installationStop(w http.ResponseWriter, r *http.Request) {
	s.installationControl(w, r, "stop")
}
func (s *Server) installationRestart(w http.ResponseWriter, r *http.Request) {
	s.installationControl(w, r, "restart")
}
func (s *Server) installationDisable(w http.ResponseWriter, r *http.Request) {
	if err := s.repo.DisableInstallation(r.Context(), r.PathValue("id")); err != nil {
		writeJSON(w, http.StatusInternalServerError, map[string]string{"error": "failed to disable installation"})
		return
	}
	s.audit(r.Context(), "admin", "disable_installation", r.PathValue("id"), "ok", nil)
	writeJSON(w, http.StatusOK, map[string]string{"status": "disabled"})
}
func (s *Server) installationUninstall(w http.ResponseWriter, r *http.Request) {
	id := r.PathValue("id")
	inst, err := s.repo.GetInstallation(r.Context(), id)
	if err != nil {
		writeJSON(w, http.StatusNotFound, map[string]string{"error": "installation not found"})
		return
	}
	if err := s.runtime.Uninstall(r.Context(), inst.RuntimeRef); err != nil {
		writeJSON(w, http.StatusBadGateway, map[string]string{"error": "runtime uninstall failed"})
		return
	}
	if err := s.repo.DeleteInstallation(r.Context(), id); err != nil {
		writeJSON(w, http.StatusInternalServerError, map[string]string{"error": "failed to remove installation"})
		return
	}
	s.audit(r.Context(), "admin", "uninstall", id, "ok", nil)
	writeJSON(w, http.StatusOK, map[string]string{"status": "removed"})
}

func (s *Server) installationControl(w http.ResponseWriter, r *http.Request, action string) {
	id := r.PathValue("id")
	inst, err := s.repo.GetInstallation(r.Context(), id)
	if err != nil {
		writeJSON(w, http.StatusNotFound, map[string]string{"error": "installation not found"})
		return
	}
	switch action {
	case "start":
		err = s.runtime.Start(r.Context(), inst.RuntimeRef)
		inst.State = "running"
	case "stop":
		err = s.runtime.Stop(r.Context(), inst.RuntimeRef)
		inst.State = "stopped"
	case "restart":
		err = s.runtime.Restart(r.Context(), inst.RuntimeRef)
		inst.State = "running"
	}
	if err != nil {
		writeJSON(w, http.StatusBadGateway, map[string]string{"error": "runtime operation failed"})
		return
	}
	inst.UpdatedAt = time.Now().UTC()
	_ = s.repo.UpdateInstallation(r.Context(), inst)
	s.audit(r.Context(), "admin", action, id, "ok", nil)
	writeJSON(w, http.StatusOK, map[string]string{"status": inst.State})
}

func (s *Server) createProfile(w http.ResponseWriter, r *http.Request) {
	var p domain.Profile
	if !decodeJSON(w, r, &p) {
		return
	}
	if p.Name == "" {
		writeJSON(w, http.StatusBadRequest, map[string]string{"error": "name is required"})
		return
	}
	p.ID = newID("profile")
	p.CreatedAt = time.Now().UTC()
	created, err := s.repo.CreateProfile(r.Context(), p)
	if err != nil {
		writeJSON(w, http.StatusBadRequest, map[string]string{"error": "failed to create profile"})
		return
	}
	writeJSON(w, http.StatusCreated, created)
}

func (s *Server) listProfiles(w http.ResponseWriter, r *http.Request) {
	items, err := s.repo.ListProfiles(r.Context())
	if err != nil {
		writeJSON(w, http.StatusInternalServerError, map[string]string{"error": "failed to list profiles"})
		return
	}
	writeJSON(w, http.StatusOK, map[string]any{"items": items})
}

func (s *Server) profileInstallations(w http.ResponseWriter, r *http.Request) {
	var req struct {
		InstallationIDs []string `json:"installation_ids"`
	}
	if !decodeJSON(w, r, &req) {
		return
	}
	if err := s.repo.SetProfileInstallations(r.Context(), r.PathValue("id"), req.InstallationIDs); err != nil {
		writeJSON(w, http.StatusBadRequest, map[string]string{"error": "failed to update profile installations"})
		return
	}
	writeJSON(w, http.StatusOK, map[string]string{"status": "ok"})
}

func (s *Server) profileTools(w http.ResponseWriter, r *http.Request) {
	var req struct {
		Tools []string `json:"tools"`
	}
	if !decodeJSON(w, r, &req) {
		return
	}
	if err := s.repo.SetProfileTools(r.Context(), r.PathValue("id"), req.Tools); err != nil {
		writeJSON(w, http.StatusBadRequest, map[string]string{"error": "failed to update profile tools"})
		return
	}
	writeJSON(w, http.StatusOK, map[string]string{"status": "ok"})
}

func (s *Server) createClient(w http.ResponseWriter, r *http.Request) {
	var req struct {
		ProfileID string `json:"profile_id"`
		Name      string `json:"name"`
	}
	if !decodeJSON(w, r, &req) {
		return
	}
	if _, err := s.repo.GetProfile(r.Context(), req.ProfileID); err != nil {
		writeJSON(w, http.StatusBadRequest, map[string]string{"error": "profile not found"})
		return
	}
	token, err := security.NewToken()
	if err != nil {
		writeJSON(w, http.StatusInternalServerError, map[string]string{"error": "failed to create token"})
		return
	}
	hash, err := security.HashToken(token)
	if err != nil {
		writeJSON(w, http.StatusInternalServerError, map[string]string{"error": "failed to hash token"})
		return
	}
	client := domain.Client{ID: newID("client"), ProfileID: req.ProfileID, Name: req.Name, CreatedAt: time.Now().UTC()}
	created, err := s.repo.CreateClient(r.Context(), client, hash)
	if err != nil {
		writeJSON(w, http.StatusBadRequest, map[string]string{"error": "failed to create client"})
		return
	}
	writeJSON(w, http.StatusCreated, map[string]any{"client": created, "token": created.ID + "." + token})
}

func (s *Server) listClients(w http.ResponseWriter, r *http.Request) {
	items, err := s.repo.ListClients(r.Context())
	if err != nil {
		writeJSON(w, http.StatusInternalServerError, map[string]string{"error": "failed to list clients"})
		return
	}
	writeJSON(w, http.StatusOK, map[string]any{"items": items})
}

func (s *Server) revokeClient(w http.ResponseWriter, r *http.Request) {
	if err := s.repo.RevokeClient(r.Context(), r.PathValue("id")); err != nil {
		writeJSON(w, http.StatusBadRequest, map[string]string{"error": "failed to revoke client"})
		return
	}
	writeJSON(w, http.StatusOK, map[string]string{"status": "revoked"})
}

func (s *Server) listAudit(w http.ResponseWriter, r *http.Request) {
	items, err := s.repo.ListAuditEvents(r.Context(), 200)
	if err != nil {
		writeJSON(w, http.StatusInternalServerError, map[string]string{"error": "failed to list audit events"})
		return
	}
	writeJSON(w, http.StatusOK, map[string]any{"items": items})
}

func (s *Server) gatewayStatus(w http.ResponseWriter, r *http.Request) {
	inst, _ := s.repo.ListInstallations(r.Context())
	prof, _ := s.repo.ListProfiles(r.Context())
	clients, _ := s.repo.ListClients(r.Context())
	writeJSON(w, http.StatusOK, map[string]any{"installations": len(inst), "profiles": len(prof), "clients": len(clients), "path": "/gateway"})
}

func (s *Server) managementMCP(w http.ResponseWriter, r *http.Request) {
	var req struct {
		Tool  string         `json:"tool"`
		Input map[string]any `json:"input"`
	}
	if !decodeJSON(w, r, &req) {
		return
	}
	ctx := r.Context()
	switch req.Tool {
	case "search_catalog":
		q, _ := req.Input["q"].(string)
		items, err := s.catalog.Search(ctx, q)
		if err != nil {
			writeJSON(w, http.StatusBadGateway, map[string]string{"error": "catalog search failed"})
			return
		}
		writeJSON(w, http.StatusOK, map[string]any{"items": items})
	case "inspect_package":
		id, _ := req.Input["id"].(string)
		pkg, err := s.catalog.GetPackage(ctx, id)
		if err != nil {
			writeJSON(w, http.StatusNotFound, map[string]string{"error": "package not found"})
			return
		}
		writeJSON(w, http.StatusOK, pkg)
	case "create_deployment_plan":
		body, _ := json.Marshal(req.Input)
		h := http.HandlerFunc(s.createPlan)
		h.ServeHTTP(w, r.WithContext(context.WithValue(ctx, rewiredBodyKey{}, io.NopCloser(strings.NewReader(string(body))))))
	case "request_installation":
		body, _ := json.Marshal(map[string]any{"plan_id": req.Input["plan_id"]})
		h := http.HandlerFunc(s.applyInstallation)
		h.ServeHTTP(w, r.WithContext(context.WithValue(ctx, rewiredBodyKey{}, io.NopCloser(strings.NewReader(string(body))))))
	case "list_installations":
		s.listInstallations(w, r)
	case "check_health":
		s.health(w, r)
	case "request_update":
		writeJSON(w, http.StatusOK, map[string]string{"status": "update prepared"})
	case "disable_installation":
		id, _ := req.Input["id"].(string)
		if err := s.repo.DisableInstallation(ctx, id); err != nil {
			writeJSON(w, http.StatusBadRequest, map[string]string{"error": "disable failed"})
			return
		}
		writeJSON(w, http.StatusOK, map[string]string{"status": "disabled"})
	default:
		writeJSON(w, http.StatusBadRequest, map[string]string{"error": "unknown management tool"})
	}
}

func (s *Server) withDefaults(next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("X-Content-Type-Options", "nosniff")
		w.Header().Set("X-Frame-Options", "DENY")
		w.Header().Set("Referrer-Policy", "no-referrer")
		w.Header().Set("Content-Security-Policy", "default-src 'self'; style-src 'self' 'unsafe-inline'; script-src 'self' 'unsafe-inline'")
		w.Header().Set("Cache-Control", "no-store")

		r.Body = http.MaxBytesReader(w, r.Body, 1<<20)
		next.ServeHTTP(w, r)
	})
}

func (s *Server) withAdminAuth(next http.HandlerFunc) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		hash, err := s.repo.GetAdminHash(r.Context())
		if err != nil {
			writeJSON(w, http.StatusUnauthorized, map[string]string{"error": "admin not initialized"})
			return
		}
		token := strings.TrimPrefix(r.Header.Get("Authorization"), "Bearer ")
		if token == "" || !security.VerifyToken(hash, token) {
			writeJSON(w, http.StatusUnauthorized, map[string]string{"error": "unauthorized"})
			return
		}
		if isMutation(r.Method) && browserRequest(r) {
			if !validateCSRF(r) {
				writeJSON(w, http.StatusForbidden, map[string]string{"error": "csrf check failed"})
				return
			}
		}
		next(w, r)
	}
}

func (s *Server) withClientAuth(next func(http.ResponseWriter, *http.Request, domain.Client, domain.Profile)) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		raw := strings.TrimPrefix(r.Header.Get("Authorization"), "Bearer ")
		parts := strings.SplitN(raw, ".", 2)
		if len(parts) != 2 {
			writeJSON(w, http.StatusUnauthorized, map[string]string{"error": "invalid gateway token"})
			return
		}
		client, tokenHash, ok, err := s.repo.GetClientAuth(r.Context(), parts[0])
		if err != nil || !ok || client.Revoked || !security.VerifyToken(tokenHash, parts[1]) {
			writeJSON(w, http.StatusUnauthorized, map[string]string{"error": "invalid gateway token"})
			return
		}
		profile, err := s.repo.GetProfile(r.Context(), client.ProfileID)
		if err != nil {
			writeJSON(w, http.StatusUnauthorized, map[string]string{"error": "profile unavailable"})
			return
		}
		next(w, r, client, profile)
	}
}

func (s *Server) gatewayTools(w http.ResponseWriter, r *http.Request, client domain.Client, profile domain.Profile) {
	tools, _, err := s.collectTools(r.Context(), profile)
	if err != nil {
		writeJSON(w, http.StatusBadGateway, map[string]string{"error": "failed to query downstream"})
		return
	}
	s.audit(r.Context(), "client", "gateway_tools", profile.ID, "ok", map[string]any{"client_id": client.ID, "count": len(tools)})
	writeJSON(w, http.StatusOK, map[string]any{"tools": tools})
}

func (s *Server) gatewayInvoke(w http.ResponseWriter, r *http.Request, client domain.Client, profile domain.Profile) {
	var req struct {
		Tool string         `json:"tool"`
		Args map[string]any `json:"args"`
	}
	if !decodeJSON(w, r, &req) {
		return
	}
	_, toolMap, err := s.collectTools(r.Context(), profile)
	if err != nil {
		writeJSON(w, http.StatusBadGateway, map[string]string{"error": "failed to query downstream"})
		return
	}
	ref, ok := toolMap[req.Tool]
	if !ok {
		writeJSON(w, http.StatusForbidden, map[string]string{"error": "tool not allowed"})
		return
	}
	payload := map[string]any{"tool": ref.ToolName, "args": req.Args}
	ctx := r.Context()
	request, err := http.NewRequestWithContext(ctx, http.MethodPost, strings.TrimRight(ref.Endpoint, "/")+"/invoke", toBody(payload))
	if err != nil {
		writeJSON(w, http.StatusBadGateway, map[string]string{"error": "invoke request failed"})
		return
	}
	request.Header.Set("Content-Type", "application/json")
	if deadline, ok := ctx.Deadline(); ok {
		request.Header.Set("X-Request-Deadline", deadline.UTC().Format(time.RFC3339Nano))
	}
	resp, err := s.httpClient.Do(request)
	if err != nil {
		writeJSON(w, http.StatusBadGateway, map[string]string{"error": "downstream invoke failed"})
		return
	}
	defer resp.Body.Close()
	if resp.StatusCode >= 400 {
		writeJSON(w, http.StatusBadGateway, map[string]string{"error": "downstream invoke failed"})
		return
	}
	var out any
	if err := json.NewDecoder(resp.Body).Decode(&out); err != nil {
		writeJSON(w, http.StatusBadGateway, map[string]string{"error": "invalid downstream response"})
		return
	}
	metadata := security.RedactMap(map[string]any{"tool": req.Tool, "args": req.Args})
	s.audit(r.Context(), "client", "gateway_invoke", req.Tool, "ok", map[string]any{"client_id": client.ID, "metadata": metadata})
	writeJSON(w, http.StatusOK, out)
}

type gatewayToolRef struct {
	Endpoint string
	ToolName string
}

func (s *Server) collectTools(ctx context.Context, profile domain.Profile) ([]map[string]any, map[string]gatewayToolRef, error) {
	if len(profile.InstallationIDs) == 0 || len(profile.ToolAllowlist) == 0 {
		return []map[string]any{}, map[string]gatewayToolRef{}, nil
	}
	allow := make(map[string]struct{}, len(profile.ToolAllowlist))
	for _, t := range profile.ToolAllowlist {
		allow[t] = struct{}{}
	}
	var result []map[string]any
	toolMap := map[string]gatewayToolRef{}
	for _, installationID := range profile.InstallationIDs {
		inst, err := s.repo.GetInstallation(ctx, installationID)
		if err != nil || !strings.HasPrefix(inst.RuntimeRef, "http") {
			continue
		}
		resp, err := s.httpClient.Get(strings.TrimRight(inst.RuntimeRef, "/") + "/tools")
		if err != nil {
			continue
		}
		var payload struct {
			Tools []map[string]any `json:"tools"`
		}
		if err := json.NewDecoder(resp.Body).Decode(&payload); err != nil {
			_ = resp.Body.Close()
			continue
		}
		_ = resp.Body.Close()
		for _, tool := range payload.Tools {
			name, _ := tool["name"].(string)
			if name == "" {
				continue
			}
			ns := installationID + "." + name
			_, allowedNamespace := allow[ns]
			_, allowedRaw := allow[name]
			if !allowedNamespace && !allowedRaw {
				continue
			}
			result = append(result, map[string]any{
				"name":        ns,
				"description": tool["description"],
				"source":      installationID,
			})
			toolMap[ns] = gatewayToolRef{Endpoint: inst.RuntimeRef, ToolName: name}
		}
	}
	sort.Slice(result, func(i, j int) bool {
		return fmt.Sprint(result[i]["name"]) < fmt.Sprint(result[j]["name"])
	})
	return result, toolMap, nil
}

func (s *Server) csrf(w http.ResponseWriter, _ *http.Request) {
	token, _ := security.NewToken()
	http.SetCookie(w, &http.Cookie{Name: "openmcp_csrf", Value: token, Path: "/", SameSite: http.SameSiteLaxMode})
	writeJSON(w, http.StatusOK, map[string]string{"csrf_token": token})
}

func validateCSRF(r *http.Request) bool {
	cookie, err := r.Cookie("openmcp_csrf")
	if err != nil {
		return false
	}
	header := r.Header.Get("X-CSRF-Token")
	return header != "" && header == cookie.Value
}

func browserRequest(r *http.Request) bool {
	return r.Header.Get("Origin") != ""
}

func isMutation(method string) bool {
	switch method {
	case http.MethodPost, http.MethodPut, http.MethodPatch, http.MethodDelete:
		return true
	default:
		return false
	}
}

func decodeJSON(w http.ResponseWriter, r *http.Request, value any) bool {
	body := r.Body
	if rewired, ok := r.Context().Value(rewiredBodyKey{}).(io.ReadCloser); ok {
		body = rewired
	}
	defer body.Close()
	decoder := json.NewDecoder(body)
	decoder.DisallowUnknownFields()
	if err := decoder.Decode(value); err != nil {
		writeJSON(w, http.StatusBadRequest, map[string]string{"error": "invalid json"})
		return false
	}
	return true
}

type rewiredBodyKey struct{}

func writeJSON(w http.ResponseWriter, status int, value any) {
	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(status)
	_ = json.NewEncoder(w).Encode(value)
}

func newID(prefix string) string {
	token, err := security.NewToken()
	if err != nil {
		return fmt.Sprintf("%s-%d", prefix, time.Now().UnixNano())
	}
	return prefix + "-" + token[:12]
}

func sanitizeName(v string) string {
	v = strings.ToLower(v)
	v = strings.ReplaceAll(v, " ", "-")
	v = strings.ReplaceAll(v, "/", "-")
	if v == "" {
		v = "mcp-server"
	}
	if len(v) > 32 {
		v = v[:32]
	}
	return v
}

func max(a, b int) int {
	if a > b {
		return a
	}
	return b
}

func toBody(v any) io.Reader {
	b, _ := json.Marshal(v)
	return strings.NewReader(string(b))
}

func (s *Server) audit(ctx context.Context, actor, action, target, result string, metadata map[string]any) {
	event := domain.AuditEvent{
		ID:         newID("audit"),
		Actor:      actor,
		Action:     action,
		Target:     target,
		Result:     result,
		Metadata:   security.RedactMap(metadata),
		Redacted:   true,
		OccurredAt: time.Now().UTC(),
	}
	if _, err := s.repo.CreateAuditEvent(ctx, event); err != nil {
		log.Printf("{\"level\":\"warn\",\"msg\":\"audit insert failed\",\"error\":%q}", err.Error())
	}
}
