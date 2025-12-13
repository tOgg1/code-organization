package config

import (
	"encoding/json"
	"os"
	"path/filepath"
)

type ServerConfig struct {
	SSH      string `json:"ssh"`
	CodeRoot string `json:"code_root,omitempty"`
}

type Config struct {
	Schema   int                     `json:"schema"`
	CodeRoot string                  `json:"code_root"`
	Editor   string                  `json:"editor,omitempty"`
	Servers  map[string]ServerConfig `json:"servers,omitempty"`
}

const CurrentConfigSchema = 1

func DefaultConfig() *Config {
	home, _ := os.UserHomeDir()
	return &Config{
		Schema:   CurrentConfigSchema,
		CodeRoot: filepath.Join(home, "Code"),
		Editor:   "",
		Servers:  map[string]ServerConfig{},
	}
}

func Load(configPath string) (*Config, error) {
	paths := getConfigPaths(configPath)

	for _, path := range paths {
		if path == "" {
			continue
		}

		data, err := os.ReadFile(path)
		if err != nil {
			if os.IsNotExist(err) {
				continue
			}
			return nil, err
		}

		var cfg Config
		if err := json.Unmarshal(data, &cfg); err != nil {
			return nil, err
		}

		cfg.expandPaths()
		return &cfg, nil
	}

	return DefaultConfig(), nil
}

func getConfigPaths(explicit string) []string {
	home, _ := os.UserHomeDir()

	var paths []string

	if explicit != "" {
		paths = append(paths, explicit)
	}

	xdgConfig := os.Getenv("XDG_CONFIG_HOME")
	if xdgConfig == "" {
		xdgConfig = filepath.Join(home, ".config")
	}
	paths = append(paths, filepath.Join(xdgConfig, "co", "config.json"))

	paths = append(paths, filepath.Join(home, "Code", "_system", "config.json"))

	return paths
}

func (c *Config) expandPaths() {
	home, _ := os.UserHomeDir()

	if len(c.CodeRoot) > 0 && c.CodeRoot[0] == '~' {
		c.CodeRoot = filepath.Join(home, c.CodeRoot[1:])
	}

	for name, server := range c.Servers {
		if server.CodeRoot == "" {
			server.CodeRoot = "~/Code"
		}
		c.Servers[name] = server
	}
}

func (c *Config) GetServer(name string) *ServerConfig {
	if server, ok := c.Servers[name]; ok {
		return &server
	}
	return &ServerConfig{
		SSH:      name,
		CodeRoot: "~/Code",
	}
}

func (c *Config) SystemDir() string {
	return filepath.Join(c.CodeRoot, "_system")
}

func (c *Config) IndexPath() string {
	return filepath.Join(c.SystemDir(), "index.jsonl")
}

func (c *Config) ArchiveDir() string {
	return filepath.Join(c.SystemDir(), "archive")
}

func (c *Config) LogsDir() string {
	return filepath.Join(c.SystemDir(), "logs")
}

func (c *Config) CacheDir() string {
	return filepath.Join(c.SystemDir(), "cache")
}

func (c *Config) WorkspacePath(slug string) string {
	return filepath.Join(c.CodeRoot, slug)
}
