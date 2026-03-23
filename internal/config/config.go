package config

import (
	"fmt"
	"os"
	"path/filepath"

	"gopkg.in/yaml.v3"
)

// Config represents the application configuration.
type Config struct {
	LLM LLMConfig `yaml:"llm"`
}

// LLMConfig holds LLM-related settings.
type LLMConfig struct {
	BaseURL string `yaml:"base_url"`
	APIKey  string `yaml:"api_key"`
	Model   string `yaml:"model"`
}

// Default values
const (
	DefaultBaseURL = "https://api.openai.com/v1"
	DefaultModel   = "gpt-4o-mini"
)

// DefaultConfigPath returns the default config file path.
func DefaultConfigPath() (string, error) {
	home, err := os.UserHomeDir()
	if err != nil {
		return "", err
	}
	return filepath.Join(home, ".blogwatcher", "config.yaml"), nil
}

// DefaultDataDir returns the default data directory.
func DefaultDataDir() (string, error) {
	home, err := os.UserHomeDir()
	if err != nil {
		return "", err
	}
	return filepath.Join(home, ".blogwatcher"), nil
}

// Load reads config from the given path. Returns default config if file doesn't exist.
func Load(path string) (*Config, error) {
	if path == "" {
		var err error
		path, err = DefaultConfigPath()
		if err != nil {
			return nil, err
		}
	}

	data, err := os.ReadFile(path)
	if err != nil {
		if os.IsNotExist(err) {
			return &Config{
				LLM: LLMConfig{
					BaseURL: DefaultBaseURL,
					Model:   DefaultModel,
				},
			}, nil
		}
		return nil, err
	}

	var cfg Config
	if err := yaml.Unmarshal(data, &cfg); err != nil {
		return nil, fmt.Errorf("parse config: %w", err)
	}

	// Apply defaults
	if cfg.LLM.BaseURL == "" {
		cfg.LLM.BaseURL = DefaultBaseURL
	}
	if cfg.LLM.Model == "" {
		cfg.LLM.Model = DefaultModel
	}

	return &cfg, nil
}

// Save writes config to the given path.
func (c *Config) Save(path string) error {
	if path == "" {
		var err error
		path, err = DefaultConfigPath()
		if err != nil {
			return err
		}
	}

	if err := os.MkdirAll(filepath.Dir(path), 0o755); err != nil {
		return err
	}

	data, err := yaml.Marshal(c)
	if err != nil {
		return err
	}

	return os.WriteFile(path, data, 0o600)
}

// IsConfigured returns true if the LLM is properly configured with an API key.
func (c *Config) IsConfigured() bool {
	return c.LLM.APIKey != ""
}