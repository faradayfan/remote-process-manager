package server

import (
	"encoding/json"
	"fmt"
	"log"
	"net"

	"github.com/faradayfan/remote-process-manager/internal/protocol"
	"github.com/faradayfan/remote-process-manager/internal/transport"
)

type AgentListener struct {
	addr     string
	registry *Registry
}

func NewAgentListener(addr string, registry *Registry) *AgentListener {
	return &AgentListener{
		addr:     addr,
		registry: registry,
	}
}

func (l *AgentListener) ListenAndServe() error {
	ln, err := net.Listen("tcp", l.addr)
	if err != nil {
		return fmt.Errorf("listen on %s: %w", l.addr, err)
	}
	log.Printf("[command-server] agent listener on %s", l.addr)

	for {
		c, err := ln.Accept()
		if err != nil {
			log.Printf("[command-server] accept error: %v", err)
			continue
		}

		go l.handleConn(c)
	}
}

func (l *AgentListener) handleConn(c net.Conn) {
	tc := transport.NewConn(c)

	// First message MUST be register
	first, err := tc.Recv()
	if err != nil {
		_ = tc.Close()
		return
	}

	if first.Kind != protocol.KindRegister || first.AgentID == "" {
		_ = tc.Close()
		return
	}

	var reg protocol.RegisterPayload
	if err := json.Unmarshal(first.Payload, &reg); err != nil {
		_ = tc.Close()
		return
	}

	l.registry.RegisterAgent(first.AgentID, reg.Servers, tc)
	log.Printf("[command-server] agent registered: %s servers=%v", first.AgentID, reg.Servers)

	// Main loop reads responses from agent
	for {
		msg, err := tc.Recv()
		if err != nil {
			log.Printf("[command-server] agent disconnected: %s", first.AgentID)
			l.registry.RemoveAgent(first.AgentID)
			_ = tc.Close()
			return
		}

		_ = msg.ValidateBasic()

		// Allow register updates mid-connection
		if msg.Kind == protocol.KindRegister && msg.AgentID != "" {
			var reg protocol.RegisterPayload
			if err := json.Unmarshal(msg.Payload, &reg); err == nil {
				l.registry.UpdateAgentServers(msg.AgentID, reg.Servers)
				log.Printf("[command-server] agent updated registration: %s servers=%v", msg.AgentID, reg.Servers)
			}
			continue
		}

		l.registry.HandleIncomingFromAgent(msg)
	}
}
