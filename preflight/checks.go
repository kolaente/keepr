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
	var errors []error

	for _, server := range cfg.Servers {
		if server.Type != "remote" {
			continue
		}

		// If no key specified, SSH will use defaults or agent - not an error
		if server.Key == "" {
			continue
		}

		// Check key file exists
		info, err := os.Stat(server.Key)
		if os.IsNotExist(err) {
			errors = append(errors, fmt.Errorf(
				"server %q: SSH key %q does not exist",
				server.Name, server.Key,
			))
			continue
		}
		if err != nil {
			errors = append(errors, fmt.Errorf(
				"server %q: SSH key %q: %w",
				server.Name, server.Key, err,
			))
			continue
		}

		// Check permissions (should be 0600 or 0400)
		mode := info.Mode().Perm()
		if mode&0077 != 0 {
			errors = append(errors, fmt.Errorf(
				"server %q: SSH key %q has insecure permissions %04o (should be 0600): run 'chmod 600 %s'",
				server.Name, server.Key, mode, server.Key,
			))
		}
	}

	return errors
}

// checkCronSchedules validates all server cron schedules
func checkCronSchedules(cfg *config.Config) []error {
	// Placeholder - will be implemented in keeper-6wz.5
	return nil
}
