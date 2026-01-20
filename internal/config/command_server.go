package config

import (
	"fmt"
	"os"

	"gopkg.in/yaml.v3"
)

type CommandServerConfig struct {
	TCPAddr  string                 `yaml:"tcp_addr"`
	HTTPAddr string                 `yaml:"http_addr"`
	TLS      CommandServerTLSConfig `yaml:"tls"`
}

type CommandServerTLSConfig struct {
	CAFile        string   `yaml:"ca_file"`
	CertFile      string   `yaml:"cert_file"`
	KeyFile       string   `yaml:"key_file"`
	AllowedAgents []string `yaml:"allowed_agents"`
}

func LoadCommandServer(path string) (*CommandServerConfig, error) {
	b, err := os.ReadFile(path)
	if err != nil {
		return nil, fmt.Errorf("read command server config %q: %w", path, err)
	}

	var cfg CommandServerConfig
	if err := yaml.Unmarshal(b, &cfg); err != nil {
		return nil, fmt.Errorf("parse command server yaml %q: %w", path, err)
	}

	if cfg.TCPAddr == "" {
		cfg.TCPAddr = "0.0.0.0:9090"
	}
	if cfg.HTTPAddr == "" {
		cfg.HTTPAddr = "0.0.0.0:8080"
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
