package server

import (
	"encoding/json"
	"io"
	"net/http"
	"strings"
	"time"

	"github.com/mohvahedi/open-mcp-control-plane/internal/domain"
)

// JSON-RPC 2.0 + MCP streamable HTTP helpers.
// Supports initialize, tools/list, tools/call, ping, and notifications/initialized.

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
	// Profile-scoped aliases for clients that prefer path-based routing.
	s.mux.HandleFunc("POST /mcp/profiles/{name}", s.withClientAuth(s.streamableMCP))
	s.mux.HandleFunc("GET /mcp/profiles/{name}", s.withClientAuth(s.streamableMCPGet))
}

// streamableMCPGet supports session probes / capability discovery without a body.
func (s *Server) streamableMCPGet(w http.ResponseWriter, r *http.Request, client domain.Client, profile domain.Profile) {
	w.Header().Set("Content-Type", "application/json")
	w.Header().Set("MCP-Protocol-Version", "2024-11-05")
	writeJSON(w, http.StatusOK, map[string]any{
		"protocolVersion": "2024-11-05",
		"serverInfo": map[string]any{
			"name":    "open-mcp-control-plane-gateway",
			"version": s.config.Version,
		},
		"capabilities": map[string]any{
			"tools": map[string]any{"listChanged": false},
		},
		"profile": map[string]any{
			"id":   profile.ID,
			"name": profile.Name,
		},
		"client_id": client.ID,
		"transport": "streamable-http",
	})
}

func (s *Server) streamableMCP(w http.ResponseWriter, r *http.Request, client domain.Client, profile domain.Profile) {
	body, err := io.ReadAll(io.LimitReader(r.Body, 1<<20))
	if err != nil {
		writeJSON(w, http.StatusBadRequest, map[string]string{"error": "invalid body"})
		return
	}
	defer r.Body.Close()

	// Accept either a single JSON-RPC object or a batch array.
	trimmed := strings.TrimSpace(string(body))
	if trimmed == "" {
		writeJSON(w, http.StatusBadRequest, map[string]string{"error": "empty body"})
		return
	}

	w.Header().Set("Content-Type", "application/json")
	w.Header().Set("MCP-Protocol-Version", "2024-11-05")

	if strings.HasPrefix(trimmed, "[") {
		var batch []jsonRPCRequest
		if err := json.Unmarshal(body, &batch); err != nil {
			writeJSONRPCError(w, nil, -32700, "parse error")
			return
		}
		out := make([]jsonRPCResponse, 0, len(batch))
		for _, req := range batch {
			if resp, ok := s.handleJSONRPC(r, client, profile, req); ok {
				out = append(out, resp)
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
	resp, ok := s.handleJSONRPC(r, client, profile, req)
	if !ok {
		// Notification — no content response is valid for JSON-RPC notifications.
		w.WriteHeader(http.StatusAccepted)
		return
	}
	_ = json.NewEncoder(w).Encode(resp)
}

func (s *Server) handleJSONRPC(r *http.Request, client domain.Client, profile domain.Profile, req jsonRPCRequest) (jsonRPCResponse, bool) {
	if req.JSONRPC != "" && req.JSONRPC != "2.0" {
		return jsonRPCResponse{JSONRPC: "2.0", ID: req.ID, Error: &jsonRPCError{Code: -32600, Message: "invalid request"}}, true
	}
	// Notifications have no id.
	isNotification := req.ID == nil && (req.Method == "notifications/initialized" || strings.HasPrefix(req.Method, "notifications/"))

	switch req.Method {
	case "initialize":
		s.audit(r.Context(), "client", "mcp_initialize", profile.ID, "ok", map[string]any{"client_id": client.ID})
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
				"instructions": "Open MCP Control Plane unified gateway. Tools are namespaced as installationId.toolName and filtered by profile allowlist.",
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
		s.audit(r.Context(), "client", "mcp_tools_call", params.Name, "ok", map[string]any{"client_id": client.ID})
		// MCP tools/call result shape
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
	w.WriteHeader(http.StatusOK) // JSON-RPC errors still use 200 at transport layer unless parse/auth failed earlier
	_ = json.NewEncoder(w).Encode(jsonRPCResponse{
		JSONRPC: "2.0",
		ID:      id,
		Error:   &jsonRPCError{Code: code, Message: message},
	})
}
