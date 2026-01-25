package runner

import (
	"context"
	"fmt"
	"net/http"
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

	// Create log function
	logFn := func(line string) {
		r.state.AppendLog(name, line)
	}

	// Clear logs and set running state
	r.state.ClearLogs(name)
	r.state.SetRunning(name)
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
		deleted, err := Cleanup(r.config.BackupBasePath, name, server.RetentionDays)
		if err != nil {
			logFn(fmt.Sprintf("Cleanup warning: %v", err))
		} else if deleted > 0 {
			logFn(fmt.Sprintf("Cleaned up %d old files", deleted))
		}
		if err := CleanupEmptyDirs(r.config.BackupBasePath, name); err != nil {
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
