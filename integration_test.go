//go:build integration

package main

import (
	"os"
	"path/filepath"
	"testing"

	"keepr/config"
	"keepr/runner"
	"keepr/state"
)

func TestIntegration_LocalBackup(t *testing.T) {
	// Create temp directories
	tmpDir, err := os.MkdirTemp("", "keepr-integration")
	if err != nil {
		t.Fatalf("Failed to create temp dir: %v", err)
	}
	defer os.RemoveAll(tmpDir)

	sourceDir := filepath.Join(tmpDir, "source")
	destDir := filepath.Join(tmpDir, "backups")
	basePath := filepath.Join(tmpDir, "base")

	// Create source directory with test file
	if err := os.MkdirAll(sourceDir, 0755); err != nil {
		t.Fatalf("Failed to create source dir: %v", err)
	}
	testFile := filepath.Join(sourceDir, "test.txt")
	if err := os.WriteFile(testFile, []byte("hello world"), 0644); err != nil {
		t.Fatalf("Failed to create test file: %v", err)
	}

	// Create config
	cfg := &config.Config{
		BackupBasePath: basePath,
	}

	// Create server config
	// Note: trailing slash on Remote means rsync copies contents, not the directory itself
	server := config.Server{
		Name: "testserver",
		Type: "local",
		Paths: []config.Path{
			{
				Remote: sourceDir + "/",
				Local:  destDir,
			},
		},
	}

	// Create state manager and runner
	sm := state.New()
	r := runner.New(cfg, sm)

	// Run backup
	err = r.Run(server)
	if err != nil {
		t.Fatalf("Backup failed: %v", err)
	}

	// Verify file was copied
	destFile := filepath.Join(destDir, "test.txt")
	content, err := os.ReadFile(destFile)
	if err != nil {
		t.Fatalf("Failed to read dest file: %v", err)
	}
	if string(content) != "hello world" {
		t.Errorf("File content = %q, want 'hello world'", string(content))
	}

	// Verify state is success
	s := sm.Get("testserver")
	if s.Status != state.StatusSuccess {
		t.Errorf("Status = %v, want %v", s.Status, state.StatusSuccess)
	}

	// Verify logs were created
	logs := sm.GetLogs("testserver")
	if len(logs) == 0 {
		t.Error("Expected logs to be created")
	}
}

func TestIntegration_LocalBackupWithHooks(t *testing.T) {
	// Create temp directories
	tmpDir, err := os.MkdirTemp("", "keepr-integration")
	if err != nil {
		t.Fatalf("Failed to create temp dir: %v", err)
	}
	defer os.RemoveAll(tmpDir)

	sourceDir := filepath.Join(tmpDir, "source")
	destDir := filepath.Join(tmpDir, "backups")
	basePath := filepath.Join(tmpDir, "base")
	hookFile := filepath.Join(tmpDir, "hook_ran")

	// Create source directory with test file
	if err := os.MkdirAll(sourceDir, 0755); err != nil {
		t.Fatalf("Failed to create source dir: %v", err)
	}
	testFile := filepath.Join(sourceDir, "data.txt")
	if err := os.WriteFile(testFile, []byte("backup data"), 0644); err != nil {
		t.Fatalf("Failed to create test file: %v", err)
	}

	// Create config
	cfg := &config.Config{
		BackupBasePath: basePath,
	}

	// Create server config with hooks
	// Note: trailing slash on Remote means rsync copies contents, not the directory itself
	server := config.Server{
		Name:     "hookserver",
		Type:     "local",
		PreHook:  "echo 'pre-hook' > " + hookFile,
		PostHook: "echo 'post-hook' >> " + hookFile,
		Paths: []config.Path{
			{
				Remote: sourceDir + "/",
				Local:  destDir,
			},
		},
	}

	// Create state manager and runner
	sm := state.New()
	r := runner.New(cfg, sm)

	// Run backup
	err = r.Run(server)
	if err != nil {
		t.Fatalf("Backup failed: %v", err)
	}

	// Verify hooks ran
	hookContent, err := os.ReadFile(hookFile)
	if err != nil {
		t.Fatalf("Failed to read hook file: %v", err)
	}
	content := string(hookContent)
	if content != "pre-hook\npost-hook\n" {
		t.Errorf("Hook file content = %q, want 'pre-hook\\npost-hook\\n'", content)
	}

	// Verify state is success
	s := sm.Get("hookserver")
	if s.Status != state.StatusSuccess {
		t.Errorf("Status = %v, want %v", s.Status, state.StatusSuccess)
	}
}
