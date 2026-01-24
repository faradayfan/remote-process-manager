package config

import (
	"fmt"
	"os"
	"time"

	"gopkg.in/yaml.v3"
)

type APIKeysConfig struct {
	Keys []APIKey `yaml:"keys"`
}

type APIKey struct {
	Name      string    `yaml:"name"`
	Key       string    `yaml:"key"`
	Roles     []string  `yaml:"roles"`
	CreatedAt time.Time `yaml:"created_at"`
}

func LoadAPIKeys(path string) (*APIKeysConfig, error) {
	b, err := os.ReadFile(path)
	if err != nil {
		if os.IsNotExist(err) {
			return &APIKeysConfig{Keys: []APIKey{}}, nil
		}
		return nil, fmt.Errorf("read api keys file %q: %w", path, err)
	}

	var cfg APIKeysConfig
	if err := yaml.Unmarshal(b, &cfg); err != nil {
		return nil, fmt.Errorf("parse api keys yaml %q: %w", path, err)
	}

	if cfg.Keys == nil {
		cfg.Keys = []APIKey{}
	}

	return &cfg, nil
}

func SaveAPIKeys(path string, cfg *APIKeysConfig) error {
	b, err := yaml.Marshal(cfg)
	if err != nil {
		return fmt.Errorf("marshal api keys: %w", err)
	}

	// Atomic write: write temp then rename
	tmp := path + ".tmp"
	if err := os.WriteFile(tmp, b, 0600); err != nil {
		return fmt.Errorf("write temp api keys file: %w", err)
	}

	if err := os.Rename(tmp, path); err != nil {
		return fmt.Errorf("rename temp -> api keys file: %w", err)
	}

	return nil
}
