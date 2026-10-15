package config

import (
	"encoding/json"
	"os"
	"path/filepath"
)

const Version = "0.1.0"

type AppConfig struct {
	DefaultKeyType string `json:"default_key_type"`
	DefaultPort    int    `json:"default_port"`
	SessionLogging bool   `json:"session_logging"`
	UseSSHAgent    bool   `json:"use_ssh_agent"`
}

var DefaultConfig = AppConfig{
	DefaultKeyType: "ed25519",
	DefaultPort:    22,
	SessionLogging: true,
	UseSSHAgent:    true,
}

func Dir() string {
	if d := os.Getenv("SPOCK_DIR"); d != "" {
		return d
	}
	home, _ := os.UserHomeDir()
	return filepath.Join(home, ".spock")
}

func KeysDir() string            { return filepath.Join(Dir(), "keys") }
func LogsDir() string            { return filepath.Join(Dir(), "logs") }
func SessionsDir() string        { return filepath.Join(Dir(), "sessions") }
func SocketsDir() string         { return filepath.Join(SessionsDir(), "sockets") }
func ConfigPath() string         { return filepath.Join(Dir(), "config.json") }
func ConnectionsPath() string    { return filepath.Join(Dir(), "connections.json") }
func ActiveSessionsPath() string { return filepath.Join(SessionsDir(), "active.json") }

func EnsureDirs() error {
	for _, d := range []string{Dir(), KeysDir(), LogsDir(), SessionsDir(), SocketsDir()} {
		if err := os.MkdirAll(d, 0700); err != nil {
			return err
		}
	}
	return nil
}

func Load() (*AppConfig, error) {
	if err := EnsureDirs(); err != nil {
		return nil, err
	}
	data, err := os.ReadFile(ConfigPath())
	if err != nil {
		if os.IsNotExist(err) {
			cfg := DefaultConfig
			return &cfg, nil
		}
		return nil, err
	}
	cfg := DefaultConfig
	if err := json.Unmarshal(data, &cfg); err != nil {
		return nil, err
	}
	return &cfg, nil
}

func Save(cfg *AppConfig) error {
	if err := EnsureDirs(); err != nil {
		return err
	}
	data, err := json.MarshalIndent(cfg, "", "  ")
	if err != nil {
		return err
	}
	return os.WriteFile(ConfigPath(), data, 0644)
}
