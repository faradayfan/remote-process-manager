package protocol

// Request types (m.Type)
const (
	CmdStart  = "start"
	CmdStop   = "stop"
	CmdStatus = "status"
	CmdList   = "list"
)

type RegisterPayload struct {
	Instances []string `json:"instances"`
}

type InstanceTarget struct {
	Instance string `json:"instance"`
}
