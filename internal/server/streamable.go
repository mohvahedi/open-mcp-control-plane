package server

import (
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"strings"
	"time"

	"github.com/mohvahedi/open-mcp-control-plane/internal/domain"
)

// JSON-RPC 2.0 + MCP streamable HTTP helpers.
// Supports initialize, tools/list, tools/call, ping, notifications/initialized,
// SSE event streams, and MCP session lifecycle (Mcp-Session-Id).

type jsonRPCRequest struct {
	JSONRPC string          `json:"jsonrpc"`
	ID      any             `json:"id,omitempty"`
	Method  string          `json:"method"`
	Params  json.RawMessage `json:"params,omitempty"`
}

type jsonRPCResponse struct {
	JSONRPC string        `json:"jsonrpc"`
	ID      any           `json:"id,omitempty"`
	Result  any           `json:"result,omitempty"`
	Error   *jsonRPCError `json:"error,omitempty"`
}

type jsonRPCError struct {
	Code    int    `json:"code"`
	Message string `json:"message"`
}

func (s *Server) registerStreamableGateway() {
	// Canonical MCP streamable HTTP endpoints (client-auth).
	s.mux.HandleFunc("POST /gateway/mcp", s.withClientAuth(s.streamableMCP))
	s.mux.HandleFunc("GET /gateway/mcp", s.withClientAuth(s.streamableMCPGet))
	s.mux.HandleFunc("DELETE /gateway/mcp", s.withClientAuth(s.streamableMCPDelete))
	// Profile-scoped aliases for clients that prefer path-based routing.
	s.mux.HandleFunc("POST /mcp/profiles/{name}", s.withClientAuth(s.streamableMCP))
	s.mux.HandleFunc("GET /mcp/profiles/{name}", s.withClientAuth(s.streamableMCPGet))
	s.mux.HandleFunc("DELETE /mcp/profiles/{name}", s.withClientAuth(s.streamableMCPDelete))
}

func wantsSSE(r *http.Request) bool {
	accept := r.Header.Get("Accept")
	return strings.Contains(accept, "text/event-stream")
}

// streamableMCPGet supports session probes, SSE streams, and capability discovery.
func (s *Server) streamableMCPGet(w http.ResponseWriter, r *http.Request, client domain.Client, profile domain.Profile) {
	if wantsSSE(r) {
		s.streamableSSE(w, r, client, profile)
		return
	}
	sessionID := r.Header.Get("Mcp-Session-Id")
	if sessionID != "" {
		if _, ok := s.sessions.get(sessionID); !ok {
			writeJSON(w, http.StatusNotFound, map[string]string{"error": "unknown session"})
			return
		}
		w.Header().Set("Mcp-Session-Id", sessionID)
	}
	w.Header().Set("Content-Type", "application/json")
	w.Header().Set("MCP-Protocol-Version", "2024-11-05")
	writeJSON(w, http.StatusOK, map[string]any{
		"protocolVersion": "2024-11-05",
		"serverInfo": map[string]any{
			"name":    "open-mcp-control-plane-gateway",
			"version": s.config.Version,
		},
		"capabilities": map[string]any{
			"tools":     map[string]any{"listChanged": false},
			"logging":   map[string]any{},
			"resources": map[string]any{},
		},
		"profile": map[string]any{
			"id":   profile.ID,
			"name": profile.Name,
		},
		"client_id":  client.ID,
		"transport":  "streamable-http+sse",
		"session_id": sessionID,
	})
}

func (s *Server) streamableMCPDelete(w http.ResponseWriter, r *http.Request, client domain.Client, profile domain.Profile) {
	sessionID := r.Header.Get("Mcp-Session-Id")
	if sessionID == "" {
		writeJSON(w, http.StatusBadRequest, map[string]string{"error": "Mcp-Session-Id required"})
		return
	}
	sess, ok := s.sessions.get(sessionID)
	if !ok {
		writeJSON(w, http.StatusNotFound, map[string]string{"error": "unknown session"})
		return
	}
	if sess.ClientID != client.ID {
		writeJSON(w, http.StatusForbidden, map[string]string{"error": "session belongs to another client"})
		return
	}
	_ = s.sessions.delete(sessionID)
	s.audit(r.Context(), "client", "mcp_session_end", sessionID, "ok", map[string]any{"client_id": client.ID, "profile_id": profile.ID})
	writeJSON(w, http.StatusOK, map[string]string{"status": "session_closed"})
}

func (s *Server) streamableSSE(w http.ResponseWriter, r *http.Request, client domain.Client, profile domain.Profile) {
	flusher, ok := w.(http.Flusher)
	if !ok {
		writeJSON(w, http.StatusInternalServerError, map[string]string{"error": "streaming unsupported"})
		return
	}
	sessionID := r.Header.Get("Mcp-Session-Id")
	var sess *mcpSession
	if sessionID != "" {
		sess, ok = s.sessions.get(sessionID)
		if !ok {
			writeJSON(w, http.StatusNotFound, map[string]string{"error": "unknown session"})
			return
		}
		if sess.ClientID != client.ID {
			writeJSON(w, http.StatusForbidden, map[string]string{"error": "session belongs to another client"})
			return
		}
	} else {
		sess = s.sessions.create(client, profile)
		sessionID = sess.ID
		s.audit(r.Context(), "client", "mcp_session_start", sessionID, "ok", map[string]any{"client_id": client.ID, "profile_id": profile.ID, "mode": "sse"})
	}

	w.Header().Set("Content-Type", "text/event-stream")
	w.Header().Set("Cache-Control", "no-cache")
	w.Header().Set("Connection", "keep-alive")
	w.Header().Set("Mcp-Session-Id", sessionID)
	w.Header().Set("MCP-Protocol-Version", "2024-11-05")
	w.WriteHeader(http.StatusOK)
	flusher.Flush()

	// endpoint event (MCP streamable HTTP convention)
	_, _ = fmt.Fprintf(w, "event: endpoint\ndata: /gateway/mcp\n\n")
	flusher.Flush()

	ch := sess.subscribe()
	defer sess.unsubscribe(ch)

	// heartbeat ticker
	tick := time.NewTicker(15 * time.Second)
	defer tick.Stop()

	ctx := r.Context()
	for {
		select {
		case <-ctx.Done():
			return
		case <-tick.C:
			_, _ = fmt.Fprintf(w, ": ping %d\n\n", time.Now().Unix())
			flusher.Flush()
		case msg, ok := <-ch:
			if !ok {
				return
			}
			_, _ = fmt.Fprintf(w, "event: message\ndata: %s\n\n", string(msg))
			flusher.Flush()
		}
	}
}

func (s *Server) streamableMCP(w http.ResponseWriter, r *http.Request, client domain.Client, profile domain.Profile) {
	body, err := io.ReadAll(io.LimitReader(r.Body, 1<<20))
	if err != nil {
		writeJSON(w, http.StatusBadRequest, map[string]string{"error": "invalid body"})
		return
	}
	defer r.Body.Close()

	trimmed := strings.TrimSpace(string(body))
	if trimmed == "" {
		writeJSON(w, http.StatusBadRequest, map[string]string{"error": "empty body"})
		return
	}

	// Attach or create session.
	sessionID := r.Header.Get("Mcp-Session-Id")
	var sess *mcpSession
	if sessionID != "" {
		var ok bool
		sess, ok = s.sessions.get(sessionID)
		if !ok {
			writeJSON(w, http.StatusNotFound, map[string]string{"error": "unknown session"})
			return
		}
		if sess.ClientID != client.ID {
			writeJSON(w, http.StatusForbidden, map[string]string{"error": "session belongs to another client"})
			return
		}
	}

	w.Header().Set("Content-Type", "application/json")
	w.Header().Set("MCP-Protocol-Version", "2024-11-05")

	if strings.HasPrefix(trimmed, "[") {
		var batch []jsonRPCRequest
		if err := json.Unmarshal(body, &batch); err != nil {
			writeJSONRPCError(w, nil, -32700, "parse error")
			return
		}
		// Create session on first initialize in batch if needed.
		for _, req := range batch {
			if req.Method == "initialize" && sess == nil {
				sess = s.sessions.create(client, profile)
				sessionID = sess.ID
				s.audit(r.Context(), "client", "mcp_session_start", sessionID, "ok", map[string]any{"client_id": client.ID})
			}
		}
		if sessionID != "" {
			w.Header().Set("Mcp-Session-Id", sessionID)
		}
		out := make([]jsonRPCResponse, 0, len(batch))
		for _, req := range batch {
			if resp, ok := s.handleJSONRPC(r, client, profile, req, sess); ok {
				out = append(out, resp)
				if sess != nil {
					if b, err := json.Marshal(resp); err == nil {
						sess.publish(b)
					}
				}
			}
		}
		_ = json.NewEncoder(w).Encode(out)
		return
	}

	var req jsonRPCRequest
	if err := json.Unmarshal(body, &req); err != nil {
		writeJSONRPCError(w, nil, -32700, "parse error")
		return
	}
	if req.Method == "initialize" && sess == nil {
		sess = s.sessions.create(client, profile)
		sessionID = sess.ID
		s.audit(r.Context(), "client", "mcp_session_start", sessionID, "ok", map[string]any{"client_id": client.ID})
	}
	if sessionID != "" {
		w.Header().Set("Mcp-Session-Id", sessionID)
	}
	resp, ok := s.handleJSONRPC(r, client, profile, req, sess)
	if !ok {
		w.WriteHeader(http.StatusAccepted)
		return
	}
	if sess != nil {
		if b, err := json.Marshal(resp); err == nil {
			sess.publish(b)
		}
	}
	_ = json.NewEncoder(w).Encode(resp)
}

func (s *Server) handleJSONRPC(r *http.Request, client domain.Client, profile domain.Profile, req jsonRPCRequest, sess *mcpSession) (jsonRPCResponse, bool) {
	if req.JSONRPC != "" && req.JSONRPC != "2.0" {
		return jsonRPCResponse{JSONRPC: "2.0", ID: req.ID, Error: &jsonRPCError{Code: -32600, Message: "invalid request"}}, true
	}
	isNotification := req.ID == nil && (req.Method == "notifications/initialized" || strings.HasPrefix(req.Method, "notifications/"))

	switch req.Method {
	case "initialize":
		s.audit(r.Context(), "client", "mcp_initialize", profile.ID, "ok", map[string]any{"client_id": client.ID})
		sessionMeta := map[string]any{}
		if sess != nil {
			sessionMeta["sessionId"] = sess.ID
		}
		return jsonRPCResponse{
			JSONRPC: "2.0",
			ID:      req.ID,
			Result: map[string]any{
				"protocolVersion": "2024-11-05",
				"capabilities": map[string]any{
					"tools": map[string]any{"listChanged": false},
				},
				"serverInfo": map[string]any{
					"name":    "open-mcp-control-plane-gateway",
					"version": s.config.Version,
				},
				"instructions": "Open MCP Control Plane unified gateway. Tools are namespaced as installationId.toolName and filtered by profile allowlist. Use Mcp-Session-Id and GET with Accept: text/event-stream for SSE.",
				"_meta":        sessionMeta,
			},
		}, true
	case "notifications/initialized":
		return jsonRPCResponse{}, false
	case "ping":
		return jsonRPCResponse{JSONRPC: "2.0", ID: req.ID, Result: map[string]any{}}, true
	case "tools/list":
		tools, _, err := s.collectTools(r.Context(), profile)
		if err != nil {
			return jsonRPCResponse{JSONRPC: "2.0", ID: req.ID, Error: &jsonRPCError{Code: -32000, Message: "failed to list tools"}}, true
		}
		mcpTools := make([]map[string]any, 0, len(tools))
		for _, t := range tools {
			name, _ := t["name"].(string)
			desc, _ := t["description"].(string)
			mcpTools = append(mcpTools, map[string]any{
				"name":        name,
				"description": desc,
				"inputSchema": map[string]any{
					"type":       "object",
					"properties": map[string]any{},
				},
			})
		}
		s.audit(r.Context(), "client", "mcp_tools_list", profile.ID, "ok", map[string]any{"client_id": client.ID, "count": len(mcpTools)})
		return jsonRPCResponse{JSONRPC: "2.0", ID: req.ID, Result: map[string]any{"tools": mcpTools}}, true
	case "tools/call":
		var params struct {
			Name      string         `json:"name"`
			Arguments map[string]any `json:"arguments"`
		}
		if len(req.Params) > 0 {
			_ = json.Unmarshal(req.Params, &params)
		}
		if params.Name == "" {
			return jsonRPCResponse{JSONRPC: "2.0", ID: req.ID, Error: &jsonRPCError{Code: -32602, Message: "tool name required"}}, true
		}
		_, toolMap, err := s.collectTools(r.Context(), profile)
		if err != nil {
			return jsonRPCResponse{JSONRPC: "2.0", ID: req.ID, Error: &jsonRPCError{Code: -32000, Message: "failed to resolve tools"}}, true
		}
		ref, ok := toolMap[params.Name]
		if !ok {
			return jsonRPCResponse{JSONRPC: "2.0", ID: req.ID, Error: &jsonRPCError{Code: -32001, Message: "tool not allowed"}}, true
		}
		// Progress notification over SSE when session is live.
		if sess != nil {
			prog, _ := json.Marshal(map[string]any{
				"jsonrpc": "2.0",
				"method":  "notifications/progress",
				"params": map[string]any{
					"progressToken": req.ID,
					"progress":      0.1,
					"total":         1,
					"message":       "invoking " + params.Name,
				},
			})
			sess.publish(prog)
		}
		payload := map[string]any{"tool": ref.ToolName, "args": params.Arguments}
		request, err := http.NewRequestWithContext(r.Context(), http.MethodPost, strings.TrimRight(ref.Endpoint, "/")+"/invoke", toBody(payload))
		if err != nil {
			return jsonRPCResponse{JSONRPC: "2.0", ID: req.ID, Error: &jsonRPCError{Code: -32002, Message: "invoke request failed"}}, true
		}
		request.Header.Set("Content-Type", "application/json")
		if deadline, ok := r.Context().Deadline(); ok {
			request.Header.Set("X-Request-Deadline", deadline.UTC().Format(time.RFC3339Nano))
		}
		resp, err := s.httpClient.Do(request)
		if err != nil {
			return jsonRPCResponse{JSONRPC: "2.0", ID: req.ID, Error: &jsonRPCError{Code: -32002, Message: "downstream invoke failed"}}, true
		}
		defer resp.Body.Close()
		if resp.StatusCode >= 400 {
			return jsonRPCResponse{JSONRPC: "2.0", ID: req.ID, Error: &jsonRPCError{Code: -32002, Message: "downstream invoke failed"}}, true
		}
		var out any
		if err := json.NewDecoder(resp.Body).Decode(&out); err != nil {
			return jsonRPCResponse{JSONRPC: "2.0", ID: req.ID, Error: &jsonRPCError{Code: -32002, Message: "invalid downstream response"}}, true
		}
		if sess != nil {
			prog, _ := json.Marshal(map[string]any{
				"jsonrpc": "2.0",
				"method":  "notifications/progress",
				"params": map[string]any{
					"progressToken": req.ID,
					"progress":      1,
					"total":         1,
					"message":       "completed " + params.Name,
				},
			})
			sess.publish(prog)
		}
		s.audit(r.Context(), "client", "mcp_tools_call", params.Name, "ok", map[string]any{"client_id": client.ID})
		text, _ := json.Marshal(out)
		return jsonRPCResponse{
			JSONRPC: "2.0",
			ID:      req.ID,
			Result: map[string]any{
				"content": []map[string]any{
					{"type": "text", "text": string(text)},
				},
				"structuredContent": out,
				"isError":           false,
			},
		}, true
	default:
		if isNotification {
			return jsonRPCResponse{}, false
		}
		return jsonRPCResponse{JSONRPC: "2.0", ID: req.ID, Error: &jsonRPCError{Code: -32601, Message: "method not found: " + req.Method}}, true
	}
}

func writeJSONRPCError(w http.ResponseWriter, id any, code int, message string) {
	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(http.StatusOK)
	_ = json.NewEncoder(w).Encode(jsonRPCResponse{
		JSONRPC: "2.0",
		ID:      id,
		Error:   &jsonRPCError{Code: code, Message: message},
	})
}
