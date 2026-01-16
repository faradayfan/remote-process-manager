package server

import (
	"context"
	"encoding/json"
	"net/http"
	"time"

	"github.com/faradayfan/remote-process-manager/internal/protocol"
)

type HTTPServer struct {
	addr     string
	registry *Registry
}

func NewHTTPServer(addr string, registry *Registry) *HTTPServer {
	if addr == "" {
		addr = "127.0.0.1:8080"
	}
	return &HTTPServer{
		addr:     addr,
		registry: registry,
	}
}

func (s *HTTPServer) Addr() string { return s.addr }

func (s *HTTPServer) Handler() http.Handler {
	mux := http.NewServeMux()

	// Agent registry
	mux.HandleFunc("GET /agents", s.handleListAgents)
	mux.HandleFunc("GET /agents/{agentID}", s.handleGetAgent)

	// Commands to agents (relay)
	mux.HandleFunc("POST /agents/{agentID}/servers/{server}/start", s.handleStart)
	mux.HandleFunc("POST /agents/{agentID}/servers/{server}/stop", s.handleStop)
	mux.HandleFunc("GET /agents/{agentID}/servers/{server}/status", s.handleStatus)
	mux.HandleFunc("GET /agents/{agentID}/instances", s.handleInstancesList)
	mux.HandleFunc("POST /agents/{agentID}/instances/create", s.handleInstancesCreate)
	mux.HandleFunc("POST /agents/{agentID}/instances/delete", s.handleInstancesDelete)

	// Health
	mux.HandleFunc("GET /healthz", func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusOK)
		_, _ = w.Write([]byte("ok"))
	})

	return mux
}

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

func (s *HTTPServer) handleStart(w http.ResponseWriter, r *http.Request) {
	s.command(w, r, protocol.CmdStart)
}

func (s *HTTPServer) handleStop(w http.ResponseWriter, r *http.Request) {
	s.command(w, r, protocol.CmdStop)
}

func (s *HTTPServer) handleStatus(w http.ResponseWriter, r *http.Request) {
	s.command(w, r, protocol.CmdStatus)
}

func (s *HTTPServer) command(w http.ResponseWriter, r *http.Request, cmdType string) {
	agentID := r.PathValue("agentID")
	serverName := r.PathValue("server")

	if agentID == "" {
		writeErr(w, http.StatusBadRequest, "missing agentID")
		return
	}
	if serverName == "" {
		writeErr(w, http.StatusBadRequest, "missing server name")
		return
	}

	ctx, cancel := context.WithTimeout(r.Context(), 10*time.Second)
	defer cancel()

	resp, err := s.registry.SendCommand(ctx, agentID, cmdType, protocol.ServerTarget{
		Server: serverName,
	})
	if err != nil {
		writeErr(w, http.StatusBadRequest, err.Error())
		return
	}

	// If agent returned an error, bubble it up cleanly
	if resp.Error != "" {
		writeJSON(w, http.StatusBadRequest, map[string]any{
			"agent_id": agentID,
			"server":   serverName,
			"error":    resp.Error,
		})
		return
	}

	// If payload is empty, just return a generic ok
	if len(resp.Payload) == 0 {
		writeJSON(w, http.StatusOK, map[string]any{
			"agent_id": agentID,
			"server":   serverName,
			"ok":       true,
		})
		return
	}

	// Otherwise return payload as JSON
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
