package api

import (
	"encoding/json"
	"fmt"
	"net/http"
	"os"
	"path/filepath"
	"strings"

	// Update this import to match your module path
	"github.com/faradayfan/remote-process-manager/internal/manager"
)

type Server struct {
	mgr     *manager.Manager
	configs map[string]manager.ServerConfig

	logDir string
	addr   string
}

func NewServer(mgr *manager.Manager, configs map[string]manager.ServerConfig, addr string, logDir string) *Server {
	if addr == "" {
		addr = "127.0.0.1:8080"
	}
	if logDir == "" {
		logDir = "logs"
	}

	return &Server{
		mgr:     mgr,
		configs: configs,
		addr:    addr,
		logDir:  logDir,
	}
}

func (s *Server) Addr() string { return s.addr }

func (s *Server) Handler() http.Handler {
	mux := http.NewServeMux()

	// collection routes
	mux.HandleFunc("GET /servers", s.handleListServers)

	// item routes (Go 1.22+ pattern matching)
	mux.HandleFunc("GET /servers/{name}", s.handleGetServer)
	mux.HandleFunc("POST /servers/{name}/start", s.handleStartServer)
	mux.HandleFunc("POST /servers/{name}/stop", s.handleStopServer)

	// cheap health endpoint
	mux.HandleFunc("GET /healthz", func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusOK)
		_, _ = w.Write([]byte("ok"))
	})

	return s.wrapJSON(mux)
}

func (s *Server) ensureLogDir() error {
	return os.MkdirAll(s.logDir, 0755)
}

func (s *Server) logPathFor(serverName string) string {
	safe := strings.ReplaceAll(serverName, string(os.PathSeparator), "_")
	return filepath.Join(s.logDir, fmt.Sprintf("%s.log", safe))
}

func writeJSON(w http.ResponseWriter, status int, v any) {
	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(status)
	_ = json.NewEncoder(w).Encode(v)
}

func writeError(w http.ResponseWriter, status int, msg string) {
	writeJSON(w, status, ErrorResponse{Error: msg})
}

func (s *Server) wrapJSON(next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		// basic JSON default headers could go here later (CORS, etc.)
		next.ServeHTTP(w, r)
	})
}

func stateToResponse(st manager.ServerState, logPath string) StartStopResponse {
	return StartStopResponse{
		Server:    st.Name,
		Running:   st.Running,
		PID:       st.PID,
		StartedAt: st.StartedAt,
		ExitedAt:  st.ExitedAt,
		ExitCode:  st.ExitCode,
		LastError: st.LastError,
		LogPath:   logPath,
	}
}
