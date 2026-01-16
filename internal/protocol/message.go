package protocol

import (
	"encoding/json"
	"fmt"
	"time"
)

type Kind string

const (
	KindRegister  Kind = "register"
	KindRequest   Kind = "request"
	KindResponse  Kind = "response"
	KindHeartbeat Kind = "heartbeat"
)

type Message struct {
	Kind    Kind            `json:"kind"`
	ID      string          `json:"id,omitempty"`       // request/response correlation
	AgentID string          `json:"agent_id,omitempty"` // who this is about
	Type    string          `json:"type,omitempty"`     // request type
	Payload json.RawMessage `json:"payload,omitempty"`  // request/response payload
	Error   string          `json:"error,omitempty"`    // response error (if any)
	TS      time.Time       `json:"ts,omitempty"`
}

func NewRegister(agentID string, payload any) (Message, error) {
	b, err := json.Marshal(payload)
	if err != nil {
		return Message{}, err
	}
	return Message{
		Kind:    KindRegister,
		AgentID: agentID,
		Payload: b,
		TS:      time.Now().UTC(),
	}, nil
}

func NewRequest(agentID, id, typ string, payload any) (Message, error) {
	b, err := json.Marshal(payload)
	if err != nil {
		return Message{}, err
	}
	return Message{
		Kind:    KindRequest,
		ID:      id,
		AgentID: agentID,
		Type:    typ,
		Payload: b,
		TS:      time.Now().UTC(),
	}, nil
}

func NewResponse(agentID, id string, payload any, respErr error) (Message, error) {
	var b []byte
	var err error
	if payload != nil {
		b, err = json.Marshal(payload)
		if err != nil {
			return Message{}, err
		}
	}
	msg := Message{
		Kind:    KindResponse,
		ID:      id,
		AgentID: agentID,
		Payload: b,
		TS:      time.Now().UTC(),
	}
	if respErr != nil {
		msg.Error = respErr.Error()
	}
	return msg, nil
}

func (m Message) ValidateBasic() error {
	if m.Kind == "" {
		return fmt.Errorf("missing kind")
	}
	switch m.Kind {
	case KindRegister:
		if m.AgentID == "" {
			return fmt.Errorf("register missing agent_id")
		}
	case KindRequest:
		if m.ID == "" {
			return fmt.Errorf("request missing id")
		}
		if m.AgentID == "" {
			return fmt.Errorf("request missing agent_id")
		}
		if m.Type == "" {
			return fmt.Errorf("request missing type")
		}
	case KindResponse:
		if m.ID == "" {
			return fmt.Errorf("response missing id")
		}
		if m.AgentID == "" {
			return fmt.Errorf("response missing agent_id")
		}
	case KindHeartbeat:
		if m.AgentID == "" {
			return fmt.Errorf("heartbeat missing agent_id")
		}
	default:
		return fmt.Errorf("unknown kind: %s", m.Kind)
	}
	return nil
}
