package main

import (
	"context"
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

const (
	agentConfigPath     = "configs/agent.yaml"
	templatesConfigPath = "configs/instance-templates.yaml"
	instancesConfigPath = "configs/instances.yaml"
)

func main() {
	log.SetFlags(log.LstdFlags | log.Lmicroseconds)

	// Root context that cancels on SIGINT/SIGTERM.
	ctx, cancel := signal.NotifyContext(context.Background(), os.Interrupt, syscall.SIGTERM)
	defer cancel()

	// Load agent settings (agent ID + server address)
	agentCfg, err := config.LoadAgent(agentConfigPath)
	if err != nil {
		log.Fatalf("[agent] failed to load agent config: %v", err)
	}

	templatesCfg, err := config.LoadTemplates(templatesConfigPath)
	if err != nil {
		log.Fatalf("[agent] failed to load templates config: %v", err)
	}

	store := instances.NewStore(instancesConfigPath)

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

	// Build a richer agent context for handlers.
	agentCtx := control.AgentContext{
		AgentID:             agentCfg.AgentID,
		AgentConfigPath:     agentConfigPath,
		TemplatesConfigPath: templatesConfigPath,
		InstancesConfigPath: instancesConfigPath,
		Instances:           instSvc,
	}

	handler := control.NewHandler(agentCtx)

	log.Printf("[agent] starting agent_id=%s command_server=%s", agentCfg.AgentID, agentCfg.CommandServerAddr)

	// Main connection loop with reconnect (runs until ctx is cancelled)
	go runAgentLoop(ctx, agentCfg.AgentID, agentCfg.CommandServerAddr, handler)

	<-ctx.Done()
	log.Printf("[agent] shutting down (note: any running game servers will continue unless you stop them via command)")
}

func runAgentLoop(ctx context.Context, agentID string, addr string, handler *control.Handler) {
	backoff := 1 * time.Second
	maxBackoff := 30 * time.Second

	for {
		// Exit cleanly if shutting down
		select {
		case <-ctx.Done():
			return
		default:
		}

		err := connectAndServe(ctx, agentID, addr, handler)
		if err != nil {
			log.Printf("[agent] connection ended: %v", err)
		}

		// Wait before reconnecting (but abort if shutting down)
		log.Printf("[agent] reconnecting in %s...", backoff)
		select {
		case <-time.After(backoff):
		case <-ctx.Done():
			return
		}

		backoff *= 2
		if backoff > maxBackoff {
			backoff = maxBackoff
		}
	}
}

func connectAndServe(ctx context.Context, agentID string, addr string, handler *control.Handler) error {
	dialer := net.Dialer{}
	c, err := dialer.DialContext(ctx, "tcp", addr)
	if err != nil {
		return err
	}
	defer c.Close()

	tc := transport.NewConn(c)

	// Send register message first
	regPayload := protocol.RegisterPayload{
		Instances: handler.SupportedInstances(),
	}

	regMsg, err := protocol.NewRegister(agentID, regPayload)
	if err != nil {
		return err
	}

	if err := tc.Send(regMsg); err != nil {
		return err
	}

	// If instance list changes, update the command server registry.
	sendRegister := func() {
		regPayload := protocol.RegisterPayload{
			Instances: handler.SupportedInstances(),
		}
		regMsg, _ := protocol.NewRegister(agentID, regPayload)
		_ = tc.Send(regMsg)
	}
	handler.OnInstanceListChanged = sendRegister

	log.Printf("[agent] registered with command-server addr=%s instances=%v", addr, regPayload.Instances)

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
			case <-ctx.Done():
				return
			}
		}
	}()
	defer close(heartbeatStop)

	// Main loop: receive commands, execute, respond
	for {
		// If shutting down, stop reading/handling commands
		select {
		case <-ctx.Done():
			return ctx.Err()
		default:
		}

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

		resp, err := handler.Handle(ctx, msg)
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
