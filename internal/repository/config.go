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

// UserConfig represents user identity configuration.
type UserConfig struct {
	Name  string `json:"name,omitempty"`
	Email string `json:"email,omitempty"`
}

// Config represents the GitLite repository configuration stored in .gitlite/config.
type Config struct {
	Version int                     `json:"version"`
	User    UserConfig              `json:"user,omitempty"`
	Remotes map[string]RemoteConfig `json:"remotes,omitempty"`
	Values  map[string]string       `json:"values,omitempty"`
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
	if cfg.Values == nil {
		cfg.Values = make(map[string]string)
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

// GetConfigValue retrieves a setting by dot-notation key (e.g. "user.name", "user.email", or custom key).
func (r *Repository) GetConfigValue(key string) (string, error) {
	cfg, err := r.LoadConfig()
	if err != nil {
		return "", err
	}
	key = strings.ToLower(strings.TrimSpace(key))
	switch key {
	case "user.name":
		return cfg.User.Name, nil
	case "user.email":
		return cfg.User.Email, nil
	default:
		if val, exists := cfg.Values[key]; exists {
			return val, nil
		}
		return "", fmt.Errorf("config key %q not set", key)
	}
}

// SetConfigValue updates a setting by dot-notation key.
func (r *Repository) SetConfigValue(key, value string) error {
	cfg, err := r.LoadConfig()
	if err != nil {
		return err
	}
	key = strings.ToLower(strings.TrimSpace(key))
	value = strings.TrimSpace(value)

	switch key {
	case "user.name":
		cfg.User.Name = value
	case "user.email":
		cfg.User.Email = value
	default:
		if cfg.Values == nil {
			cfg.Values = make(map[string]string)
		}
		cfg.Values[key] = value
	}
	return r.SaveConfig(cfg)
}

// ListConfig returns a map of all configured key-value pairs.
func (r *Repository) ListConfig() (map[string]string, error) {
	cfg, err := r.LoadConfig()
	if err != nil {
		return nil, err
	}
	res := make(map[string]string)
	if cfg.User.Name != "" {
		res["user.name"] = cfg.User.Name
	}
	if cfg.User.Email != "" {
		res["user.email"] = cfg.User.Email
	}
	for k, v := range cfg.Values {
		res[k] = v
	}
	for name, rem := range cfg.Remotes {
		res["remote."+name+".url"] = rem.URL
	}
	return res, nil
}
