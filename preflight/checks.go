package preflight

import (
	"fmt"
	"os"
	"os/exec"
	"path/filepath"

	"keepr/config"
)

// checkRsyncBinary verifies that rsync is installed and executable
func checkRsyncBinary(cfg *config.Config) []error {
	_, err := exec.LookPath("rsync")
	if err != nil {
		return []error{
			fmt.Errorf("rsync binary not found in PATH: %w (install rsync to use keepr)", err),
		}
	}
	return nil
}

// checkBackupBasePath verifies the backup base path exists and is writable
func checkBackupBasePath(cfg *config.Config) []error {
	path := cfg.BackupBasePath

	// Check if path exists
	info, err := os.Stat(path)
	if os.IsNotExist(err) {
		return []error{
			fmt.Errorf("backup_base_path %q does not exist: create it with 'mkdir -p %s'", path, path),
		}
	}
	if err != nil {
		return []error{
			fmt.Errorf("backup_base_path %q: %w", path, err),
		}
	}

	// Check if it's a directory
	if !info.IsDir() {
		return []error{
			fmt.Errorf("backup_base_path %q is not a directory", path),
		}
	}

	// Check if writable by attempting to create a temp file
	testFile := filepath.Join(path, ".keepr-preflight-test")
	f, err := os.Create(testFile)
	if err != nil {
		return []error{
			fmt.Errorf("backup_base_path %q is not writable: %w", path, err),
		}
	}
	f.Close()
	os.Remove(testFile)

	return nil
}

// checkSSHKeys verifies SSH key files exist and have correct permissions
func checkSSHKeys(cfg *config.Config) []error {
	// Placeholder - will be implemented in keeper-6wz.4
	return nil
}

// checkCronSchedules validates all server cron schedules
func checkCronSchedules(cfg *config.Config) []error {
	// Placeholder - will be implemented in keeper-6wz.5
	return nil
}
