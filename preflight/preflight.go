package preflight

import (
	"keepr/config"
)

// CheckResult represents the result of a single preflight check
type CheckResult struct {
	Name    string
	Passed  bool
	Message string
	Error   error
}

// RunAll executes all preflight checks and returns any errors found
func RunAll(cfg *config.Config) []error {
	var errors []error

	// Run checks and collect errors
	checks := []func(*config.Config) []error{
		checkRsyncBinary,
		checkBackupBasePath,
		checkSSHKeys,
		checkCronSchedules,
	}

	for _, check := range checks {
		if errs := check(cfg); len(errs) > 0 {
			errors = append(errors, errs...)
		}
	}

	return errors
}
