package runner

import (
	"os"
	"path/filepath"
	"sort"
	"time"

	"keepr/config"
)

// Cleanup removes files older than the retention cutoff from the server's
// configured backup dirs. Only configured backup_dir roots are touched, so
// synced data dirs that happen to end in "_old" are never affected.
// Returns the number of files deleted.
func Cleanup(basePath string, server config.Server, retentionDays int) (int, error) {
	cutoff := time.Now().Add(-time.Duration(retentionDays) * 24 * time.Hour)
	deleted := 0

	for _, root := range backupDirs(basePath, server) {
		err := filepath.Walk(root, func(path string, info os.FileInfo, err error) error {
			if err != nil {
				if os.IsNotExist(err) {
					return nil
				}
				return err
			}
			if info.IsDir() {
				return nil
			}
			if info.ModTime().Before(cutoff) {
				if err := os.Remove(path); err != nil {
					return err
				}
				deleted++
			}
			return nil
		})
		if err != nil {
			return deleted, err
		}
	}

	return deleted, nil
}

// CleanupEmptyDirs removes empty directories inside the server's backup dirs,
// including the backup dir itself if it ends up empty
func CleanupEmptyDirs(basePath string, server config.Server) error {
	for _, root := range backupDirs(basePath, server) {
		var dirs []string
		err := filepath.Walk(root, func(path string, info os.FileInfo, err error) error {
			if err != nil {
				if os.IsNotExist(err) {
					return nil
				}
				return err
			}
			if info.IsDir() {
				dirs = append(dirs, path)
			}
			return nil
		})
		if err != nil {
			return err
		}

		// Deepest first so parents emptied by child removal get removed too
		sort.Slice(dirs, func(i, j int) bool { return len(dirs[i]) > len(dirs[j]) })
		for _, dir := range dirs {
			entries, err := os.ReadDir(dir)
			if err != nil {
				if os.IsNotExist(err) {
					continue
				}
				return err
			}
			if len(entries) == 0 {
				if err := os.Remove(dir); err != nil {
					return err
				}
			}
		}
	}

	return nil
}

// backupDirs returns the resolved backup dirs configured for a server
func backupDirs(basePath string, server config.Server) []string {
	var dirs []string
	for _, p := range server.Paths {
		if p.BackupDir != "" {
			dirs = append(dirs, ResolvePath(basePath, p.BackupDir))
		}
	}
	return dirs
}
