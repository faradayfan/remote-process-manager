package main

import (
	"context"
	"log"
	"net/http"
	"os"
	"os/signal"
	"syscall"
	"time"

	"github.com/faradayfan/remote-process-manager/internal/server"
)

func main() {
	log.SetFlags(log.LstdFlags | log.Lmicroseconds)

	// Shared in-memory registry of connected agents
	reg := server.NewRegistry()

	// 1) TCP listener for agents (outbound agent -> cloud)
	agentAddr := "0.0.0.0:9090"
	agentListener := server.NewAgentListener(agentAddr, reg)

	go func() {
		if err := agentListener.ListenAndServe(); err != nil {
			log.Fatalf("[command-server] agent listener failed: %v", err)
		}
	}()

	// 2) HTTP API for CLI/users (cloud control plane)
	httpAddr := "0.0.0.0:8080"
	api := server.NewHTTPServer(httpAddr, reg)

	httpSrv := &http.Server{
		Addr:    api.Addr(),
		Handler: api.Handler(),
	}

	go func() {
		log.Printf("[command-server] http api listening on http://%s", httpSrv.Addr)
		if err := httpSrv.ListenAndServe(); err != nil && err != http.ErrServerClosed {
			log.Fatalf("[command-server] http server failed: %v", err)
		}
	}()

	// shutdown handling
	sigCh := make(chan os.Signal, 1)
	signal.Notify(sigCh, os.Interrupt, syscall.SIGTERM)
	sig := <-sigCh

	log.Printf("[command-server] received %v, shutting down...", sig)

	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()

	if err := httpSrv.Shutdown(ctx); err != nil {
		log.Printf("[command-server] http shutdown error: %v", err)
	}

	log.Printf("[command-server] stopped cleanly")
}
