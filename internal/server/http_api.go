package server

import (
	"context"
	"encoding/json"
	"net/http"
	"strings"
	"time"

	"github.com/faradayfan/remote-process-manager/internal/protocol"
)

type HTTPServer struct {
	addr     string
	registry *Registry
	auth     *AuthMiddleware // NEW
}

func NewHTTPServer(addr string, registry *Registry, auth *AuthMiddleware) *HTTPServer {
	if addr == "" {
		addr = "127.0.0.1:8080"
	}
	return &HTTPServer{
		addr:     addr,
		registry: registry,
		auth:     auth, // NEW
	}
}

func (s *HTTPServer) Addr() string { return s.addr }

func (s *HTTPServer) Handler() http.Handler {
	mux := http.NewServeMux()

	// Read-only endpoints (RoleRead or RoleAdmin)
	mux.Handle("GET /v1/agents", s.auth.Wrap(RoleRead, s.handleListAgents))
	mux.Handle("GET /v1/agents/{agentID}", s.auth.Wrap(RoleRead, s.handleGetAgent))
	mux.Handle("GET /v1/agents/{agentID}/instances", s.auth.Wrap(RoleRead, s.handleInstancesList))
	mux.Handle("GET /v1/agents/{agentID}/instances/{instance}/status", s.auth.Wrap(RoleRead, s.handleInstanceStatus))
	mux.Handle("GET /v1/agents/{agentID}/templates", s.auth.Wrap(RoleRead, s.handleTemplatesList))
	mux.Handle("GET /v1/agents/{agentID}/templates/{templateName}", s.auth.Wrap(RoleRead, s.handleTemplatesInspect))

	// Write endpoints (RoleAdmin only)
	mux.Handle("POST /v1/agents/{agentID}/instances/{instance}/start", s.auth.Wrap(RoleAdmin, s.handleInstanceStart))
	mux.Handle("POST /v1/agents/{agentID}/instances/{instance}/stop", s.auth.Wrap(RoleAdmin, s.handleInstanceStop))
	mux.Handle("POST /v1/agents/{agentID}/instances/{name}/disable", s.auth.Wrap(RoleAdmin, s.handleInstanceDisable))
	mux.Handle("POST /v1/agents/{agentID}/instances/{name}/enable", s.auth.Wrap(RoleAdmin, s.handleInstanceEnable))
	mux.Handle("POST /v1/agents/{agentID}/instances/{name}/params/set", s.auth.Wrap(RoleAdmin, s.handleInstanceParamsSet))
	mux.Handle("POST /v1/agents/{agentID}/instances/{name}/params/unset", s.auth.Wrap(RoleAdmin, s.handleInstanceParamsUnset))
	mux.Handle("POST /v1/agents/{agentID}/instances/{name}/rename", s.auth.Wrap(RoleAdmin, s.handleInstanceRename))
	mux.Handle("POST /v1/agents/{agentID}/instances/create", s.auth.Wrap(RoleAdmin, s.handleInstancesCreate))
	mux.Handle("POST /v1/agents/{agentID}/instances/delete", s.auth.Wrap(RoleAdmin, s.handleInstancesDelete))

	// Health check - no auth required
	mux.Handle("GET /healthz", s.auth.WrapNoAuth(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusOK)
		_, _ = w.Write([]byte("ok"))
	}))

	return mux
}

// ... rest of the file remains unchanged ...
func writeJSON(w http.ResponseWriter, status int, v any) {
	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(status)
	_ = json.NewEncoder(w).Encode(v)
}

func writeErr(w http.ResponseWriter, status int, msg string) {
	writeJSON(w, status, map[string]string{"error": msg})
}

func (s *HTTPServer) handleListAgents(w http.ResponseWriter, r *http.Request) {
	writeJSON(w, http.StatusOK, map[string]any{
		"agents": s.registry.ListAgents(),
	})
}

func (s *HTTPServer) handleGetAgent(w http.ResponseWriter, r *http.Request) {
	id := r.PathValue("agentID")
	info, ok := s.registry.GetAgent(id)
	if !ok {
		writeErr(w, http.StatusNotFound, "agent not connected")
		return
	}
	writeJSON(w, http.StatusOK, info)
}

func (s *HTTPServer) handleInstanceStart(w http.ResponseWriter, r *http.Request) {
	s.command(w, r, protocol.CmdStart)
}

func (s *HTTPServer) handleInstanceStop(w http.ResponseWriter, r *http.Request) {
	s.command(w, r, protocol.CmdStop)
}

func (s *HTTPServer) handleInstanceStatus(w http.ResponseWriter, r *http.Request) {
	s.command(w, r, protocol.CmdStatus)
}

func (s *HTTPServer) command(w http.ResponseWriter, r *http.Request, cmdType string) {
	agentID := r.PathValue("agentID")
	instanceName := r.PathValue("instance")

	if agentID == "" {
		writeErr(w, http.StatusBadRequest, "missing agentID")
		return
	}
	if instanceName == "" {
		writeErr(w, http.StatusBadRequest, "missing instance name")
		return
	}

	ctx, cancel := context.WithTimeout(r.Context(), 10*time.Second)
	defer cancel()

	resp, err := s.registry.SendCommand(ctx, agentID, cmdType, protocol.InstanceTarget{
		Instance: instanceName,
	})
	if err != nil {
		writeErr(w, http.StatusBadRequest, err.Error())
		return
	}

	if resp.Error != "" {
		writeJSON(w, http.StatusBadRequest, map[string]any{
			"agent_id": agentID,
			"instance": instanceName,
			"error":    resp.Error,
		})
		return
	}

	if len(resp.Payload) == 0 {
		writeJSON(w, http.StatusOK, map[string]any{
			"agent_id": agentID,
			"instance": instanceName,
			"ok":       true,
		})
		return
	}

	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(http.StatusOK)
	_, _ = w.Write(resp.Payload)
}

func (s *HTTPServer) handleInstancesList(w http.ResponseWriter, r *http.Request) {
	agentID := r.PathValue("agentID")
	if agentID == "" {
		writeErr(w, http.StatusBadRequest, "missing agentID")
		return
	}

	ctx, cancel := context.WithTimeout(r.Context(), 10*time.Second)
	defer cancel()

	resp, err := s.registry.SendCommand(ctx, agentID, protocol.CmdInstancesList, map[string]any{})
	if err != nil {
		writeErr(w, http.StatusBadRequest, err.Error())
		return
	}
	if resp.Error != "" {
		writeErr(w, http.StatusBadRequest, resp.Error)
		return
	}

	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(http.StatusOK)
	_, _ = w.Write(resp.Payload)
}

func (s *HTTPServer) handleInstancesCreate(w http.ResponseWriter, r *http.Request) {
	agentID := r.PathValue("agentID")
	if agentID == "" {
		writeErr(w, http.StatusBadRequest, "missing agentID")
		return
	}

	var req protocol.CreateInstanceRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		writeErr(w, http.StatusBadRequest, "invalid json body")
		return
	}

	ctx, cancel := context.WithTimeout(r.Context(), 10*time.Second)
	defer cancel()

	resp, err := s.registry.SendCommand(ctx, agentID, protocol.CmdInstancesCreate, req)
	if err != nil {
		writeErr(w, http.StatusBadRequest, err.Error())
		return
	}
	if resp.Error != "" {
		writeErr(w, http.StatusBadRequest, resp.Error)
		return
	}

	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(http.StatusOK)
	_, _ = w.Write(resp.Payload)
}

func (s *HTTPServer) handleInstancesDelete(w http.ResponseWriter, r *http.Request) {
	agentID := r.PathValue("agentID")
	if agentID == "" {
		writeErr(w, http.StatusBadRequest, "missing agentID")
		return
	}

	var req protocol.DeleteInstanceRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		writeErr(w, http.StatusBadRequest, "invalid json body")
		return
	}

	ctx, cancel := context.WithTimeout(r.Context(), 10*time.Second)
	defer cancel()

	resp, err := s.registry.SendCommand(ctx, agentID, protocol.CmdInstancesDelete, req)
	if err != nil {
		writeErr(w, http.StatusBadRequest, err.Error())
		return
	}
	if resp.Error != "" {
		writeErr(w, http.StatusBadRequest, resp.Error)
		return
	}

	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(http.StatusOK)
	_, _ = w.Write(resp.Payload)
}

func (s *HTTPServer) handleTemplatesList(w http.ResponseWriter, r *http.Request) {
	agentID := r.PathValue("agentID")
	if agentID == "" {
		writeErr(w, http.StatusBadRequest, "missing agentID")
		return
	}

	ctx, cancel := context.WithTimeout(r.Context(), 10*time.Second)
	defer cancel()

	resp, err := s.registry.SendCommand(ctx, agentID, protocol.CmdTemplatesList, map[string]any{})
	if err != nil {
		writeErr(w, http.StatusBadGateway, err.Error())
		return
	}
	if resp.Error != "" {
		writeErr(w, http.StatusBadRequest, resp.Error)
		return
	}

	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(http.StatusOK)
	_, _ = w.Write(resp.Payload)
}

func (s *HTTPServer) handleTemplatesInspect(w http.ResponseWriter, r *http.Request) {
	agentID := r.PathValue("agentID")
	if agentID == "" {
		writeErr(w, http.StatusBadRequest, "missing agentID")
		return
	}

	templateName := r.PathValue("templateName")
	if templateName == "" {
		writeErr(w, http.StatusBadRequest, "missing templateName")
		return
	}

	ctx, cancel := context.WithTimeout(r.Context(), 10*time.Second)
	defer cancel()

	req := protocol.TemplatesInspectRequest{
		Name: templateName,
	}

	resp, err := s.registry.SendCommand(ctx, agentID, protocol.CmdTemplatesInspect, req)
	if err != nil {
		writeErr(w, http.StatusBadGateway, err.Error())
		return
	}
	if resp.Error != "" {
		writeErr(w, http.StatusBadRequest, resp.Error)
		return
	}

	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(http.StatusOK)
	_, _ = w.Write(resp.Payload)
}

func (s *HTTPServer) handleInstanceEnable(w http.ResponseWriter, r *http.Request) {
	s.instanceToggleEnabled(w, r, true)
}

func (s *HTTPServer) handleInstanceDisable(w http.ResponseWriter, r *http.Request) {
	s.instanceToggleEnabled(w, r, false)
}

func (s *HTTPServer) instanceToggleEnabled(w http.ResponseWriter, r *http.Request, enabled bool) {
	agentID := r.PathValue("agentID")
	name := r.PathValue("name")

	if agentID == "" {
		writeErr(w, http.StatusBadRequest, "missing agentID")
		return
	}
	if name == "" {
		writeErr(w, http.StatusBadRequest, "missing instance name")
		return
	}

	ctx, cancel := context.WithTimeout(r.Context(), 10*time.Second)
	defer cancel()

	req := protocol.ToggleInstanceEnabledRequest{
		Name:    name,
		Enabled: enabled,
	}

	cmdType := protocol.CmdInstancesDisable
	if enabled {
		cmdType = protocol.CmdInstancesEnable
	}

	resp, err := s.registry.SendCommand(ctx, agentID, cmdType, req)
	if err != nil {
		writeErr(w, http.StatusBadGateway, err.Error())
		return
	}
	if resp.Error != "" {
		writeErr(w, http.StatusBadRequest, resp.Error)
		return
	}

	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(http.StatusOK)
	_, _ = w.Write(resp.Payload)
}

func (s *HTTPServer) handleInstanceParamsSet(w http.ResponseWriter, r *http.Request) {
	agentID := r.PathValue("agentID")
	name := r.PathValue("name")

	if agentID == "" {
		writeErr(w, http.StatusBadRequest, "missing agentID")
		return
	}
	if name == "" {
		writeErr(w, http.StatusBadRequest, "missing instance name")
		return
	}

	var body map[string]string
	if err := json.NewDecoder(r.Body).Decode(&body); err != nil {
		writeErr(w, http.StatusBadRequest, "invalid json body (expected object of key/value pairs)")
		return
	}

	ctx, cancel := context.WithTimeout(r.Context(), 10*time.Second)
	defer cancel()

	req := protocol.InstancesParamsSetRequest{
		Name: name,
		Set:  body,
	}

	resp, err := s.registry.SendCommand(ctx, agentID, protocol.CmdInstancesParamsSet, req)
	if err != nil {
		writeErr(w, http.StatusBadGateway, err.Error())
		return
	}
	if resp.Error != "" {
		writeErr(w, http.StatusBadRequest, resp.Error)
		return
	}

	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(http.StatusOK)
	_, _ = w.Write(resp.Payload)
}

func (s *HTTPServer) handleInstanceParamsUnset(w http.ResponseWriter, r *http.Request) {
	agentID := r.PathValue("agentID")
	name := r.PathValue("name")

	if agentID == "" {
		writeErr(w, http.StatusBadRequest, "missing agentID")
		return
	}
	if name == "" {
		writeErr(w, http.StatusBadRequest, "missing instance name")
		return
	}

	var body struct {
		Unset []string `json:"unset"`
	}
	if err := json.NewDecoder(r.Body).Decode(&body); err != nil {
		writeErr(w, http.StatusBadRequest, "invalid json body")
		return
	}

	ctx, cancel := context.WithTimeout(r.Context(), 10*time.Second)
	defer cancel()

	req := protocol.InstancesParamsUnsetRequest{
		Name:  name,
		Unset: body.Unset,
	}

	resp, err := s.registry.SendCommand(ctx, agentID, protocol.CmdInstancesParamsUnset, req)
	if err != nil {
		writeErr(w, http.StatusBadGateway, err.Error())
		return
	}
	if resp.Error != "" {
		writeErr(w, http.StatusBadRequest, resp.Error)
		return
	}

	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(http.StatusOK)
	_, _ = w.Write(resp.Payload)
}

func (s *HTTPServer) handleInstanceRename(w http.ResponseWriter, r *http.Request) {
	agentID := r.PathValue("agentID")
	name := r.PathValue("name")

	if agentID == "" {
		writeErr(w, http.StatusBadRequest, "missing agentID")
		return
	}
	if name == "" {
		writeErr(w, http.StatusBadRequest, "missing instance name")
		return
	}

	var body struct {
		NewName string `json:"new_name"`
	}
	if err := json.NewDecoder(r.Body).Decode(&body); err != nil {
		writeErr(w, http.StatusBadRequest, "invalid json body")
		return
	}
	if strings.TrimSpace(body.NewName) == "" {
		writeErr(w, http.StatusBadRequest, "new_name is required")
		return
	}

	ctx, cancel := context.WithTimeout(r.Context(), 10*time.Second)
	defer cancel()

	req := protocol.InstancesRenameRequest{
		Name:    name,
		NewName: body.NewName,
	}

	resp, err := s.registry.SendCommand(ctx, agentID, protocol.CmdInstancesRename, req)
	if err != nil {
		writeErr(w, http.StatusBadGateway, err.Error())
		return
	}
	if resp.Error != "" {
		writeErr(w, http.StatusBadRequest, resp.Error)
		return
	}

	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(http.StatusOK)
	_, _ = w.Write(resp.Payload)
}
