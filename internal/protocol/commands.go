package protocol

// Request types (m.Type)
const (
	CmdStart                = "start"
	CmdStop                 = "stop"
	CmdStatus               = "status"
	CmdList                 = "list"
	CmdInstancesEnable      = "instances.enable"
	CmdInstancesDisable     = "instances.disable"
	CmdInstancesParamsSet   = "instances.params.set"
	CmdInstancesParamsUnset = "instances.params.unset"
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

type InstancesParamsSetRequest struct {
	Name string            `json:"name"`
	Set  map[string]string `json:"set"`
}

type InstancesParamsUnsetRequest struct {
	Name  string   `json:"name"`
	Unset []string `json:"unset"`
}
