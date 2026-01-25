package runner

import (
	"os"
	"path/filepath"
	"testing"
	"time"
)

func TestCleanup(t *testing.T) {
	// Create temp directory structure
	tmpDir, err := os.MkdirTemp("", "cleanup-test")
	if err != nil {
		t.Fatalf("Failed to create temp dir: %v", err)
	}
	defer func() { _ = os.RemoveAll(tmpDir) }()

	// Create server directory with _old subdirectory
	serverDir := filepath.Join(tmpDir, "testserver")
	oldDir := filepath.Join(serverDir, "data_old")
	if err := os.MkdirAll(oldDir, 0755); err != nil {
		t.Fatalf("Failed to create old dir: %v", err)
	}

	// Create old file (31 days ago)
	oldFile := filepath.Join(oldDir, "old-file.txt")
	if err := os.WriteFile(oldFile, []byte("old"), 0644); err != nil {
		t.Fatalf("Failed to create old file: %v", err)
	}
	oldTime := time.Now().Add(-31 * 24 * time.Hour)
	if err := os.Chtimes(oldFile, oldTime, oldTime); err != nil {
		t.Fatalf("Failed to set old file time: %v", err)
	}

	// Create new file (1 day ago)
	newFile := filepath.Join(oldDir, "new-file.txt")
	if err := os.WriteFile(newFile, []byte("new"), 0644); err != nil {
		t.Fatalf("Failed to create new file: %v", err)
	}
	newTime := time.Now().Add(-1 * 24 * time.Hour)
	if err := os.Chtimes(newFile, newTime, newTime); err != nil {
		t.Fatalf("Failed to set new file time: %v", err)
	}

	// Run cleanup with 30 day retention
	deleted, err := Cleanup(tmpDir, "testserver", 30)
	if err != nil {
		t.Fatalf("Cleanup failed: %v", err)
	}

	if deleted != 1 {
		t.Errorf("Expected 1 file deleted, got %d", deleted)
	}

	// Verify old file is deleted
	if _, err := os.Stat(oldFile); !os.IsNotExist(err) {
		t.Error("Old file should be deleted")
	}

	// Verify new file remains
	if _, err := os.Stat(newFile); err != nil {
		t.Error("New file should remain")
	}
}

func TestCleanup_NoOldDirs(t *testing.T) {
	// Create temp directory without _old subdirectories
	tmpDir, err := os.MkdirTemp("", "cleanup-test")
	if err != nil {
		t.Fatalf("Failed to create temp dir: %v", err)
	}
	defer func() { _ = os.RemoveAll(tmpDir) }()

	serverDir := filepath.Join(tmpDir, "testserver")
	normalDir := filepath.Join(serverDir, "data")
	if err := os.MkdirAll(normalDir, 0755); err != nil {
		t.Fatalf("Failed to create dir: %v", err)
	}

	// Create a file in normal directory
	normalFile := filepath.Join(normalDir, "file.txt")
	if err := os.WriteFile(normalFile, []byte("data"), 0644); err != nil {
		t.Fatalf("Failed to create file: %v", err)
	}
	// Make it old
	oldTime := time.Now().Add(-31 * 24 * time.Hour)
	if err := os.Chtimes(normalFile, oldTime, oldTime); err != nil {
		t.Fatalf("Failed to set file time: %v", err)
	}

	// Run cleanup - should not delete files outside _old directories
	deleted, err := Cleanup(tmpDir, "testserver", 30)
	if err != nil {
		t.Fatalf("Cleanup failed: %v", err)
	}

	if deleted != 0 {
		t.Errorf("Expected 0 files deleted (not in _old dir), got %d", deleted)
	}

	// Verify file still exists
	if _, err := os.Stat(normalFile); err != nil {
		t.Error("File outside _old dir should remain")
	}
}

func TestCleanupEmptyDirs(t *testing.T) {
	// Create temp directory with empty _old subdirectory
	tmpDir, err := os.MkdirTemp("", "cleanup-test")
	if err != nil {
		t.Fatalf("Failed to create temp dir: %v", err)
	}
	defer func() { _ = os.RemoveAll(tmpDir) }()

	serverDir := filepath.Join(tmpDir, "testserver")
	emptyOldDir := filepath.Join(serverDir, "data_old")
	if err := os.MkdirAll(emptyOldDir, 0755); err != nil {
		t.Fatalf("Failed to create old dir: %v", err)
	}

	// Run cleanup for empty dirs
	err = CleanupEmptyDirs(tmpDir, "testserver")
	if err != nil {
		t.Fatalf("CleanupEmptyDirs failed: %v", err)
	}

	// Verify empty _old dir is removed
	if _, err := os.Stat(emptyOldDir); !os.IsNotExist(err) {
		t.Error("Empty _old directory should be removed")
	}
}
