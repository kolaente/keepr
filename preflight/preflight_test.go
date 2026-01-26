package preflight

import (
	"strings"
	"testing"

	"keepr/config"
)

func TestRunAll_EmptyConfig(t *testing.T) {
	cfg := &config.Config{
		BackupBasePath: "/tmp/test-backups",
		Servers:        []config.Server{},
	}

	errors := RunAll(cfg)

	// Should pass with empty servers (only base checks)
	// rsync check may fail if rsync not installed, filter it out
	var realErrors []error
	for _, err := range errors {
		if !strings.Contains(err.Error(), "rsync") {
			realErrors = append(realErrors, err)
		}
	}

	if len(realErrors) > 0 {
		t.Errorf("Expected no errors for empty config, got: %v", realErrors)
	}
}

func TestRunAll_ReturnsMultipleErrors(t *testing.T) {
	cfg := &config.Config{
		BackupBasePath: "/nonexistent/path/that/does/not/exist",
		Servers: []config.Server{
			{
				Name: "test-server",
				Type: "remote",
				Host: "invalid.host.example.com",
				Port: 22,
				User: "backup",
				Key:  "/nonexistent/ssh/key",
			},
		},
	}

	errors := RunAll(cfg)

	if len(errors) == 0 {
		t.Error("Expected errors for invalid config, got none")
	}

	// Should have multiple errors (backup path, SSH key)
	if len(errors) < 2 {
		t.Errorf("Expected at least 2 errors, got %d: %v", len(errors), errors)
	}
}
