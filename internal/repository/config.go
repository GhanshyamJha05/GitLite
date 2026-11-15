package repository

import (
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"strings"
)

// RemoteConfig represents a configured remote repository.
type RemoteConfig struct {
	Name  string `json:"name"`
	URL   string `json:"url"`
	Token string `json:"token,omitempty"`
}

// Config represents the GitLite repository configuration stored in .gitlite/config.
type Config struct {
	Version int                     `json:"version"`
	Remotes map[string]RemoteConfig `json:"remotes,omitempty"`
}

// LoadConfig reads and parses the .gitlite/config file.
func (r *Repository) LoadConfig() (*Config, error) {
	configPath := filepath.Join(r.GitLiteDir, ConfigFile)
	data, err := os.ReadFile(configPath)
	if err != nil {
		if os.IsNotExist(err) {
			return &Config{
				Version: 1,
				Remotes: make(map[string]RemoteConfig),
			}, nil
		}
		return nil, fmt.Errorf("reading config file: %w", err)
	}

	var cfg Config
	if err := json.Unmarshal(data, &cfg); err != nil {
		return nil, fmt.Errorf("parsing config file: %w", err)
	}
	if cfg.Remotes == nil {
		cfg.Remotes = make(map[string]RemoteConfig)
	}
	return &cfg, nil
}

// SaveConfig writes the repository configuration to .gitlite/config safely.
func (r *Repository) SaveConfig(cfg *Config) error {
	configPath := filepath.Join(r.GitLiteDir, ConfigFile)
	data, err := json.MarshalIndent(cfg, "", "  ")
	if err != nil {
		return fmt.Errorf("marshaling config: %w", err)
	}
	data = append(data, '\n')
	if err := os.WriteFile(configPath, data, 0644); err != nil {
		return fmt.Errorf("writing config file: %w", err)
	}
	return nil
}

// AddRemote adds or updates a named remote in the repository configuration.
func (r *Repository) AddRemote(name, url, token string) error {
	name = strings.TrimSpace(name)
	url = strings.TrimSpace(url)
	if name == "" {
		return fmt.Errorf("remote name cannot be empty")
	}
	if url == "" {
		return fmt.Errorf("remote URL cannot be empty")
	}

	cfg, err := r.LoadConfig()
	if err != nil {
		return err
	}

	cfg.Remotes[name] = RemoteConfig{
		Name:  name,
		URL:   url,
		Token: token,
	}
	return r.SaveConfig(cfg)
}

// RemoveRemote removes a named remote from configuration.
func (r *Repository) RemoveRemote(name string) error {
	cfg, err := r.LoadConfig()
	if err != nil {
		return err
	}
	if _, exists := cfg.Remotes[name]; !exists {
		return fmt.Errorf("remote %q not found", name)
	}
	delete(cfg.Remotes, name)
	return r.SaveConfig(cfg)
}

// GetRemote retrieves the configuration for a given remote name.
func (r *Repository) GetRemote(name string) (*RemoteConfig, error) {
	cfg, err := r.LoadConfig()
	if err != nil {
		return nil, err
	}
	rem, exists := cfg.Remotes[name]
	if !exists {
		return nil, fmt.Errorf("remote %q not found", name)
	}
	return &rem, nil
}
