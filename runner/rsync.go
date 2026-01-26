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
// The localPath parameter should be the resolved absolute path for the destination.
func BuildRsyncArgs(server config.Server, path config.Path, localPath string) []string {
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
	if path.BackupDir != "" {
		args = append(args, "-b", "--backup-dir="+path.BackupDir)
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

// RunRsync executes rsync for a server/path pair and streams output to logFn
func RunRsync(ctx context.Context, server config.Server, path config.Path, basePath string, logFn LogFunc) error {
	// Resolve local path: if not absolute, join with basePath
	localPath := path.Local
	if !filepath.IsAbs(localPath) {
		localPath = filepath.Join(basePath, localPath)
	}

	args := BuildRsyncArgs(server, path, localPath)

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
