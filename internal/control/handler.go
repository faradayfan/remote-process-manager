package control

import (
	"encoding/json"
	"fmt"

	"github.com/faradayfan/remote-process-manager/internal/instances"
	"github.com/faradayfan/remote-process-manager/internal/protocol"
)

type Handler struct {
	AgentID   string
	Instances *instances.Service

	// Callback so the agent can update the command server’s registry after changes
	OnInstanceListChanged func()
}

func NewHandler(agentID string, inst *instances.Service) *Handler {
	return &Handler{
		AgentID:   agentID,
		Instances: inst,
	}
}

func (h *Handler) SupportedServers() []string {
	// Protocol field is still called "servers", but values are instance names.
	return h.Instances.ListInstanceNames()
}

func (h *Handler) Handle(msg protocol.Message) (protocol.Message, error) {
	if err := msg.ValidateBasic(); err != nil {
		return protocol.Message{}, err
	}
	if msg.Kind != protocol.KindRequest {
		return protocol.Message{}, fmt.Errorf("handler only accepts request messages")
	}

	switch msg.Type {

	// --------------------
	// Instance management
	// --------------------
	case protocol.CmdInstancesList:
		summaries := h.Instances.ListInstanceSummaries()
		return protocol.NewResponse(h.AgentID, msg.ID, map[string]any{
			"instances": summaries,
		}, nil)

	case protocol.CmdInstancesCreate:
		var req protocol.CreateInstanceRequest
		if err := json.Unmarshal(msg.Payload, &req); err != nil {
			resp, _ := protocol.NewResponse(h.AgentID, msg.ID, nil, fmt.Errorf("bad payload: %w", err))
			return resp, nil
		}

		if err := h.Instances.CreateInstance(req.Name, req.Template, req.Enabled, req.Params); err != nil {
			resp, _ := protocol.NewResponse(h.AgentID, msg.ID, nil, err)
			return resp, nil
		}

		if h.OnInstanceListChanged != nil {
			h.OnInstanceListChanged()
		}

		return protocol.NewResponse(h.AgentID, msg.ID, map[string]any{
			"ok":   true,
			"name": req.Name,
		}, nil)

	case protocol.CmdInstancesDelete:
		var req protocol.DeleteInstanceRequest
		if err := json.Unmarshal(msg.Payload, &req); err != nil {
			resp, _ := protocol.NewResponse(h.AgentID, msg.ID, nil, fmt.Errorf("bad payload: %w", err))
			return resp, nil
		}

		if err := h.Instances.DeleteInstance(req.Name, req.Force, req.DeleteData); err != nil {
			resp, _ := protocol.NewResponse(h.AgentID, msg.ID, nil, err)
			return resp, nil
		}

		if h.OnInstanceListChanged != nil {
			h.OnInstanceListChanged()
		}

		return protocol.NewResponse(h.AgentID, msg.ID, map[string]any{
			"ok":   true,
			"name": req.Name,
		}, nil)

	// --------------------
	// Process operations on an instance name
	// --------------------
	case protocol.CmdList:
		// Status for all known instances
		states := make([]any, 0)
		for _, name := range h.Instances.ListInstanceNames() {
			st := h.Instances.Mgr.Status(name)
			states = append(states, st)
		}
		return protocol.NewResponse(h.AgentID, msg.ID, states, nil)

	case protocol.CmdStatus:
		var tgt protocol.ServerTarget
		if err := json.Unmarshal(msg.Payload, &tgt); err != nil {
			resp, _ := protocol.NewResponse(h.AgentID, msg.ID, nil, fmt.Errorf("bad payload: %w", err))
			return resp, nil
		}
		st := h.Instances.Mgr.Status(tgt.Server)
		return protocol.NewResponse(h.AgentID, msg.ID, st, nil)

	case protocol.CmdStart:
		var tgt protocol.ServerTarget
		if err := json.Unmarshal(msg.Payload, &tgt); err != nil {
			resp, _ := protocol.NewResponse(h.AgentID, msg.ID, nil, fmt.Errorf("bad payload: %w", err))
			return resp, nil
		}

		cfg, logPath, err := h.Instances.ResolveConfig(tgt.Server)
		if err != nil {
			resp, _ := protocol.NewResponse(h.AgentID, msg.ID, nil, err)
			return resp, nil
		}

		st, startErr := h.Instances.Mgr.Start(cfg, logPath)
		return protocol.NewResponse(h.AgentID, msg.ID, st, startErr)

	case protocol.CmdStop:
		var tgt protocol.ServerTarget
		if err := json.Unmarshal(msg.Payload, &tgt); err != nil {
			resp, _ := protocol.NewResponse(h.AgentID, msg.ID, nil, fmt.Errorf("bad payload: %w", err))
			return resp, nil
		}

		st, stopErr := h.Instances.Mgr.Stop(tgt.Server)
		return protocol.NewResponse(h.AgentID, msg.ID, st, stopErr)

	default:
		resp, _ := protocol.NewResponse(h.AgentID, msg.ID, nil, fmt.Errorf("unknown command type: %s", msg.Type))
		return resp, nil
	}
}
