package preflight

import (
	"fmt"
	"os/exec"

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
	// Placeholder - will be implemented in keeper-6wz.3
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
