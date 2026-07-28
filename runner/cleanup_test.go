package runner

import (
	"os"
	"path/filepath"
	"testing"
	"time"

	"keepr/config"
)

func writeFileWithAge(t *testing.T, path string, age time.Duration) {
	t.Helper()
	if err := os.MkdirAll(filepath.Dir(path), 0755); err != nil {
		t.Fatalf("Failed to create dir: %v", err)
	}
	if err := os.WriteFile(path, []byte("data"), 0644); err != nil {
		t.Fatalf("Failed to create file: %v", err)
	}
	mtime := time.Now().Add(-age)
	if err := os.Chtimes(path, mtime, mtime); err != nil {
		t.Fatalf("Failed to set file time: %v", err)
	}
}

func testServer(paths ...config.Path) config.Server {
	return config.Server{Name: "testserver", Paths: paths}
}

func TestCleanup(t *testing.T) {
	tmpDir := t.TempDir()
	server := testServer(config.Path{Remote: "/data", Local: "testserver/data", BackupDir: "testserver/data_old"})

	oldFile := filepath.Join(tmpDir, "testserver/data_old/old-file.txt")
	newFile := filepath.Join(tmpDir, "testserver/data_old/new-file.txt")
	writeFileWithAge(t, oldFile, 31*24*time.Hour)
	writeFileWithAge(t, newFile, 1*24*time.Hour)

	deleted, err := Cleanup(tmpDir, server, 30)
	if err != nil {
		t.Fatalf("Cleanup failed: %v", err)
	}

	if deleted != 1 {
		t.Errorf("Expected 1 file deleted, got %d", deleted)
	}
	if _, err := os.Stat(oldFile); !os.IsNotExist(err) {
		t.Error("Old file should be deleted")
	}
	if _, err := os.Stat(newFile); err != nil {
		t.Error("New file should remain")
	}
}

func TestCleanup_NestedFiles(t *testing.T) {
	// rsync --backup-dir mirrors the source tree, so backed up files
	// usually sit in subdirectories of the backup dir
	tmpDir := t.TempDir()
	server := testServer(config.Path{Remote: "/data", Local: "testserver/data", BackupDir: "testserver/data_old"})

	nested := filepath.Join(tmpDir, "testserver/data_old/someapp/config/settings.json")
	writeFileWithAge(t, nested, 31*24*time.Hour)

	deleted, err := Cleanup(tmpDir, server, 30)
	if err != nil {
		t.Fatalf("Cleanup failed: %v", err)
	}

	if deleted != 1 {
		t.Errorf("Expected 1 file deleted, got %d", deleted)
	}
	if _, err := os.Stat(nested); !os.IsNotExist(err) {
		t.Error("Nested old file should be deleted")
	}
}

func TestCleanup_IgnoresSyncedData(t *testing.T) {
	// Old files in the synced data tree must never be touched,
	// even in dirs whose name ends in _old
	tmpDir := t.TempDir()
	server := testServer(config.Path{Remote: "/data", Local: "testserver/data", BackupDir: "testserver/data_old"})

	dataFile := filepath.Join(tmpDir, "testserver/data/file.txt")
	trapFile := filepath.Join(tmpDir, "testserver/data/configs_old/file.txt")
	writeFileWithAge(t, dataFile, 31*24*time.Hour)
	writeFileWithAge(t, trapFile, 31*24*time.Hour)

	deleted, err := Cleanup(tmpDir, server, 30)
	if err != nil {
		t.Fatalf("Cleanup failed: %v", err)
	}

	if deleted != 0 {
		t.Errorf("Expected 0 files deleted, got %d", deleted)
	}
	if _, err := os.Stat(trapFile); err != nil {
		t.Error("File in synced data dir ending in _old should remain")
	}
}

func TestCleanup_MissingBackupDir(t *testing.T) {
	tmpDir := t.TempDir()
	server := testServer(config.Path{Remote: "/data", Local: "testserver/data", BackupDir: "testserver/data_old"})

	deleted, err := Cleanup(tmpDir, server, 30)
	if err != nil {
		t.Fatalf("Cleanup should ignore missing backup dirs: %v", err)
	}
	if deleted != 0 {
		t.Errorf("Expected 0 files deleted, got %d", deleted)
	}
}

func TestCleanupEmptyDirs(t *testing.T) {
	tmpDir := t.TempDir()
	server := testServer(config.Path{Remote: "/data", Local: "testserver/data", BackupDir: "testserver/data_old"})

	// Nested empty dirs: removing the child must also empty the parent
	nestedEmpty := filepath.Join(tmpDir, "testserver/data_old/someapp/logs")
	if err := os.MkdirAll(nestedEmpty, 0755); err != nil {
		t.Fatalf("Failed to create dirs: %v", err)
	}

	if err := CleanupEmptyDirs(tmpDir, server); err != nil {
		t.Fatalf("CleanupEmptyDirs failed: %v", err)
	}

	if _, err := os.Stat(filepath.Join(tmpDir, "testserver/data_old")); !os.IsNotExist(err) {
		t.Error("Empty backup dir tree should be removed")
	}
}

func TestCleanupEmptyDirs_KeepsNonEmpty(t *testing.T) {
	tmpDir := t.TempDir()
	server := testServer(config.Path{Remote: "/data", Local: "testserver/data", BackupDir: "testserver/data_old"})

	kept := filepath.Join(tmpDir, "testserver/data_old/someapp/file.txt")
	writeFileWithAge(t, kept, time.Hour)
	empty := filepath.Join(tmpDir, "testserver/data_old/emptyapp")
	if err := os.MkdirAll(empty, 0755); err != nil {
		t.Fatalf("Failed to create dir: %v", err)
	}

	if err := CleanupEmptyDirs(tmpDir, server); err != nil {
		t.Fatalf("CleanupEmptyDirs failed: %v", err)
	}

	if _, err := os.Stat(kept); err != nil {
		t.Error("Non-empty dirs should remain")
	}
	if _, err := os.Stat(empty); !os.IsNotExist(err) {
		t.Error("Empty sibling dir should be removed")
	}
}
