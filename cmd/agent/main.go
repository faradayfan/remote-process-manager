package main

import (
	"log"
	"net"
	"os"
	"os/signal"
	"syscall"
	"time"

	"github.com/faradayfan/remote-process-manager/internal/config"
	"github.com/faradayfan/remote-process-manager/internal/control"
	"github.com/faradayfan/remote-process-manager/internal/instances"
	"github.com/faradayfan/remote-process-manager/internal/manager"
	"github.com/faradayfan/remote-process-manager/internal/protocol"
	"github.com/faradayfan/remote-process-manager/internal/transport"
)

func main() {
	log.SetFlags(log.LstdFlags | log.Lmicroseconds)

	// Load agent settings (agent ID + server address)
	agentCfg, err := config.LoadAgent("configs/agent.yaml")
	if err != nil {
		log.Fatalf("[agent] failed to load agent config: %v", err)
	}
	templatesCfg, err := config.LoadTemplates("configs/server-templates.yaml")
	if err != nil {
		log.Fatalf("[agent] failed to load templates config: %v", err)
	}

	store := instances.NewStore("configs/instances.yaml")

	loadedInstances, err := store.Load()
	if err != nil {
		log.Fatalf("[agent] failed to load instances: %v", err)
	}

	mgr := manager.NewManager()

	instSvc := instances.NewService(
		mgr,
		templatesCfg.Templates,
		loadedInstances,
		store,
		"data/instances",
		"logs",
	)

	handler := control.NewHandler(agentCfg.AgentID, instSvc)

	log.Printf("[agent] starting agent_id=%s command_server=%s", agentCfg.AgentID, agentCfg.CommandServerAddr)

	// Graceful shutdown support
	stopCh := make(chan os.Signal, 1)
	signal.Notify(stopCh, os.Interrupt, syscall.SIGTERM)

	// Main connection loop with reconnect
	go func() {
		runAgentLoop(agentCfg.AgentID, agentCfg.CommandServerAddr, handler)
	}()

	<-stopCh
	log.Printf("[agent] shutting down (note: any running game servers will continue unless you stop them via command)")
}

func runAgentLoop(agentID string, addr string, handler *control.Handler) {
	backoff := 1 * time.Second
	maxBackoff := 30 * time.Second

	for {
		err := connectAndServe(agentID, addr, handler)
		if err != nil {
			log.Printf("[agent] connection ended: %v", err)
		}

		log.Printf("[agent] reconnecting in %s...", backoff)
		time.Sleep(backoff)

		backoff *= 2
		if backoff > maxBackoff {
			backoff = maxBackoff
		}
	}
}

func connectAndServe(agentID string, addr string, handler *control.Handler) error {
	c, err := net.Dial("tcp", addr)
	if err != nil {
		return err
	}
	defer c.Close()

	tc := transport.NewConn(c)

	// Send register message first
	regPayload := protocol.RegisterPayload{
		Servers: handler.SupportedServers(),
	}

	regMsg, err := protocol.NewRegister(agentID, regPayload)
	if err != nil {
		return err
	}

	if err := tc.Send(regMsg); err != nil {
		return err
	}

	sendRegister := func() {
		regPayload := protocol.RegisterPayload{
			Servers: handler.SupportedServers(),
		}
		regMsg, _ := protocol.NewRegister(agentID, regPayload)
		_ = tc.Send(regMsg)
	}
	handler.OnInstanceListChanged = sendRegister

	log.Printf("[agent] registered with command-server addr=%s servers=%v", addr, regPayload.Servers)

	// Heartbeats keep the registry "fresh"
	heartbeatStop := make(chan struct{})
	go func() {
		t := time.NewTicker(30 * time.Second)
		defer t.Stop()
		for {
			select {
			case <-t.C:
				_ = tc.Send(protocol.Message{
					Kind:    protocol.KindHeartbeat,
					AgentID: agentID,
					TS:      time.Now().UTC(),
				})
			case <-heartbeatStop:
				return
			}
		}
	}()
	defer close(heartbeatStop)

	// Main loop: receive commands, execute, respond
	for {
		msg, err := tc.Recv()
		if err != nil {
			return err
		}

		if err := msg.ValidateBasic(); err != nil {
			log.Printf("[agent] invalid message: %v", err)
			continue
		}

		// Only act on requests targeting this agent
		if msg.Kind != protocol.KindRequest {
			continue
		}
		if msg.AgentID != agentID {
			log.Printf("[agent] ignoring request for agent_id=%s (this=%s)", msg.AgentID, agentID)
			continue
		}

		resp, err := handler.Handle(msg)
		if err != nil {
			// If handler couldn't even produce a response, we can still try to return one
			fallback, _ := protocol.NewResponse(agentID, msg.ID, nil, err)
			_ = tc.Send(fallback)
			continue
		}

		if err := tc.Send(resp); err != nil {
			return err
		}
	}
}
