package runner

import (
	"bufio"
	"context"
	"fmt"
	"os/exec"
	"path/filepath"

	"keepr/config"
)

// BuildRsyncArgs builds the rsync command arguments for a given server and path.
func BuildRsyncArgs(server config.Server, path config.Path) []string {
	args := []string{"-avz"}

	// For remote servers, add SSH options
	if server.Type == "remote" {
		sshCmd := fmt.Sprintf("ssh -p %d", server.Port)
		if server.Key != "" {
			sshCmd += " -i " + server.Key
		}
		args = append(args, "-e", sshCmd)
		args = append(args, "--delete")
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

	args = append(args, source, path.Local)

	return args
}

// RunRsync executes rsync for a server/path pair and streams output to logFn
func RunRsync(ctx context.Context, server config.Server, path config.Path, basePath string, logFn LogFunc) error {
	args := BuildRsyncArgs(server, path)

	// Ensure destination directory exists
	destDir := filepath.Dir(path.Local)
	if err := exec.CommandContext(ctx, "mkdir", "-p", destDir).Run(); err != nil {
		return fmt.Errorf("failed to create destination directory: %w", err)
	}

	cmd := exec.CommandContext(ctx, "rsync", args...)

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
