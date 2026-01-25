package config

import (
	"os"
	"strings"
	"testing"
)

func TestParseConfig(t *testing.T) {
	yaml := `
backup_base_path: /backups
web:
  listen: ":8080"
servers:
  - name: server1
    type: rsync
    host: example.com
    port: 22
    user: backup
    key: /path/to/key
    schedule: "0 2 * * *"
    paths:
      - remote: /data
        local: /local/data
`
	cfg, err := Parse(strings.NewReader(yaml))
	if err != nil {
		t.Fatalf("Parse failed: %v", err)
	}

	if cfg.BackupBasePath != "/backups" {
		t.Errorf("BackupBasePath = %q, want /backups", cfg.BackupBasePath)
	}

	if cfg.Web.Listen != ":8080" {
		t.Errorf("Web.Listen = %q, want :8080", cfg.Web.Listen)
	}

	if len(cfg.Servers) != 1 {
		t.Fatalf("len(Servers) = %d, want 1", len(cfg.Servers))
	}

	srv := cfg.Servers[0]
	if srv.Name != "server1" {
		t.Errorf("Server.Name = %q, want server1", srv.Name)
	}
	if srv.Host != "example.com" {
		t.Errorf("Server.Host = %q, want example.com", srv.Host)
	}
}

func TestApplyDefaults(t *testing.T) {
	yaml := `
backup_base_path: /backups
defaults:
  user: backup
  port: 42541
  retention_days: 7
servers:
  - name: server1
    host: example.com
    port: 22
  - name: server2
    host: other.com
`
	cfg, err := Parse(strings.NewReader(yaml))
	if err != nil {
		t.Fatalf("Parse failed: %v", err)
	}

	cfg.ApplyDefaults()

	// Server1: port 22 should NOT be overridden by default 42541
	if cfg.Servers[0].Port != 22 {
		t.Errorf("Server1 Port = %d, want 22 (should not be overridden)", cfg.Servers[0].Port)
	}
	// Server1: user should be applied from defaults
	if cfg.Servers[0].User != "backup" {
		t.Errorf("Server1 User = %q, want backup", cfg.Servers[0].User)
	}
	// Server1: retention_days should be applied from defaults
	if cfg.Servers[0].RetentionDays != 7 {
		t.Errorf("Server1 RetentionDays = %d, want 7", cfg.Servers[0].RetentionDays)
	}

	// Server2: port should be applied from defaults (was 0)
	if cfg.Servers[1].Port != 42541 {
		t.Errorf("Server2 Port = %d, want 42541", cfg.Servers[1].Port)
	}
	// Server2: user should be applied from defaults
	if cfg.Servers[1].User != "backup" {
		t.Errorf("Server2 User = %q, want backup", cfg.Servers[1].User)
	}

	// Type should default to "remote" when empty
	if cfg.Servers[0].Type != "remote" {
		t.Errorf("Server1 Type = %q, want remote", cfg.Servers[0].Type)
	}
}

func TestValidate(t *testing.T) {
	tests := []struct {
		name    string
		yaml    string
		wantErr bool
	}{
		{
			name: "missing backup_base_path",
			yaml: `
servers:
  - name: server1
    host: example.com
    paths:
      - remote: /data
`,
			wantErr: true,
		},
		{
			name: "duplicate server names",
			yaml: `
backup_base_path: /backups
servers:
  - name: server1
    host: example.com
    paths:
      - remote: /data
  - name: server1
    host: other.com
    paths:
      - remote: /data
`,
			wantErr: true,
		},
		{
			name: "remote server missing host",
			yaml: `
backup_base_path: /backups
servers:
  - name: server1
    type: remote
    paths:
      - remote: /data
`,
			wantErr: true,
		},
		{
			name: "server missing paths",
			yaml: `
backup_base_path: /backups
servers:
  - name: server1
    host: example.com
`,
			wantErr: true,
		},
		{
			name: "valid config",
			yaml: `
backup_base_path: /backups
servers:
  - name: server1
    host: example.com
    paths:
      - remote: /data
`,
			wantErr: false,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			cfg, err := Parse(strings.NewReader(tt.yaml))
			if err != nil {
				t.Fatalf("Parse failed: %v", err)
			}
			cfg.ApplyDefaults()
			err = cfg.Validate()
			if (err != nil) != tt.wantErr {
				t.Errorf("Validate() error = %v, wantErr %v", err, tt.wantErr)
			}
		})
	}
}

func TestLoadFile(t *testing.T) {
	// Create temp file with valid YAML config
	content := []byte(`
backup_base_path: /backups
web:
  listen: ":8080"
servers:
  - name: server1
    host: example.com
    port: 22
    paths:
      - remote: /data
`)
	tmpfile, err := os.CreateTemp("", "config*.yaml")
	if err != nil {
		t.Fatalf("Failed to create temp file: %v", err)
	}
	defer func() { _ = os.Remove(tmpfile.Name()) }()

	if _, err := tmpfile.Write(content); err != nil {
		t.Fatalf("Failed to write temp file: %v", err)
	}
	if err := tmpfile.Close(); err != nil {
		t.Fatalf("Failed to close temp file: %v", err)
	}

	cfg, err := LoadFile(tmpfile.Name())
	if err != nil {
		t.Fatalf("LoadFile failed: %v", err)
	}

	if cfg.BackupBasePath != "/backups" {
		t.Errorf("BackupBasePath = %q, want /backups", cfg.BackupBasePath)
	}
	if cfg.Web.Listen != ":8080" {
		t.Errorf("Web.Listen = %q, want :8080", cfg.Web.Listen)
	}
	if len(cfg.Servers) != 1 {
		t.Fatalf("len(Servers) = %d, want 1", len(cfg.Servers))
	}
	// Verify defaults were applied
	if cfg.Servers[0].Type != "remote" {
		t.Errorf("Server.Type = %q, want remote (from defaults)", cfg.Servers[0].Type)
	}
}

func TestLoadFile_NotFound(t *testing.T) {
	_, err := LoadFile("/nonexistent/path/config.yaml")
	if err == nil {
		t.Error("LoadFile should fail for nonexistent file")
	}
}

func TestLoadFile_Invalid(t *testing.T) {
	// Create temp file with invalid config (missing backup_base_path)
	content := []byte(`
servers:
  - name: server1
    host: example.com
    paths:
      - remote: /data
`)
	tmpfile, err := os.CreateTemp("", "config*.yaml")
	if err != nil {
		t.Fatalf("Failed to create temp file: %v", err)
	}
	defer func() { _ = os.Remove(tmpfile.Name()) }()

	if _, err := tmpfile.Write(content); err != nil {
		t.Fatalf("Failed to write temp file: %v", err)
	}
	if err := tmpfile.Close(); err != nil {
		t.Fatalf("Failed to close temp file: %v", err)
	}

	_, err = LoadFile(tmpfile.Name())
	if err == nil {
		t.Error("LoadFile should fail for invalid config")
	}
}
