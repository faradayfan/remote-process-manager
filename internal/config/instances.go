package config

import (
	"fmt"
	"os"

	"gopkg.in/yaml.v3"
)

type InstanceConfig struct {
	Instances map[string]Instance `yaml:"instances"`
}

type Instance struct {
	Template string            `yaml:"template"`
	Enabled  bool              `yaml:"enabled"`
	Params   map[string]string `yaml:"params"`
}

func LoadInstances(path string) (*InstanceConfig, error) {
	b, err := os.ReadFile(path)
	if err != nil {
		return nil, fmt.Errorf("read instances file %q: %w", path, err)
	}

	var cfg InstanceConfig
	if err := yaml.Unmarshal(b, &cfg); err != nil {
		return nil, fmt.Errorf("parse instances yaml %q: %w", path, err)
	}

	if cfg.Instances == nil {
		cfg.Instances = map[string]Instance{}
	}

	for name, inst := range cfg.Instances {
		if name == "" {
			return nil, fmt.Errorf("instance name cannot be empty")
		}
		if inst.Template == "" {
			return nil, fmt.Errorf("instance %q missing template", name)
		}
		if inst.Params == nil {
			cfg.Instances[name] = Instance{
				Template: inst.Template,
				Enabled:  inst.Enabled,
				Params:   map[string]string{},
			}
		}
	}

	return &cfg, nil
}
