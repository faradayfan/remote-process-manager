package config

import (
	"fmt"
	"strings"
	"syscall"
	"time"

	"github.com/faradayfan/remote-process-manager/internal/manager"
)

func ConvertStopPublic(serverName string, s Stop) (manager.StopConfig, error) {
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
			cmd = "stop\n"
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

func parseSignal(s string) (syscall.Signal, error) {
	u := strings.ToUpper(strings.TrimSpace(s))
	if !strings.HasPrefix(u, "SIG") {
		u = "SIG" + u
	}

	switch u {
	case "SIGTERM":
		return syscall.SIGTERM, nil
	case "SIGINT":
		return syscall.SIGINT, nil
	case "SIGKILL":
		return syscall.SIGKILL, nil
	case "SIGHUP":
		return syscall.SIGHUP, nil
	case "SIGQUIT":
		return syscall.SIGQUIT, nil
	default:
		return 0, fmt.Errorf("unsupported signal %q (try SIGTERM, SIGINT, SIGKILL, SIGHUP, SIGQUIT)", s)
	}
}
