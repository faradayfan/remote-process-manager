package server

import (
	"crypto/tls"
	"encoding/json"
	"fmt"
	"log"
	"net"
	"strings"

	"github.com/faradayfan/remote-process-manager/internal/protocol"
	"github.com/faradayfan/remote-process-manager/internal/transport"
)

const agentURIPrefix = "spiffe://remote-process-manager/agent/"

type AgentListener struct {
	addr           string
	registry       *Registry
	tlsConfig      *tls.Config
	allowedAgents  map[string]struct{}
	allowAnySigned bool
}

func NewAgentListener(addr string, registry *Registry, tlsCfg *tls.Config, allowedAgents []string) *AgentListener {
	m := map[string]struct{}{}
	for _, a := range allowedAgents {
		a = strings.TrimSpace(a)
		if a == "" {
			continue
		}
		m[a] = struct{}{}
	}

	return &AgentListener{
		addr:           addr,
		registry:       registry,
		tlsConfig:      tlsCfg,
		allowedAgents:  m,
		allowAnySigned: len(m) == 0,
	}
}

func (l *AgentListener) ListenAndServe() error {
	if l.tlsConfig == nil {
		return fmt.Errorf("agent listener requires tlsConfig (mTLS)")
	}

	ln, err := tls.Listen("tcp", l.addr, l.tlsConfig)
	if err != nil {
		return fmt.Errorf("listen on %s: %w", l.addr, err)
	}
	log.Printf("[command-server] agent listener (mTLS) on %s", l.addr)

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
	defer func() { _ = c.Close() }()

	// Must be a TLS connection to inspect peer certs
	tlsConn, ok := c.(*tls.Conn)
	if !ok {
		log.Printf("[command-server] rejecting non-tls connection from %s", c.RemoteAddr())
		return
	}

	// Ensure handshake has completed so ConnectionState is populated
	if err := tlsConn.Handshake(); err != nil {
		log.Printf("[command-server] tls handshake failed from %s: %v", c.RemoteAddr(), err)
		return
	}

	agentID, err := l.agentIDFromPeerCert(tlsConn)
	if err != nil {
		log.Printf("[command-server] rejecting agent connection from %s: %v", c.RemoteAddr(), err)
		return
	}

	tc := transport.NewConn(tlsConn)

	// First message MUST be register
	first, err := tc.Recv()
	if err != nil {
		return
	}

	// We do NOT trust first.AgentID as authoritative anymore.
	if first.Kind != protocol.KindRegister {
		log.Printf("[command-server] rejecting agent=%s: first message not register kind=%s", agentID, first.Kind)
		return
	}

	// Optional safety: if agent still sets AgentID field, it must match cert-derived ID.
	if first.AgentID != "" && first.AgentID != agentID {
		log.Printf("[command-server] rejecting agent: cert_id=%s payload_id=%s mismatch", agentID, first.AgentID)
		return
	}

	var reg protocol.RegisterPayload
	if err := json.Unmarshal(first.Payload, &reg); err != nil {
		log.Printf("[command-server] rejecting agent=%s: bad register payload: %v", agentID, err)
		return
	}

	// Register agent using cert-derived identity
	l.registry.RegisterAgent(agentID, reg.Instances, tc)
	log.Printf("[command-server] agent registered: %s instances=%v", agentID, reg.Instances)

	// Main loop reads responses from agent
	for {
		msg, err := tc.Recv()
		if err != nil {
			log.Printf("[command-server] agent disconnected: %s", agentID)
			l.registry.RemoveAgent(agentID)
			return
		}

		_ = msg.ValidateBasic()

		// Allow register updates mid-connection
		if msg.Kind == protocol.KindRegister {
			// Optional safety: reject mismatched claimed agent IDs
			if msg.AgentID != "" && msg.AgentID != agentID {
				log.Printf("[command-server] ignoring register update: cert_id=%s payload_id=%s mismatch", agentID, msg.AgentID)
				continue
			}

			var reg protocol.RegisterPayload
			if err := json.Unmarshal(msg.Payload, &reg); err == nil {
				l.registry.UpdateAgentInstances(agentID, reg.Instances)
				log.Printf("[command-server] agent updated registration: %s instances=%v", agentID, reg.Instances)
			}
			continue
		}

		// Route incoming agent messages using cert-derived agentID
		// (If your registry uses msg.AgentID, you should ignore it or overwrite it here)
		msg.AgentID = agentID
		l.registry.HandleIncomingFromAgent(msg)
	}
}

// Extract agent identity from the client certificate URI SAN:
// spiffe://remote-process-manager/agent/<agent-id>
func (l *AgentListener) agentIDFromPeerCert(conn *tls.Conn) (string, error) {
	state := conn.ConnectionState()
	if len(state.PeerCertificates) == 0 {
		return "", fmt.Errorf("no peer certificates presented")
	}

	cert := state.PeerCertificates[0]

	// Find matching URI SAN
	var uriVal string
	for _, u := range cert.URIs {
		if u == nil {
			continue
		}
		s := u.String()
		if strings.HasPrefix(s, agentURIPrefix) {
			uriVal = s
			break
		}
	}

	if uriVal == "" {
		return "", fmt.Errorf("client cert missing URI SAN with prefix %q", agentURIPrefix)
	}

	agentID := strings.TrimPrefix(uriVal, agentURIPrefix)
	agentID = strings.TrimSpace(agentID)

	if err := validateAgentID(agentID); err != nil {
		return "", err
	}

	// Allowlist enforcement
	if !l.allowAnySigned {
		if _, ok := l.allowedAgents[agentID]; !ok {
			return "", fmt.Errorf("agent_id %q not in allowlist", agentID)
		}
	}

	return agentID, nil
}

func validateAgentID(id string) error {
	if id == "" {
		return fmt.Errorf("empty agent id")
	}
	if len(id) > 64 {
		return fmt.Errorf("agent id too long (max 64)")
	}

	// Only allow: a-z A-Z 0-9 . _ -
	for _, r := range id {
		switch {
		case r >= 'a' && r <= 'z':
		case r >= 'A' && r <= 'Z':
		case r >= '0' && r <= '9':
		case r == '.' || r == '_' || r == '-':
		default:
			return fmt.Errorf("invalid agent id %q: contains illegal character %q", id, r)
		}
	}
	return nil
}
