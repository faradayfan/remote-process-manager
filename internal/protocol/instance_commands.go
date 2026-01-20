package protocol

const (
	CmdInstancesList        = "instances.list"
	CmdInstancesCreate      = "instances.create"
	CmdInstancesDelete      = "instances.delete"
	CmdInstancesEnable      = "instances.enable"
	CmdInstancesDisable     = "instances.disable"
	CmdInstancesParamsSet   = "instances.params.set"
	CmdInstancesParamsUnset = "instances.params.unset"
)

type InstanceSummary struct {
	Name     string            `json:"name"`
	Template string            `json:"template"`
	Enabled  bool              `json:"enabled"`
	Params   map[string]string `json:"params,omitempty"`
	Running  bool              `json:"running"`
	PID      int               `json:"pid,omitempty"`
}

type CreateInstanceRequest struct {
	Name     string            `json:"name"`
	Template string            `json:"template"`
	Enabled  bool              `json:"enabled"`
	Params   map[string]string `json:"params,omitempty"`
}

type DeleteInstanceRequest struct {
	Name       string `json:"name"`
	Force      bool   `json:"force"`       // stop if running
	DeleteData bool   `json:"delete_data"` // remove instance directory
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
