package protocol

const (
	// Instance management commands (agent-side)
	CmdInstancesList   = "instances.list"
	CmdInstancesCreate = "instances.create"
	CmdInstancesDelete = "instances.delete"
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
