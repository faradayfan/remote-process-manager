package config

import (
	"fmt"
	"os"

	"gopkg.in/yaml.v3"
)

type AgentConfig struct {
	AgentID           string `yaml:"agent_id"`
	CommandServerAddr string `yaml:"command_server_addr"`

	TLS AgentTLSConfig `yaml:"tls"`
}

type AgentTLSConfig struct {
	CAFile   string `yaml:"ca_file"`
	CertFile string `yaml:"cert_file"`
	KeyFile  string `yaml:"key_file"`

	ServerName string `yaml:"server_name"`
}

func LoadAgent(path string) (*AgentConfig, error) {
	b, err := os.ReadFile(path)
	if err != nil {
		return nil, fmt.Errorf("read agent config %q: %w", path, err)
	}

	var cfg AgentConfig
	if err := yaml.Unmarshal(b, &cfg); err != nil {
		return nil, fmt.Errorf("parse agent yaml %q: %w", path, err)
	}

	if cfg.AgentID == "" {
		return nil, fmt.Errorf("agent_id is required")
	}
	if cfg.CommandServerAddr == "" {
		return nil, fmt.Errorf("command_server_addr is required")
	}

	if cfg.TLS.CAFile == "" {
		return nil, fmt.Errorf("tls.ca_file is required")
	}
	if cfg.TLS.CertFile == "" {
		return nil, fmt.Errorf("tls.cert_file is required")
	}
	if cfg.TLS.KeyFile == "" {
		return nil, fmt.Errorf("tls.key_file is required")
	}

	return &cfg, nil
}
