package runner

import (
	"os"
	"path/filepath"
	"strings"
	"time"
)

// Cleanup removes old backup files from *_old directories
// Returns the number of files deleted
func Cleanup(basePath, serverName string, retentionDays int) (int, error) {
	serverPath := filepath.Join(basePath, serverName)
	cutoff := time.Now().Add(-time.Duration(retentionDays) * 24 * time.Hour)
	deleted := 0

	err := filepath.Walk(serverPath, func(path string, info os.FileInfo, err error) error {
		if err != nil {
			return err
		}

		// Skip directories
		if info.IsDir() {
			return nil
		}

		// Only process files in *_old directories
		dir := filepath.Dir(path)
		if !strings.HasSuffix(filepath.Base(dir), "_old") {
			return nil
		}

		// Delete files older than retention cutoff
		if info.ModTime().Before(cutoff) {
			if err := os.Remove(path); err != nil {
				return err
			}
			deleted++
		}

		return nil
	})

	return deleted, err
}

// CleanupEmptyDirs removes empty *_old directories
func CleanupEmptyDirs(basePath, serverName string) error {
	serverPath := filepath.Join(basePath, serverName)

	return filepath.Walk(serverPath, func(path string, info os.FileInfo, err error) error {
		if err != nil {
			// Directory might have been deleted already
			if os.IsNotExist(err) {
				return nil
			}
			return err
		}

		// Only process directories ending in _old
		if !info.IsDir() || !strings.HasSuffix(info.Name(), "_old") {
			return nil
		}

		// Check if directory is empty
		entries, err := os.ReadDir(path)
		if err != nil {
			return err
		}

		if len(entries) == 0 {
			if err := os.Remove(path); err != nil {
				return err
			}
		}

		return nil
	})
}
