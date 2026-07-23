package server

import (
	"errors"
	"net/http"

	"github.com/mohvahedi/open-mcp-control-plane/internal/domain"
	"github.com/mohvahedi/open-mcp-control-plane/internal/store"
)

func (s *Server) listSkills(w http.ResponseWriter, r *http.Request) {
	items, err := s.repo.ListSkills(r.Context())
	if err != nil {
		writeJSON(w, http.StatusInternalServerError, map[string]string{"error": "failed to list skills"})
		return
	}
	writeJSON(w, http.StatusOK, map[string]any{"items": items})
}

func (s *Server) listSkillsPublic(w http.ResponseWriter, r *http.Request) {
	s.listSkills(w, r)
}

func (s *Server) createSkill(w http.ResponseWriter, r *http.Request) {
	var skill domain.Skill
	if !decodeJSON(w, r, &skill) {
		return
	}
	if skill.Name == "" {
		writeJSON(w, http.StatusBadRequest, map[string]string{"error": "name is required"})
		return
	}
	if skill.Source == "" {
		skill.Source = "local"
	}
	if skill.Kind == "" {
		skill.Kind = "prompt"
	}
	if skill.ID == "" {
		skill.ID = newID("skill")
	}
	stored, err := s.repo.CreateSkill(r.Context(), skill)
	if err != nil {
		writeJSON(w, http.StatusBadRequest, map[string]string{"error": "failed to create skill"})
		return
	}
	s.audit(r.Context(), "admin", "create_skill", stored.ID, "ok", map[string]any{"name": stored.Name, "kind": stored.Kind})
	writeJSON(w, http.StatusCreated, stored)
}

func (s *Server) getSkill(w http.ResponseWriter, r *http.Request) {
	skill, err := s.repo.GetSkill(r.Context(), r.PathValue("id"))
	if err != nil {
		if errors.Is(err, store.ErrNotFound) {
			writeJSON(w, http.StatusNotFound, map[string]string{"error": "skill not found"})
			return
		}
		writeJSON(w, http.StatusInternalServerError, map[string]string{"error": "failed to get skill"})
		return
	}
	writeJSON(w, http.StatusOK, skill)
}

func (s *Server) deleteSkill(w http.ResponseWriter, r *http.Request) {
	id := r.PathValue("id")
	if err := s.repo.DeleteSkill(r.Context(), id); err != nil {
		if errors.Is(err, store.ErrNotFound) {
			writeJSON(w, http.StatusNotFound, map[string]string{"error": "skill not found"})
			return
		}
		writeJSON(w, http.StatusInternalServerError, map[string]string{"error": "failed to delete skill"})
		return
	}
	s.audit(r.Context(), "admin", "delete_skill", id, "ok", nil)
	writeJSON(w, http.StatusOK, map[string]string{"status": "deleted"})
}

func (s *Server) bindSkill(w http.ResponseWriter, r *http.Request) {
	var req struct {
		ProfileID string `json:"profile_id"`
		SkillID   string `json:"skill_id"`
		Enabled   *bool  `json:"enabled"`
	}
	if !decodeJSON(w, r, &req) {
		return
	}
	if req.ProfileID == "" || req.SkillID == "" {
		writeJSON(w, http.StatusBadRequest, map[string]string{"error": "profile_id and skill_id are required"})
		return
	}
	if _, err := s.repo.GetProfile(r.Context(), req.ProfileID); err != nil {
		writeJSON(w, http.StatusBadRequest, map[string]string{"error": "profile not found"})
		return
	}
	if _, err := s.repo.GetSkill(r.Context(), req.SkillID); err != nil {
		writeJSON(w, http.StatusBadRequest, map[string]string{"error": "skill not found"})
		return
	}
	enabled := true
	if req.Enabled != nil {
		enabled = *req.Enabled
	}
	b := domain.SkillBinding{ID: newID("sbind"), ProfileID: req.ProfileID, SkillID: req.SkillID, Enabled: enabled}
	stored, err := s.repo.BindSkill(r.Context(), b)
	if err != nil {
		writeJSON(w, http.StatusBadRequest, map[string]string{"error": "failed to bind skill"})
		return
	}
	s.audit(r.Context(), "admin", "bind_skill", stored.ID, "ok", map[string]any{"profile_id": stored.ProfileID, "skill_id": stored.SkillID})
	writeJSON(w, http.StatusCreated, stored)
}

func (s *Server) listSkillBindings(w http.ResponseWriter, r *http.Request) {
	profileID := r.URL.Query().Get("profile_id")
	items, err := s.repo.ListSkillBindings(r.Context(), profileID)
	if err != nil {
		writeJSON(w, http.StatusInternalServerError, map[string]string{"error": "failed to list skill bindings"})
		return
	}
	writeJSON(w, http.StatusOK, map[string]any{"items": items})
}

func (s *Server) unbindSkill(w http.ResponseWriter, r *http.Request) {
	id := r.PathValue("id")
	if err := s.repo.UnbindSkill(r.Context(), id); err != nil {
		if errors.Is(err, store.ErrNotFound) {
			writeJSON(w, http.StatusNotFound, map[string]string{"error": "binding not found"})
			return
		}
		writeJSON(w, http.StatusInternalServerError, map[string]string{"error": "failed to unbind skill"})
		return
	}
	s.audit(r.Context(), "admin", "unbind_skill", id, "ok", nil)
	writeJSON(w, http.StatusOK, map[string]string{"status": "unbound"})
}
