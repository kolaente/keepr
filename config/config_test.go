package config

import (
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
