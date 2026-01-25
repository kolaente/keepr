package config

import (
	"io"

	"gopkg.in/yaml.v3"
)

type Config struct {
	BackupBasePath string    `yaml:"backup_base_path"`
	Web            WebConfig `yaml:"web"`
	Defaults       Server    `yaml:"defaults"`
	Servers        []Server  `yaml:"servers"`
}

type WebConfig struct {
	Listen string `yaml:"listen"`
}

type Server struct {
	Name          string `yaml:"name"`
	Type          string `yaml:"type"`
	Host          string `yaml:"host"`
	Port          int    `yaml:"port"`
	User          string `yaml:"user"`
	Key           string `yaml:"key"`
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
