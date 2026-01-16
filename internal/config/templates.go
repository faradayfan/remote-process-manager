package config

import (
	"fmt"
	"os"

	"gopkg.in/yaml.v3"
)

type TemplateConfig struct {
	Templates map[string]Template `yaml:"templates"`
}

type Template struct {
	Command string   `yaml:"command"`
	Args    []string `yaml:"args"`
	Cwd     string   `yaml:"cwd"`
	Env     []string `yaml:"env"`
	Stop    Stop     `yaml:"stop"`
}

func LoadTemplates(path string) (*TemplateConfig, error) {
	b, err := os.ReadFile(path)
	if err != nil {
		return nil, fmt.Errorf("read templates file %q: %w", path, err)
	}

	var cfg TemplateConfig
	if err := yaml.Unmarshal(b, &cfg); err != nil {
		return nil, fmt.Errorf("parse templates yaml %q: %w", path, err)
	}

	if len(cfg.Templates) == 0 {
		return nil, fmt.Errorf("templates config contains no templates")
	}

	for name, t := range cfg.Templates {
		if name == "" {
			return nil, fmt.Errorf("template name cannot be empty")
		}
		if t.Command == "" {
			return nil, fmt.Errorf("template %q missing command", name)
		}
	}

	return &cfg, nil
}
