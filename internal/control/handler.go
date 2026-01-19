package control

import (
	"context"
	"encoding/json"
	"fmt"
	"sort"

	"github.com/faradayfan/remote-process-manager/internal/config"
	"github.com/faradayfan/remote-process-manager/internal/instances"
	"github.com/faradayfan/remote-process-manager/internal/protocol"
)

type AgentContext struct {
	AgentID string

	// Useful for future endpoints (templates inspect, config introspection, etc.)
	AgentConfigPath     string
	TemplatesConfigPath string
	InstancesConfigPath string

	Instances *instances.Service
}

type Handler struct {
	Ctx AgentContext

	// Callback so the agent can update the command server’s registry after changes
	OnInstanceListChanged func()
}

func NewHandler(ctx AgentContext) *Handler {
	return &Handler{Ctx: ctx}
}

func (h *Handler) SupportedServers() []string {
	// Protocol field is still called "servers", but values are instance names.
	return h.Ctx.Instances.ListInstanceNames()
}

func (h *Handler) Handle(ctx context.Context, msg protocol.Message) (protocol.Message, error) {
	// Only return a Go error for truly invalid transport/protocol cases.
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
		summaries := h.Ctx.Instances.ListInstanceSummaries()
		return h.replyOK(msg, map[string]any{"instances": summaries}), nil

	case protocol.CmdInstancesCreate:
		req, err := decode[protocol.CreateInstanceRequest](msg)
		if err != nil {
			return h.replyErr(msg, fmt.Errorf("bad payload: %w", err)), nil
		}

		if err := h.Ctx.Instances.CreateInstance(req.Name, req.Template, req.Enabled, req.Params); err != nil {
			return h.replyErr(msg, err), nil
		}

		h.notifyInstanceListChanged()

		return h.replyOK(msg, map[string]any{
			"ok":   true,
			"name": req.Name,
		}), nil

	case protocol.CmdInstancesDelete:
		req, err := decode[protocol.DeleteInstanceRequest](msg)
		if err != nil {
			return h.replyErr(msg, fmt.Errorf("bad payload: %w", err)), nil
		}

		if err := h.Ctx.Instances.DeleteInstance(req.Name, req.Force, req.DeleteData); err != nil {
			return h.replyErr(msg, err), nil
		}

		h.notifyInstanceListChanged()

		return h.replyOK(msg, map[string]any{
			"ok":   true,
			"name": req.Name,
		}), nil

	// --------------------
	// Process operations on an instance name
	// --------------------
	case protocol.CmdList:
		// Status for all known instances
		states := make([]any, 0)
		for _, name := range h.Ctx.Instances.ListInstanceNames() {
			states = append(states, h.status(name))
		}
		return h.replyOK(msg, states), nil

	case protocol.CmdStatus:
		tgt, err := decode[protocol.ServerTarget](msg)
		if err != nil {
			return h.replyErr(msg, fmt.Errorf("bad payload: %w", err)), nil
		}

		return h.replyOK(msg, h.status(tgt.Server)), nil

	case protocol.CmdStart:
		tgt, err := decode[protocol.ServerTarget](msg)
		if err != nil {
			return h.replyErr(msg, fmt.Errorf("bad payload: %w", err)), nil
		}

		st, err := h.start(ctx, tgt.Server)
		return h.reply(msg, st, err), nil

	case protocol.CmdStop:
		tgt, err := decode[protocol.ServerTarget](msg)
		if err != nil {
			return h.replyErr(msg, fmt.Errorf("bad payload: %w", err)), nil
		}

		st, err := h.stop(ctx, tgt.Server)
		return h.reply(msg, st, err), nil

	// --------------------
	// Templates
	// --------------------
	case protocol.CmdTemplatesList:
		cfg, err := config.LoadTemplates(h.Ctx.TemplatesConfigPath)
		if err != nil {
			return h.replyErr(msg, err), nil
		}

		names := make([]string, 0, len(cfg.Templates))
		for name := range cfg.Templates {
			names = append(names, name)
		}
		sort.Strings(names)

		return h.replyOK(msg, map[string]any{
			"templates": names,
		}), nil

	case protocol.CmdTemplatesInspect:
		req, err := decode[protocol.TemplatesInspectRequest](msg)
		if err != nil {
			return h.replyErr(msg, fmt.Errorf("bad payload: %w", err)), nil
		}
		if req.Name == "" {
			return h.replyErr(msg, fmt.Errorf("name is required")), nil
		}

		cfg, err := config.LoadTemplates(h.Ctx.TemplatesConfigPath)
		if err != nil {
			return h.replyErr(msg, err), nil
		}

		tpl, ok := cfg.Templates[req.Name]
		if !ok {
			return h.replyErr(msg, fmt.Errorf("template %q not found", req.Name)), nil
		}
		return h.replyOK(msg, map[string]any{
			"name":     req.Name,
			"template": tpl,
		}), nil

	default:
		return h.replyErr(msg, fmt.Errorf("unknown command type: %s", msg.Type)), nil
	}
}

//
// internal helpers
//

func (h *Handler) notifyInstanceListChanged() {
	if h.OnInstanceListChanged != nil {
		h.OnInstanceListChanged()
	}
}

func (h *Handler) replyOK(req protocol.Message, payload any) protocol.Message {
	resp, _ := protocol.NewResponse(h.Ctx.AgentID, req.ID, payload, nil)
	return resp
}

func (h *Handler) replyErr(req protocol.Message, err error) protocol.Message {
	resp, _ := protocol.NewResponse(h.Ctx.AgentID, req.ID, nil, err)
	return resp
}

func (h *Handler) reply(req protocol.Message, payload any, err error) protocol.Message {
	resp, _ := protocol.NewResponse(h.Ctx.AgentID, req.ID, payload, err)
	return resp
}

func decode[T any](msg protocol.Message) (T, error) {
	var out T
	if err := json.Unmarshal(msg.Payload, &out); err != nil {
		return out, err
	}
	return out, nil
}

//
// manager wrappers
//

func (h *Handler) status(instance string) any {
	return h.Ctx.Instances.Mgr.Status(instance)
}

func (h *Handler) start(ctx context.Context, instance string) (any, error) {
	cfg, logPath, err := h.Ctx.Instances.ResolveConfig(instance)
	if err != nil {
		return nil, err
	}

	// NOTE: Your manager.Start() signature currently doesn't accept ctx.
	// For now, we ignore ctx here. Next refactor would thread ctx into manager methods.
	st, err := h.Ctx.Instances.Mgr.Start(cfg, logPath)
	return st, err
}

func (h *Handler) stop(ctx context.Context, instance string) (any, error) {
	// NOTE: Same as start() regarding ctx
	st, err := h.Ctx.Instances.Mgr.Stop(instance)
	return st, err
}
