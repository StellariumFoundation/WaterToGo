package config

import (
	"encoding/json"
	"os"
	"path/filepath"
)

type Config struct {
	APIKeys []string `json:"api_keys,omitempty"`
}

const configFileName = "watertogo_config.json"

func ConfigPath() (string, error) {
	exe, err := os.Executable()
	if err != nil {
		return "", err
	}
	return filepath.Join(filepath.Dir(exe), configFileName), nil
}

func Load() (*Config, error) {
	path, err := ConfigPath()
	if err != nil {
		return nil, err
	}
	data, err := os.ReadFile(path)
	if err != nil {
		if os.IsNotExist(err) {
			return &Config{}, nil
		}
		return nil, err
	}

	var raw map[string]any
	if err := json.Unmarshal(data, &raw); err != nil {
		return nil, err
	}

	keys, _ := raw["api_keys"].([]any)
	var apiKeys []string
	for _, k := range keys {
		if s, ok := k.(string); ok && s != "" {
			apiKeys = append(apiKeys, s)
		}
	}

	if len(apiKeys) == 0 {
		if oldKey, ok := raw["api_key"].(string); ok && oldKey != "" {
			apiKeys = []string{oldKey}
			cfg := &Config{APIKeys: apiKeys}
			cfg.Save()
			return cfg, nil
		}
	}

	return &Config{APIKeys: apiKeys}, nil
}

func (c *Config) Save() error {
	path, err := ConfigPath()
	if err != nil {
		return err
	}
	data, err := json.MarshalIndent(c, "", "  ")
	if err != nil {
		return err
	}
	return os.WriteFile(path, data, 0644)
}

func (c *Config) HasKeys() bool {
	return len(c.APIKeys) > 0
}
