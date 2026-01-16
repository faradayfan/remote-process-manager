package protocol

// Request types (m.Type)
const (
	CmdStart  = "start"
	CmdStop   = "stop"
	CmdStatus = "status"
	CmdList   = "list"
)

type RegisterPayload struct {
	Servers []string `json:"servers"`
}

type ServerTarget struct {
	Server string `json:"server"`
}
