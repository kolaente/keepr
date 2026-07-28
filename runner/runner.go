package runner

import (
	"context"
	"fmt"
	"net/http"
	"os"
	"os/user"
	"time"

	"keepr/config"
	"keepr/state"
)

const (
	hookTimeout = 5 * time.Minute
)

// Runner orchestrates backup operations for a server
type Runner struct {
	config *config.Config
	state  *state.Manager
}

// New creates a new backup runner
func New(cfg *config.Config, sm *state.Manager) *Runner {
	return &Runner{
		config: cfg,
		state:  sm,
	}
}

// Run executes a backup for the given server
func (r *Runner) Run(server config.Server) error {
	ctx := context.Background()
	name := server.Name

	// Try to acquire running state atomically
	if !r.state.TrySetRunning(name) {
		return fmt.Errorf("backup already running for %s", name)
	}

	// Clear logs after acquiring lock to avoid clearing logs from the running backup
	r.state.ClearLogs(name)

	// Create log function
	logFn := func(line string) {
		r.state.AppendLog(name, line)
	}

	logFn(fmt.Sprintf("Starting backup for %s", name))

	// Run pre-hook
	if server.PreHook != "" {
		logFn(fmt.Sprintf("Running pre-hook: %s", server.PreHook))
		if err := RunHook(ctx, server.PreHook, hookTimeout, logFn); err != nil {
			logFn(fmt.Sprintf("Pre-hook failed: %v", err))
			r.state.SetFailed(name, state.StatusFailedPreHook)
			return err
		}
		logFn("Pre-hook completed successfully")
	}

	// Check permissions for local servers before starting
	if server.Type == "local" {
		if err := checkLocalPermissions(server, logFn); err != nil {
			r.state.SetFailed(name, state.StatusFailedBackup)
			return err
		}
	}

	// Run rsync for each path
	for _, path := range server.Paths {
		logFn(fmt.Sprintf("Syncing %s -> %s", path.Remote, path.Local))
		if err := RunRsync(ctx, server, path, r.config.BackupBasePath, logFn); err != nil {
			logFn(fmt.Sprintf("Rsync failed: %v", err))
			// Still run post-hook even if rsync fails (ignore its error)
			_ = r.runPostHook(ctx, server, logFn)
			r.state.SetFailed(name, state.StatusFailedBackup)
			return err
		}
		logFn(fmt.Sprintf("Sync completed for %s", path.Local))
	}

	// Run post-hook (always runs, even after failure handled above)
	if err := r.runPostHook(ctx, server, logFn); err != nil {
		r.state.SetFailed(name, state.StatusFailedPostHook)
		return err
	}

	// Run cleanup if retention is configured
	if server.RetentionDays > 0 {
		logFn(fmt.Sprintf("Running cleanup (retention: %d days)", server.RetentionDays))
		deleted, err := Cleanup(r.config.BackupBasePath, server, server.RetentionDays)
		if err != nil {
			logFn(fmt.Sprintf("Cleanup warning: %v", err))
		} else if deleted > 0 {
			logFn(fmt.Sprintf("Cleaned up %d old files", deleted))
		}
		if err := CleanupEmptyDirs(r.config.BackupBasePath, server); err != nil {
			logFn(fmt.Sprintf("Cleanup empty dirs warning: %v", err))
		}
	}

	// Call heartbeat URL on success
	if server.Heartbeat != "" {
		logFn(fmt.Sprintf("Calling heartbeat: %s", server.Heartbeat))
		if err := r.callHeartbeat(server.Heartbeat); err != nil {
			logFn(fmt.Sprintf("Heartbeat warning: %v", err))
		}
	}

	// Set success state
	r.state.SetSuccess(name)
	logFn("Backup completed successfully")

	return nil
}

func (r *Runner) runPostHook(ctx context.Context, server config.Server, logFn LogFunc) error {
	if server.PostHook == "" {
		return nil
	}
	logFn(fmt.Sprintf("Running post-hook: %s", server.PostHook))
	if err := RunHook(ctx, server.PostHook, hookTimeout, logFn); err != nil {
		logFn(fmt.Sprintf("Post-hook failed: %v", err))
		return err
	}
	logFn("Post-hook completed successfully")
	return nil
}

func (r *Runner) callHeartbeat(url string) error {
	resp, err := http.Get(url)
	if err != nil {
		return err
	}
	defer func() { _ = resp.Body.Close() }()
	if resp.StatusCode >= 400 {
		return fmt.Errorf("heartbeat returned status %d", resp.StatusCode)
	}
	return nil
}

// checkLocalPermissions verifies that all local paths are readable before starting backup
func checkLocalPermissions(server config.Server, logFn LogFunc) error {
	var unreadable []string

	for _, path := range server.Paths {
		// Check if path is readable by trying to read directory contents
		// os.Open alone doesn't check read permissions for directory contents
		f, err := os.Open(path.Remote)
		if err != nil {
			if os.IsPermission(err) {
				unreadable = append(unreadable, path.Remote)
			}
			// Ignore other errors (like not exists) - rsync will handle those
			continue
		}

		// For directories, try to read entries to verify read permission
		info, err := f.Stat()
		if err == nil && info.IsDir() {
			_, err = f.Readdirnames(1)
			if err != nil && os.IsPermission(err) {
				unreadable = append(unreadable, path.Remote)
			}
		}
		_ = f.Close()
	}

	if len(unreadable) == 0 {
		return nil
	}

	// Get current username for ACL command
	username := "youruser"
	if u, err := user.Current(); err == nil {
		username = u.Username
	}

	logFn("Permission denied for the following paths:")
	for _, p := range unreadable {
		logFn(fmt.Sprintf("  - %s", p))
	}
	logFn("")
	logFn("To fix, grant read access with ACLs:")
	for _, p := range unreadable {
		logFn(fmt.Sprintf("  sudo setfacl -R -m u:%s:rX %s", username, p))
	}

	return fmt.Errorf("permission denied: %d path(s) not readable", len(unreadable))
}
