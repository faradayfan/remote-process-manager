package protocol

// Request types (m.Type)
const (
	CmdStart            = "start"
	CmdStop             = "stop"
	CmdStatus           = "status"
	CmdList             = "list"
	CmdInstancesEnable  = "instances.enable"
	CmdInstancesDisable = "instances.disable"
)

type RegisterPayload struct {
	Instances []string `json:"instances"`
}

type InstanceTarget struct {
	Instance string `json:"instance"`
}

type ToggleInstanceEnabledRequest struct {
	Name    string `json:"name"`
	Enabled bool   `json:"enabled"`
}
