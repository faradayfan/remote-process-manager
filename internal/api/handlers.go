package api

import (
	"fmt"
	"net/http"

	// Update this import to match your module path
	"github.com/faradayfan/remote-process-manager/internal/manager"
)

func (s *Server) handleListServers(w http.ResponseWriter, r *http.Request) {
	// Show all servers from config, even if never started yet.
	resp := ListResponse{Servers: make([]StartStopResponse, 0, len(s.configs))}

	for name := range s.configs {
		st := s.mgr.Status(name)
		resp.Servers = append(resp.Servers, stateToResponse(st, s.logPathFor(name)))
	}

	writeJSON(w, http.StatusOK, resp)
}

func (s *Server) handleGetServer(w http.ResponseWriter, r *http.Request) {
	name := r.PathValue("name")
	if name == "" {
		writeError(w, http.StatusBadRequest, "missing server name")
		return
	}

	if _, ok := s.configs[name]; !ok {
		writeError(w, http.StatusNotFound, fmt.Sprintf("unknown server: %s", name))
		return
	}

	st := s.mgr.Status(name)
	writeJSON(w, http.StatusOK, stateToResponse(st, s.logPathFor(name)))
}

func (s *Server) handleStartServer(w http.ResponseWriter, r *http.Request) {
	name := r.PathValue("name")
	if name == "" {
		writeError(w, http.StatusBadRequest, "missing server name")
		return
	}

	cfg, ok := s.configs[name]
	if !ok {
		writeError(w, http.StatusNotFound, fmt.Sprintf("unknown server: %s", name))
		return
	}

	if err := s.ensureLogDir(); err != nil {
		writeError(w, http.StatusInternalServerError, fmt.Sprintf("failed to create log dir: %v", err))
		return
	}

	logPath := s.logPathFor(name)
	st, err := s.mgr.Start(cfg, logPath)
	if err != nil {
		// if already running, return current state + conflict
		cur := s.mgr.Status(name)
		writeJSON(w, http.StatusConflict, stateToResponse(cur, logPath))
		return
	}

	writeJSON(w, http.StatusOK, stateToResponse(st, logPath))
}

func (s *Server) handleStopServer(w http.ResponseWriter, r *http.Request) {
	name := r.PathValue("name")
	if name == "" {
		writeError(w, http.StatusBadRequest, "missing server name")
		return
	}

	if _, ok := s.configs[name]; !ok {
		writeError(w, http.StatusNotFound, fmt.Sprintf("unknown server: %s", name))
		return
	}

	st, err := s.mgr.Stop(name)
	if err != nil {
		// If it's already stopped, return the current state but as 409-ish info
		cur := s.mgr.Status(name)
		writeJSON(w, http.StatusConflict, stateToResponse(cur, s.logPathFor(name)))
		return
	}

	writeJSON(w, http.StatusOK, stateToResponse(st, s.logPathFor(name)))
}

// compile-time assertion (optional)
var _ = manager.ServerState{}
