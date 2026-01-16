package manager

import (
	"bufio"
	"context"
	"errors"
	"fmt"
	"os"
	"os/exec"
	"syscall"
	"time"
)

func (m *Manager) Start(cfg ServerConfig, logPath string) (ServerState, error) {
	m.mu.Lock()
	defer m.mu.Unlock()

	if p, ok := m.procs[cfg.Name]; ok && p.state.Running {
		return p.state, fmt.Errorf("%s already running (pid=%d)", cfg.Name, p.state.PID)
	}

	ctx, cancel := context.WithCancel(context.Background())

	cmd := exec.CommandContext(ctx, cfg.Command, cfg.Args...)
	cmd.Dir = cfg.Cwd
	cmd.Env = append(os.Environ(), cfg.Env...)

	// Put the process into its own process group (Unix)
	cmd.SysProcAttr = &syscall.SysProcAttr{Setpgid: true}

	// Logs
	logFile, err := os.OpenFile(logPath, os.O_CREATE|os.O_APPEND|os.O_WRONLY, 0644)
	if err != nil {
		cancel()
		return ServerState{}, err
	}
	cmd.Stdout = logFile
	cmd.Stderr = logFile

	// Optional stdin control
	stdinPipe, err := cmd.StdinPipe()
	if err != nil {
		cancel()
		_ = logFile.Close()
		return ServerState{}, err
	}

	if err := cmd.Start(); err != nil {
		cancel()
		_ = logFile.Close()
		return ServerState{}, err
	}

	p := &managedProc{
		cfg:    cfg,
		cmd:    cmd,
		state:  ServerState{Name: cfg.Name, Running: true, PID: cmd.Process.Pid, StartedAt: time.Now()},
		stdin:  bufio.NewWriter(stdinPipe),
		cancel: cancel,
	}

	m.procs[cfg.Name] = p

	// Reap process asynchronously
	go func() {
		err := cmd.Wait()
		exitCode := 0
		if err != nil {
			// best-effort exit code extraction
			if ee := new(exec.ExitError); errors.As(err, &ee) {
				if ws, ok := ee.Sys().(syscall.WaitStatus); ok {
					exitCode = ws.ExitStatus()
				} else {
					exitCode = 1
				}
			} else {
				exitCode = 1
			}
		}

		m.mu.Lock()
		defer m.mu.Unlock()

		p.state.Running = false
		p.state.ExitedAt = time.Now()
		p.state.ExitCode = exitCode
		if err != nil {
			p.state.LastError = err.Error()
		}
		_ = logFile.Close()
	}()

	return p.state, nil
}

func (m *Manager) Stop(name string) (ServerState, error) {
	m.mu.Lock()
	p, ok := m.procs[name]
	if !ok {
		m.mu.Unlock()
		return ServerState{}, fmt.Errorf("unknown server: %s", name)
	}
	if !p.state.Running || p.cmd.Process == nil {
		state := p.state
		m.mu.Unlock()
		return state, fmt.Errorf("%s is not running", name)
	}

	// Snapshot values we need without holding lock too long
	stopCfg := p.cfg.Stop
	pid := p.cmd.Process.Pid
	m.mu.Unlock()

	// Attempt graceful stop
	switch stopCfg.Type {
	case StopStdin:
		if stopCfg.StdinCommand == "" {
			stopCfg.StdinCommand = "stop\n"
		}
		_, _ = p.stdin.WriteString(stopCfg.StdinCommand)
		_ = p.stdin.Flush()

	case StopSignal:
		if stopCfg.Signal == 0 {
			stopCfg.Signal = syscall.SIGTERM
		}
		// kill process group: negative PID
		_ = syscall.Kill(-pid, stopCfg.Signal)

	default:
		_ = syscall.Kill(-pid, syscall.SIGTERM)
	}

	grace := stopCfg.GracePeriod
	if grace == 0 {
		grace = 10 * time.Second
	}

	// Wait for exit
	deadline := time.Now().Add(grace)
	for time.Now().Before(deadline) {
		if !m.IsRunning(name) {
			return m.Status(name), nil
		}
		time.Sleep(200 * time.Millisecond)
	}

	// Force kill
	_ = syscall.Kill(-pid, syscall.SIGKILL)

	// Give it a moment to settle
	time.Sleep(250 * time.Millisecond)

	return m.Status(name), nil
}

func (m *Manager) IsRunning(name string) bool {
	m.mu.Lock()
	defer m.mu.Unlock()
	p, ok := m.procs[name]
	return ok && p.state.Running
}

func (m *Manager) Status(name string) ServerState {
	m.mu.Lock()
	defer m.mu.Unlock()
	if p, ok := m.procs[name]; ok {
		return p.state
	}
	return ServerState{Name: name, Running: false}
}

func (m *Manager) List() []ServerState {
	m.mu.Lock()
	defer m.mu.Unlock()
	out := make([]ServerState, 0, len(m.procs))
	for _, p := range m.procs {
		out = append(out, p.state)
	}
	return out
}
