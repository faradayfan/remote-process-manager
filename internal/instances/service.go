package instances

import (
	"bytes"
	"fmt"
	"os"
	"path/filepath"
	"sync"
	"text/template"

	"github.com/faradayfan/remote-process-manager/internal/config"
	"github.com/faradayfan/remote-process-manager/internal/manager"
)

type Service struct {
	mu sync.Mutex

	Mgr       *manager.Manager
	Templates map[string]config.Template
	Instances map[string]config.Instance

	Store *Store

	BaseInstanceDir string
	LogDir          string
}

func NewService(
	mgr *manager.Manager,
	templates map[string]config.Template,
	instances map[string]config.Instance,
	store *Store,
	baseInstanceDir string,
	logDir string,
) *Service {
	if baseInstanceDir == "" {
		baseInstanceDir = "data/instances"
	}
	if logDir == "" {
		logDir = "logs"
	}

	return &Service{
		Mgr:             mgr,
		Templates:       templates,
		Instances:       instances,
		Store:           store,
		BaseInstanceDir: baseInstanceDir,
		LogDir:          logDir,
	}
}

func (s *Service) ListInstanceNames() []string {
	s.mu.Lock()
	defer s.mu.Unlock()

	out := make([]string, 0, len(s.Instances))
	for name := range s.Instances {
		out = append(out, name)
	}
	return out
}

func (s *Service) ListInstanceSummaries() []map[string]any {
	s.mu.Lock()
	defer s.mu.Unlock()

	out := make([]map[string]any, 0, len(s.Instances))
	for name, inst := range s.Instances {
		st := s.Mgr.Status(name)
		out = append(out, map[string]any{
			"name":     name,
			"template": inst.Template,
			"enabled":  inst.Enabled,
			"params":   inst.Params,
			"running":  st.Running,
			"pid":      st.PID,
		})
	}
	return out
}

func (s *Service) InstanceDir(name string) string {
	return filepath.Join(s.BaseInstanceDir, name)
}

func (s *Service) LogPath(name string) string {
	return filepath.Join(s.LogDir, fmt.Sprintf("%s.log", name))
}

func (s *Service) EnsureDirs(name string) error {
	if err := os.MkdirAll(s.LogDir, 0755); err != nil {
		return err
	}
	if err := os.MkdirAll(s.InstanceDir(name), 0755); err != nil {
		return err
	}
	return nil
}

// CreateInstance adds an instance to memory and persists it to instances.yaml.
func (s *Service) CreateInstance(name string, templateName string, enabled bool, params map[string]string) error {
	s.mu.Lock()
	defer s.mu.Unlock()

	if name == "" {
		return fmt.Errorf("instance name is required")
	}
	if templateName == "" {
		return fmt.Errorf("template name is required")
	}

	if _, ok := s.Templates[templateName]; !ok {
		return fmt.Errorf("unknown template: %s", templateName)
	}

	if _, exists := s.Instances[name]; exists {
		return fmt.Errorf("instance already exists: %s", name)
	}

	if params == nil {
		params = map[string]string{}
	}

	s.Instances[name] = config.Instance{
		Template: templateName,
		Enabled:  enabled,
		Params:   params,
	}

	if s.Store != nil {
		if err := s.Store.Save(s.Instances); err != nil {
			// rollback in-memory on failure
			delete(s.Instances, name)
			return err
		}
	}

	// create directories (best effort)
	_ = s.EnsureDirs(name)

	return nil
}

// DeleteInstance deletes an instance and persists. Optionally deletes disk directory.
func (s *Service) DeleteInstance(name string, force bool, deleteData bool) error {
	s.mu.Lock()
	defer s.mu.Unlock()

	if _, ok := s.Instances[name]; !ok {
		return fmt.Errorf("unknown instance: %s", name)
	}

	st := s.Mgr.Status(name)
	if st.Running {
		if !force {
			return fmt.Errorf("instance %q is running; use force to stop it before delete", name)
		}
		_, _ = s.Mgr.Stop(name)
	}

	delete(s.Instances, name)

	if s.Store != nil {
		if err := s.Store.Save(s.Instances); err != nil {
			return err
		}
	}

	if deleteData {
		_ = os.RemoveAll(s.InstanceDir(name))
	}

	return nil
}

func (s *Service) ResolveConfig(instanceName string) (manager.ServerConfig, string, error) {
	s.mu.Lock()
	inst, ok := s.Instances[instanceName]
	s.mu.Unlock()

	if !ok {
		return manager.ServerConfig{}, "", fmt.Errorf("unknown instance: %s", instanceName)
	}
	if !inst.Enabled {
		return manager.ServerConfig{}, "", fmt.Errorf("instance %q is disabled", instanceName)
	}

	tpl, ok := s.Templates[inst.Template]
	if !ok {
		return manager.ServerConfig{}, "", fmt.Errorf("instance %q references unknown template %q", instanceName, inst.Template)
	}

	if err := s.EnsureDirs(instanceName); err != nil {
		return manager.ServerConfig{}, "", fmt.Errorf("ensure dirs: %w", err)
	}

	instanceDir := s.InstanceDir(instanceName)
	logPath := s.LogPath(instanceName)

	ctx := map[string]string{}
	for k, v := range inst.Params {
		ctx[k] = v
	}
	ctx["instance_name"] = instanceName
	ctx["instance_dir"] = instanceDir
	ctx["log_path"] = logPath

	command, err := render(tpl.Command, ctx)
	if err != nil {
		return manager.ServerConfig{}, "", fmt.Errorf("render template.command: %w", err)
	}

	args := make([]string, 0, len(tpl.Args))
	for _, a := range tpl.Args {
		r, err := render(a, ctx)
		if err != nil {
			return manager.ServerConfig{}, "", fmt.Errorf("render template.args: %w", err)
		}
		args = append(args, r)
	}

	cwd, err := render(tpl.Cwd, ctx)
	if err != nil {
		return manager.ServerConfig{}, "", fmt.Errorf("render template.cwd: %w", err)
	}

	env := make([]string, 0, len(tpl.Env))
	for _, e := range tpl.Env {
		r, err := render(e, ctx)
		if err != nil {
			return manager.ServerConfig{}, "", fmt.Errorf("render template.env: %w", err)
		}
		env = append(env, r)
	}

	stopCfg, err := config.ConvertStopPublic(instanceName, tpl.Stop)
	if err != nil {
		return manager.ServerConfig{}, "", err
	}

	cfg := manager.ServerConfig{
		Name:    instanceName,
		Command: command,
		Args:    args,
		Cwd:     cwd,
		Env:     env,
		Stop:    stopCfg,
	}

	return cfg, logPath, nil
}

func render(tmpl string, ctx map[string]string) (string, error) {
	t, err := template.New("x").Option("missingkey=error").Parse(tmpl)
	if err != nil {
		return "", err
	}
	var buf bytes.Buffer
	if err := t.Execute(&buf, ctx); err != nil {
		return "", err
	}
	return buf.String(), nil
}
