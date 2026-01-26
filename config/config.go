package config

import (
	"errors"
	"fmt"
	"io"
	"os"
	"path/filepath"

	"gopkg.in/yaml.v3"
)

type Config struct {
	BackupBasePath string    `yaml:"backup_base_path"`
	Web            WebConfig `yaml:"web"`
	Defaults       Server    `yaml:"defaults"`
	Servers        []Server  `yaml:"servers"`
}

type WebConfig struct {
	Listen    string `yaml:"listen"`
	APISecret string `yaml:"api_secret"`
}

type Server struct {
	Name          string `yaml:"name"`
	Type          string `yaml:"type"`
	Host          string `yaml:"host"`
	Port          int    `yaml:"port"`
	User          string `yaml:"user"`
	Key           string `yaml:"key"`
	RsyncPath     string `yaml:"rsync_path"`
	Schedule      string `yaml:"schedule"`
	Heartbeat     string `yaml:"heartbeat"`
	PreHook       string `yaml:"pre_hook"`
	PostHook      string `yaml:"post_hook"`
	RetentionDays int    `yaml:"retention_days"`
	Paths         []Path `yaml:"paths"`
}

type Path struct {
	Remote    string `yaml:"remote"`
	Local     string `yaml:"local"`
	BackupDir string `yaml:"backup_dir"`
}

func Parse(r io.Reader) (*Config, error) {
	var cfg Config
	if err := yaml.NewDecoder(r).Decode(&cfg); err != nil {
		return nil, err
	}
	return &cfg, nil
}

func LoadFile(path string) (*Config, error) {
	f, err := os.Open(path)
	if err != nil {
		return nil, err
	}
	defer func() { _ = f.Close() }()

	cfg, err := Parse(f)
	if err != nil {
		return nil, err
	}

	cfg.ApplyDefaults()

	if err := cfg.Validate(); err != nil {
		return nil, err
	}

	return cfg, nil
}

func (c *Config) ApplyDefaults() {
	for i := range c.Servers {
		s := &c.Servers[i]

		// Set type to "remote" if empty
		if s.Type == "" {
			s.Type = "remote"
		}

		// Apply defaults only if server value is zero/empty
		if s.User == "" {
			s.User = c.Defaults.User
		}
		if s.Port == 0 {
			s.Port = c.Defaults.Port
		}
		if s.Key == "" {
			s.Key = c.Defaults.Key
		}
		if s.RsyncPath == "" {
			s.RsyncPath = c.Defaults.RsyncPath
		}
		if s.Schedule == "" {
			s.Schedule = c.Defaults.Schedule
		}
		if s.PreHook == "" {
			s.PreHook = c.Defaults.PreHook
		}
		if s.PostHook == "" {
			s.PostHook = c.Defaults.PostHook
		}
		if s.RetentionDays == 0 {
			s.RetentionDays = c.Defaults.RetentionDays
		}
	}
}

func (c *Config) Validate() error {
	if c.BackupBasePath == "" {
		return errors.New("backup_base_path is required")
	}
	if !filepath.IsAbs(c.BackupBasePath) {
		return errors.New("backup_base_path must be an absolute path")
	}

	// Check for duplicate server names
	seen := make(map[string]bool)
	for _, s := range c.Servers {
		if seen[s.Name] {
			return fmt.Errorf("duplicate server name: %s", s.Name)
		}
		seen[s.Name] = true

		// Check remote servers have host
		if s.Type == "remote" && s.Host == "" {
			return fmt.Errorf("server %s: remote server requires host", s.Name)
		}

		// Check all servers have at least one path
		if len(s.Paths) == 0 {
			return fmt.Errorf("server %s: at least one path is required", s.Name)
		}
	}

	return nil
}
