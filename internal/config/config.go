// Package config stores the X app credentials and user tokens for xTui.
package config

import (
	"encoding/json"
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"sync"
	"time"
)

// Config is the on-disk session. Tokens are written with mode 0600.
type Config struct {
	ClientID     string    `json:"client_id,omitempty"`
	ClientSecret string    `json:"client_secret,omitempty"`
	AccessToken  string    `json:"access_token,omitempty"`
	RefreshToken string    `json:"refresh_token,omitempty"`
	Expiry       time.Time `json:"expiry,omitempty"`
	RedirectPort int       `json:"redirect_port,omitempty"`
}

// Port is the loopback port used for the OAuth redirect.
func (c Config) Port() int {
	if c.RedirectPort == 0 {
		return 53682
	}
	return c.RedirectPort
}

// RedirectURI is the callback that must be registered on the X app.
func (c Config) RedirectURI() string {
	return fmt.Sprintf("http://127.0.0.1:%d/callback", c.Port())
}

// Dir is the configuration directory. XTUI_CONFIG_DIR overrides it.
func Dir() (string, error) {
	if d := os.Getenv("XTUI_CONFIG_DIR"); d != "" {
		return d, nil
	}
	base, err := os.UserConfigDir()
	if err != nil {
		return "", err
	}
	return filepath.Join(base, "xtui"), nil
}

// Path is the config file path.
func Path() (string, error) {
	dir, err := Dir()
	if err != nil {
		return "", err
	}
	return filepath.Join(dir, "config.json"), nil
}

// Load reads the config file. A missing file is an empty config.
func Load() (Config, error) {
	path, err := Path()
	if err != nil {
		return Config{}, err
	}
	b, err := os.ReadFile(path)
	if err != nil {
		if errors.Is(err, os.ErrNotExist) {
			return Config{}, nil
		}
		return Config{}, err
	}
	var cfg Config
	if err := json.Unmarshal(b, &cfg); err != nil {
		return Config{}, fmt.Errorf("read %s: %w", path, err)
	}
	return cfg, nil
}

// Save writes the config atomically.
func Save(cfg Config) error {
	path, err := Path()
	if err != nil {
		return err
	}
	if err := os.MkdirAll(filepath.Dir(path), 0o700); err != nil {
		return err
	}
	b, err := json.MarshalIndent(cfg, "", "  ")
	if err != nil {
		return err
	}
	b = append(b, '\n')
	tmp := path + ".tmp"
	if err := os.WriteFile(tmp, b, 0o600); err != nil {
		return err
	}
	if err := os.Chmod(tmp, 0o600); err != nil {
		_ = os.Remove(tmp)
		return err
	}
	return os.Rename(tmp, path)
}

// ApplyEnv overlays XTUI_CLIENT_ID, XTUI_CLIENT_SECRET, XTUI_ACCESS_TOKEN,
// and XTUI_REFRESH_TOKEN when they are set.
func ApplyEnv(cfg Config) Config {
	if v := os.Getenv("XTUI_CLIENT_ID"); v != "" {
		cfg.ClientID = v
	}
	if v := os.Getenv("XTUI_CLIENT_SECRET"); v != "" {
		cfg.ClientSecret = v
	}
	if v := os.Getenv("XTUI_ACCESS_TOKEN"); v != "" {
		cfg.AccessToken = v
	}
	if v := os.Getenv("XTUI_REFRESH_TOKEN"); v != "" {
		cfg.RefreshToken = v
	}
	return cfg
}

// ClearTokens forgets the session and keeps the app credentials.
func ClearTokens(cfg Config) Config {
	cfg.AccessToken = ""
	cfg.RefreshToken = ""
	cfg.Expiry = time.Time{}
	return cfg
}

// Store is the config plus a mutex so token refresh and the UI can share it.
type Store struct {
	mu  sync.Mutex
	cfg Config
}

// NewStore returns a store holding cfg.
func NewStore(cfg Config) *Store {
	return &Store{cfg: cfg}
}

// Get returns a copy of the config.
func (s *Store) Get() Config {
	s.mu.Lock()
	defer s.mu.Unlock()
	return s.cfg
}

// Update mutates the config and saves it.
func (s *Store) Update(fn func(*Config)) error {
	s.mu.Lock()
	fn(&s.cfg)
	cfg := s.cfg
	s.mu.Unlock()
	return Save(cfg)
}
