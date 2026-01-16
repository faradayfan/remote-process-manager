package config

import (
	"fmt"
	"os"
	"strings"
	"syscall"
	"time"

	"gopkg.in/yaml.v3"

	"github.com/faradayfan/remote-process-manager/internal/manager"
)

func Load(path string) (*Config, error) {
	b, err := os.ReadFile(path)
	if err != nil {
		return nil, fmt.Errorf("read config file %q: %w", path, err)
	}

	var cfg Config
	if err := yaml.Unmarshal(b, &cfg); err != nil {
		return nil, fmt.Errorf("parse yaml %q: %w", path, err)
	}

	if len(cfg.Servers) == 0 {
		return nil, fmt.Errorf("config contains no servers")
	}

	return &cfg, nil
}

// ToManagerConfigs converts YAML config -> manager.ServerConfig
func (c *Config) ToManagerConfigs() (map[string]manager.ServerConfig, error) {
	out := make(map[string]manager.ServerConfig, len(c.Servers))

	for name, s := range c.Servers {
		if strings.TrimSpace(name) == "" {
			return nil, fmt.Errorf("server name cannot be empty")
		}
		if strings.TrimSpace(s.Command) == "" {
			return nil, fmt.Errorf("server %q missing command", name)
		}

		stopCfg, err := convertStop(name, s.Stop)
		if err != nil {
			return nil, err
		}

		out[name] = manager.ServerConfig{
			Name:    name,
			Command: s.Command,
			Args:    s.Args,
			Cwd:     s.Cwd,
			Env:     s.Env,
			Stop:    stopCfg,
		}
	}

	return out, nil
}

func convertStop(serverName string, s Stop) (manager.StopConfig, error) {
	// defaults
	cfg := manager.StopConfig{
		Type:        manager.StopSignal,
		Signal:      syscall.SIGTERM,
		GracePeriod: 10 * time.Second,
	}

	if strings.TrimSpace(s.Type) != "" {
		switch strings.ToLower(strings.TrimSpace(s.Type)) {
		case "stdin":
			cfg.Type = manager.StopStdin
		case "signal":
			cfg.Type = manager.StopSignal
		default:
			return manager.StopConfig{}, fmt.Errorf("server %q has invalid stop.type %q (expected stdin|signal)", serverName, s.Type)
		}
	}

	// Grace period
	if strings.TrimSpace(s.GracePeriod) != "" {
		d, err := time.ParseDuration(strings.TrimSpace(s.GracePeriod))
		if err != nil {
			return manager.StopConfig{}, fmt.Errorf("server %q has invalid stop.grace_period %q: %w", serverName, s.GracePeriod, err)
		}
		cfg.GracePeriod = d
	}

	// stdin command
	if cfg.Type == manager.StopStdin {
		cmd := s.Command
		if strings.TrimSpace(cmd) == "" {
			cmd = "stop\n" // common default for Minecraft
		}
		cfg.StdinCommand = cmd
		return cfg, nil
	}

	// signal
	if cfg.Type == manager.StopSignal && strings.TrimSpace(s.Signal) != "" {
		sig, err := parseSignal(strings.TrimSpace(s.Signal))
		if err != nil {
			return manager.StopConfig{}, fmt.Errorf("server %q has invalid stop.signal %q: %w", serverName, s.Signal, err)
		}
		cfg.Signal = sig
	}

	return cfg, nil
}

// func parseSignal(s string) (syscall.Signal, error) {
// 	// Accept "TERM", "SIGTERM", etc.
// 	u := strings.ToUpper(strings.TrimSpace(s))
// 	if !strings.HasPrefix(u, "SIG") {
// 		u = "SIG" + u
// 	}

// 	switch u {
// 	case "SIGTERM":
// 		return syscall.SIGTERM, nil
// 	case "SIGINT":
// 		return syscall.SIGINT, nil
// 	case "SIGKILL":
// 		return syscall.SIGKILL, nil
// 	case "SIGHUP":
// 		return syscall.SIGHUP, nil
// 	case "SIGQUIT":
// 		return syscall.SIGQUIT, nil
// 	default:
// 		return 0, fmt.Errorf("unsupported signal %q (try SIGTERM, SIGINT, SIGKILL, SIGHUP, SIGQUIT)", s)
// 	}
// }
