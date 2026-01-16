package instances

import (
	"fmt"
	"os"
	"path/filepath"

	"gopkg.in/yaml.v3"

	"github.com/faradayfan/remote-process-manager/internal/config"
)

type Store struct {
	Path string
}

func NewStore(path string) *Store {
	return &Store{Path: path}
}

func (s *Store) Load() (map[string]config.Instance, error) {
	cfg, err := config.LoadInstances(s.Path)
	if err != nil {
		return nil, err
	}
	return cfg.Instances, nil
}

func (s *Store) Save(instances map[string]config.Instance) error {
	out := config.InstanceConfig{
		Instances: instances,
	}

	b, err := yaml.Marshal(out)
	if err != nil {
		return fmt.Errorf("marshal instances: %w", err)
	}

	// Ensure parent dir exists
	if err := os.MkdirAll(filepath.Dir(s.Path), 0755); err != nil {
		return fmt.Errorf("mkdir parent: %w", err)
	}

	// Atomic write: write temp then rename
	tmp := s.Path + ".tmp"
	if err := os.WriteFile(tmp, b, 0644); err != nil {
		return fmt.Errorf("write temp instances file: %w", err)
	}

	if err := os.Rename(tmp, s.Path); err != nil {
		return fmt.Errorf("rename temp -> instances file: %w", err)
	}

	return nil
}
