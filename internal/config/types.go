package config

// Config matches the shape of configs/servers.yaml
type Config struct {
	Servers map[string]Server `yaml:"servers"`
}

// Server is a YAML-friendly server definition
type Server struct {
	Command string   `yaml:"command"`
	Args    []string `yaml:"args"`
	Cwd     string   `yaml:"cwd"`
	Env     []string `yaml:"env"`
	Stop    Stop     `yaml:"stop"`
}

// Stop defines how to stop the server
type Stop struct {
	Type        string `yaml:"type"`         // "stdin" or "signal"
	Command     string `yaml:"command"`      // for stdin stop (e.g. "stop\n")
	Signal      string `yaml:"signal"`       // for signal stop (e.g. "SIGTERM")
	GracePeriod string `yaml:"grace_period"` // e.g. "15s"
}
