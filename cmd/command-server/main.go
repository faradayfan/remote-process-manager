package main

import (
	"context"
	"flag"
	"log"
	"net/http"
	"os"
	"os/signal"
	"syscall"
	"time"

	"github.com/faradayfan/remote-process-manager/internal/config"
	"github.com/faradayfan/remote-process-manager/internal/server"
)

func main() {
	log.SetFlags(log.LstdFlags | log.Lmicroseconds)

	var cfgPath string
	flag.StringVar(&cfgPath, "config", "configs/command-server.yaml", "path to command-server config file")
	flag.Parse()

	cfg, err := config.LoadCommandServer(cfgPath)
	if err != nil {
		log.Fatalf("[command-server] failed to load config: %v", err)
	}

	keyStore, err := server.InitAPIKeyStore(cfg.API.KeysFile)
	if err != nil {
		log.Fatalf("[command-server] failed to initialize api keys: %v", err)
	}

	auth := server.NewAuthMiddleware(keyStore.GetKeys(), cfg.API.AllowUnauthenticated)

	if cfg.API.AllowUnauthenticated {
		log.Printf("[command-server] WARNING: API authentication disabled (allow_unauthenticated=true)")
		log.Printf("[command-server] WARNING: All endpoints are open - development use only!")
	}

	reg := server.NewRegistry()

	tlsCfg, err := server.BuildMTLSServerConfig(
		cfg.TLS.CAFile,
		cfg.TLS.CertFile,
		cfg.TLS.KeyFile,
	)
	if err != nil {
		log.Fatalf("[command-server] failed to build mTLS config: %v", err)
	}

	agentListener := server.NewAgentListener(cfg.TCPAddr, reg, tlsCfg, cfg.TLS.AllowedAgents)

	go func() {
		log.Printf("[command-server] agent listener starting on %s", cfg.TCPAddr)
		if err := agentListener.ListenAndServe(); err != nil {
			log.Fatalf("[command-server] agent listener failed: %v", err)
		}
	}()

	// Pass auth middleware to HTTP server
	api := server.NewHTTPServer(cfg.HTTPAddr, reg, auth)

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
