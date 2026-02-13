package cli

import (
	"os"
	"path/filepath"

	"gopkg.in/yaml.v3"
)

type Config struct {
	Server string `yaml:"server,omitempty"`
	Token  string `yaml:"token,omitempty"`
}

// ConfigPathOverride allows tests to override the config file path.
var ConfigPathOverride string

func configPath() string {
	if ConfigPathOverride != "" {
		return ConfigPathOverride
	}
	home, _ := os.UserHomeDir()
	return filepath.Join(home, ".design-reviewer.yaml")
}

func LoadConfig() (*Config, error) {
	data, err := os.ReadFile(configPath())
	if err != nil {
		if os.IsNotExist(err) {
			return &Config{}, nil
		}
		return nil, err
	}
	var cfg Config
	if err := yaml.Unmarshal(data, &cfg); err != nil {
		return nil, err
	}
	return &cfg, nil
}

func SaveConfig(cfg *Config) error {
	data, err := yaml.Marshal(cfg)
	if err != nil {
		return err
	}
	return os.WriteFile(configPath(), data, 0600)
}
