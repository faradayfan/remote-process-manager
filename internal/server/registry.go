package server

import (
	"context"
	"crypto/rand"
	"encoding/hex"
	"fmt"
	"sync"
	"time"

	"github.com/faradayfan/remote-process-manager/internal/protocol"
	"github.com/faradayfan/remote-process-manager/internal/transport"
)

type AgentInfo struct {
	AgentID     string    `json:"agent_id"`
	Servers     []string  `json:"servers"`
	ConnectedAt time.Time `json:"connected_at"`
	LastSeen    time.Time `json:"last_seen"`
}

type agentConn struct {
	info AgentInfo
	conn *transport.Conn

	mu      sync.Mutex
	pending map[string]chan protocol.Message // request id -> response channel
}

type Registry struct {
	mu     sync.Mutex
	agents map[string]*agentConn
}

func NewRegistry() *Registry {
	return &Registry{
		agents: map[string]*agentConn{},
	}
}

func (r *Registry) ListAgents() []AgentInfo {
	r.mu.Lock()
	defer r.mu.Unlock()

	out := make([]AgentInfo, 0, len(r.agents))
	for _, a := range r.agents {
		out = append(out, a.info)
	}
	return out
}

func (r *Registry) GetAgent(agentID string) (AgentInfo, bool) {
	r.mu.Lock()
	defer r.mu.Unlock()
	a, ok := r.agents[agentID]
	if !ok {
		return AgentInfo{}, false
	}
	return a.info, true
}

func (r *Registry) RegisterAgent(agentID string, servers []string, c *transport.Conn) {
	r.mu.Lock()
	defer r.mu.Unlock()

	r.agents[agentID] = &agentConn{
		info: AgentInfo{
			AgentID:     agentID,
			Servers:     servers,
			ConnectedAt: time.Now().UTC(),
			LastSeen:    time.Now().UTC(),
		},
		conn:    c,
		pending: map[string]chan protocol.Message{},
	}
}

func (r *Registry) UpdateAgentServers(agentID string, servers []string) {
	r.mu.Lock()
	defer r.mu.Unlock()

	a, ok := r.agents[agentID]
	if !ok {
		return
	}
	a.info.Servers = servers
	a.info.LastSeen = time.Now().UTC()
}

func (r *Registry) RemoveAgent(agentID string) {
	r.mu.Lock()
	defer r.mu.Unlock()
	delete(r.agents, agentID)
}

func (r *Registry) Touch(agentID string) {
	r.mu.Lock()
	defer r.mu.Unlock()
	if a, ok := r.agents[agentID]; ok {
		a.info.LastSeen = time.Now().UTC()
	}
}

func (r *Registry) SendCommand(ctx context.Context, agentID string, typ string, payload any) (protocol.Message, error) {
	a := r.get(agentID)
	if a == nil {
		return protocol.Message{}, fmt.Errorf("agent not connected: %s", agentID)
	}

	reqID := newID()

	req, err := protocol.NewRequest(agentID, reqID, typ, payload)
	if err != nil {
		return protocol.Message{}, err
	}

	ch := make(chan protocol.Message, 1)

	a.mu.Lock()
	a.pending[reqID] = ch
	a.mu.Unlock()

	// Always cleanup pending
	defer func() {
		a.mu.Lock()
		delete(a.pending, reqID)
		a.mu.Unlock()
	}()

	// Send
	if err := a.conn.Send(req); err != nil {
		return protocol.Message{}, err
	}

	select {
	case <-ctx.Done():
		return protocol.Message{}, fmt.Errorf("timeout waiting for agent response")
	case resp := <-ch:
		return resp, nil
	}
}

func (r *Registry) HandleIncomingFromAgent(msg protocol.Message) {
	// Dispatch response -> pending channel
	r.Touch(msg.AgentID)

	a := r.get(msg.AgentID)
	if a == nil {
		return
	}

	if msg.Kind != protocol.KindResponse {
		return
	}

	a.mu.Lock()
	ch, ok := a.pending[msg.ID]
	a.mu.Unlock()

	if ok {
		ch <- msg
	}
}

func (r *Registry) get(agentID string) *agentConn {
	r.mu.Lock()
	defer r.mu.Unlock()
	return r.agents[agentID]
}

func newID() string {
	var b [16]byte
	_, _ = rand.Read(b[:])
	return hex.EncodeToString(b[:])
}
