package runner

import (
	"bufio"
	"context"
	"fmt"
	"os/exec"
	"path/filepath"
	"strings"

	"keepr/config"
)

// BuildRsyncArgs builds the rsync command arguments for a given server and path.
// localPath and backupDir must be resolved absolute paths (backupDir may be empty).
func BuildRsyncArgs(server config.Server, path config.Path, localPath, backupDir string) []string {
	args := []string{"-avz", "--no-group"}

	// For remote servers, add SSH options
	if server.Type == "remote" {
		sshCmd := fmt.Sprintf("ssh -p %d -o StrictHostKeyChecking=accept-new -o BatchMode=yes", server.Port)
		if server.Key != "" {
			sshCmd += " -i " + server.Key
		}
		args = append(args, "-e", sshCmd)
		args = append(args, "--delete")
		if server.RsyncPath != "" {
			args = append(args, "--rsync-path="+server.RsyncPath)
		}
	}

	// Handle backup directory option
	if backupDir != "" {
		args = append(args, "-b", "--backup-dir="+backupDir)
		// If backup dir lies inside the destination, --delete would remove it
		// (it doesn't exist on the source) and it would back up into itself.
		// Exclude it, anchored to the transfer root so nothing else matches.
		if rel, err := filepath.Rel(localPath, backupDir); err == nil && rel != "." && !strings.HasPrefix(rel, "..") {
			args = append(args, "--exclude=/"+rel)
		}
	}

	// Build source path
	var source string
	if server.Type == "remote" {
		source = fmt.Sprintf("%s@%s:%s", server.User, server.Host, path.Remote)
	} else {
		source = path.Remote
	}

	args = append(args, source, localPath)

	return args
}

// ResolvePath joins a config path with basePath unless it is already absolute
func ResolvePath(basePath, p string) string {
	if p == "" || filepath.IsAbs(p) {
		return p
	}
	return filepath.Join(basePath, p)
}

// RunRsync executes rsync for a server/path pair and streams output to logFn
func RunRsync(ctx context.Context, server config.Server, path config.Path, basePath string, logFn LogFunc) error {
	localPath := ResolvePath(basePath, path.Local)
	// Resolve backup dir too: rsync treats a relative --backup-dir as relative
	// to the *destination*, which nests it inside the synced tree
	backupDir := ResolvePath(basePath, path.BackupDir)

	args := BuildRsyncArgs(server, path, localPath, backupDir)

	// Ensure destination directory exists
	destDir := filepath.Dir(localPath)
	if err := exec.CommandContext(ctx, "mkdir", "-p", destDir).Run(); err != nil {
		return fmt.Errorf("failed to create destination directory: %w", err)
	}

	cmd := exec.CommandContext(ctx, "rsync", args...)

	if logFn != nil {
		logFn(fmt.Sprintf("Running: rsync %s", strings.Join(args, " ")))
	}

	stdout, err := cmd.StdoutPipe()
	if err != nil {
		return fmt.Errorf("failed to get stdout pipe: %w", err)
	}

	stderr, err := cmd.StderrPipe()
	if err != nil {
		return fmt.Errorf("failed to get stderr pipe: %w", err)
	}

	if err := cmd.Start(); err != nil {
		return fmt.Errorf("failed to start rsync: %w", err)
	}

	// Stream stdout
	go func() {
		scanner := bufio.NewScanner(stdout)
		for scanner.Scan() {
			if logFn != nil {
				logFn(scanner.Text())
			}
		}
	}()

	// Stream stderr
	go func() {
		scanner := bufio.NewScanner(stderr)
		for scanner.Scan() {
			if logFn != nil {
				logFn("[stderr] " + scanner.Text())
			}
		}
	}()

	err = cmd.Wait()
	exitCode := 0
	if exitErr, ok := err.(*exec.ExitError); ok {
		exitCode = exitErr.ExitCode()
	}

	return checkRsyncError(err, exitCode)
}

// checkRsyncError checks if rsync exit code indicates success
// Exit code 24 means "some files vanished" which is common and acceptable
func checkRsyncError(err error, exitCode int) error {
	if exitCode == 0 || exitCode == 24 {
		return nil
	}
	return err
}
