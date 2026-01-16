package manager

import (
	"bufio"
	"context"
	"os/exec"
	"sync"
	"syscall"
	"time"
)

type StopType string

const (
	StopSignal StopType = "signal"
	StopStdin  StopType = "stdin"
)

type StopConfig struct {
	Type         StopType
	Signal       syscall.Signal // used if Type=signal
	StdinCommand string         // used if Type=stdin
	GracePeriod  time.Duration  // how long before SIGKILL
}

type ServerConfig struct {
	Name    string
	Command string
	Args    []string
	Cwd     string
	Env     []string
	Stop    StopConfig
}

type ServerState struct {
	Name      string
	Running   bool
	PID       int
	StartedAt time.Time
	ExitedAt  time.Time
	ExitCode  int
	LastError string
}

type managedProc struct {
	cfg   ServerConfig
	cmd   *exec.Cmd
	state ServerState

	stdin  *bufio.Writer
	cancel context.CancelFunc
}

type Manager struct {
	mu    sync.Mutex
	procs map[string]*managedProc
}

func NewManager() *Manager {
	return &Manager{
		procs: map[string]*managedProc{},
	}
}
