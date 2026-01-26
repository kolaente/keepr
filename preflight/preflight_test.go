package preflight

import (
	"os"
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

func TestCheckRsyncBinary_Found(t *testing.T) {
	cfg := &config.Config{}

	errors := checkRsyncBinary(cfg)

	// rsync should be installed on most systems
	// If this fails, rsync needs to be installed
	if len(errors) > 0 {
		t.Skipf("rsync not found (install it to run this test): %v", errors)
	}
}

func TestCheckRsyncBinary_NotFound(t *testing.T) {
	// Save and restore PATH
	originalPath := os.Getenv("PATH")
	defer os.Setenv("PATH", originalPath)

	// Set PATH to empty to simulate rsync not found
	os.Setenv("PATH", "/nonexistent")

	cfg := &config.Config{}
	errors := checkRsyncBinary(cfg)

	if len(errors) != 1 {
		t.Errorf("Expected 1 error, got %d: %v", len(errors), errors)
	}

	if len(errors) > 0 && !strings.Contains(errors[0].Error(), "rsync") {
		t.Errorf("Error should mention rsync: %v", errors[0])
	}
}

func TestCheckBackupBasePath_Exists(t *testing.T) {
	tmpDir, err := os.MkdirTemp("", "preflight-test")
	if err != nil {
		t.Fatal(err)
	}
	defer os.RemoveAll(tmpDir)

	cfg := &config.Config{
		BackupBasePath: tmpDir,
	}

	errors := checkBackupBasePath(cfg)

	if len(errors) != 0 {
		t.Errorf("Expected no errors for existing writable path, got: %v", errors)
	}
}

func TestCheckBackupBasePath_NotExists(t *testing.T) {
	cfg := &config.Config{
		BackupBasePath: "/nonexistent/path/that/does/not/exist",
	}

	errors := checkBackupBasePath(cfg)

	if len(errors) != 1 {
		t.Errorf("Expected 1 error, got %d: %v", len(errors), errors)
	}

	if len(errors) > 0 && !strings.Contains(errors[0].Error(), "does not exist") {
		t.Errorf("Error should mention path does not exist: %v", errors[0])
	}
}

func TestCheckBackupBasePath_NotWritable(t *testing.T) {
	if os.Getuid() == 0 {
		t.Skip("Cannot test non-writable paths as root")
	}

	cfg := &config.Config{
		BackupBasePath: "/root", // Usually not writable by non-root
	}

	errors := checkBackupBasePath(cfg)

	// Should either not exist or not be writable
	if len(errors) == 0 {
		t.Error("Expected error for non-writable path")
	}
}

func TestCheckSSHKeys_NoRemoteServers(t *testing.T) {
	cfg := &config.Config{
		Servers: []config.Server{
			{Name: "local", Type: "local"},
		},
	}

	errors := checkSSHKeys(cfg)

	if len(errors) != 0 {
		t.Errorf("Expected no errors for local-only servers, got: %v", errors)
	}
}

func TestCheckSSHKeys_KeyExists(t *testing.T) {
	// Create a temp file to act as SSH key
	tmpFile, err := os.CreateTemp("", "test-ssh-key")
	if err != nil {
		t.Fatal(err)
	}
	defer os.Remove(tmpFile.Name())
	tmpFile.Close()

	// Set correct permissions
	os.Chmod(tmpFile.Name(), 0600)

	cfg := &config.Config{
		Servers: []config.Server{
			{
				Name: "remote",
				Type: "remote",
				Host: "example.com",
				Key:  tmpFile.Name(),
			},
		},
	}

	errors := checkSSHKeys(cfg)

	if len(errors) != 0 {
		t.Errorf("Expected no errors for existing key, got: %v", errors)
	}
}

func TestCheckSSHKeys_KeyNotExists(t *testing.T) {
	cfg := &config.Config{
		Servers: []config.Server{
			{
				Name: "remote",
				Type: "remote",
				Host: "example.com",
				Key:  "/nonexistent/ssh/key",
			},
		},
	}

	errors := checkSSHKeys(cfg)

	if len(errors) != 1 {
		t.Errorf("Expected 1 error, got %d: %v", len(errors), errors)
	}

	if len(errors) > 0 && !strings.Contains(errors[0].Error(), "does not exist") {
		t.Errorf("Error should mention key does not exist: %v", errors[0])
	}
}

func TestCheckSSHKeys_KeyBadPermissions(t *testing.T) {
	if os.Getuid() == 0 {
		t.Skip("Cannot test permissions as root")
	}

	// Create a temp file with bad permissions
	tmpFile, err := os.CreateTemp("", "test-ssh-key")
	if err != nil {
		t.Fatal(err)
	}
	defer os.Remove(tmpFile.Name())
	tmpFile.Close()

	// Set world-readable permissions (bad for SSH keys)
	os.Chmod(tmpFile.Name(), 0644)

	cfg := &config.Config{
		Servers: []config.Server{
			{
				Name: "remote",
				Type: "remote",
				Host: "example.com",
				Key:  tmpFile.Name(),
			},
		},
	}

	errors := checkSSHKeys(cfg)

	if len(errors) != 1 {
		t.Errorf("Expected 1 error for bad permissions, got %d: %v", len(errors), errors)
	}

	if len(errors) > 0 && !strings.Contains(errors[0].Error(), "permissions") {
		t.Errorf("Error should mention permissions: %v", errors[0])
	}
}

func TestCheckSSHKeys_NoKeyForRemote(t *testing.T) {
	cfg := &config.Config{
		Servers: []config.Server{
			{
				Name: "remote",
				Type: "remote",
				Host: "example.com",
				// No Key specified
			},
		},
	}

	errors := checkSSHKeys(cfg)

	// Warning but not error - SSH might use default key or agent
	// This is acceptable, just log a warning
	if len(errors) != 0 {
		t.Errorf("No key specified should not be an error (might use ssh-agent), got: %v", errors)
	}
}

func TestCheckCronSchedules_Valid(t *testing.T) {
	cfg := &config.Config{
		Servers: []config.Server{
			{Name: "server1", Schedule: "0 2 * * *"},
			{Name: "server2", Schedule: "*/15 * * * *"},
			{Name: "server3", Schedule: "@daily"},
		},
	}

	errors := checkCronSchedules(cfg)

	if len(errors) != 0 {
		t.Errorf("Expected no errors for valid schedules, got: %v", errors)
	}
}

func TestCheckCronSchedules_Invalid(t *testing.T) {
	cfg := &config.Config{
		Servers: []config.Server{
			{Name: "server1", Schedule: "invalid cron"},
			{Name: "server2", Schedule: "* * *"}, // Too few fields
		},
	}

	errors := checkCronSchedules(cfg)

	if len(errors) != 2 {
		t.Errorf("Expected 2 errors for invalid schedules, got %d: %v", len(errors), errors)
	}
}

func TestCheckCronSchedules_Empty(t *testing.T) {
	cfg := &config.Config{
		Servers: []config.Server{
			{Name: "server1", Schedule: ""}, // No schedule = manual only
		},
	}

	errors := checkCronSchedules(cfg)

	if len(errors) != 0 {
		t.Errorf("Expected no errors for empty schedule (manual-only), got: %v", errors)
	}
}
