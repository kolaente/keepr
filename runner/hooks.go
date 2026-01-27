package runner

import (
	"bufio"
	"context"
	"fmt"
	"os/exec"
	"sync"
	"time"
)

// LogFunc is a callback for streaming log output
type LogFunc func(line string)

// findShell returns the absolute path to a shell
func findShell() (string, error) {
	// First try PATH lookup (works in normal shell environments and tests)
	if sh, err := exec.LookPath("sh"); err == nil {
		return sh, nil
	}
	// Fall back to common absolute paths
	for _, path := range []string{"/bin/sh", "/usr/bin/sh"} {
		if _, err := exec.LookPath(path); err == nil {
			return path, nil
		}
	}
	return "", fmt.Errorf("no shell found in PATH or at common locations")
}

// RunHook executes a shell command with timeout and streams output
func RunHook(ctx context.Context, command string, timeout time.Duration, logFn LogFunc) error {
	// Empty command is a no-op
	if command == "" {
		return nil
	}

	shell, err := findShell()
	if err != nil {
		return err
	}

	// Create context with timeout
	ctx, cancel := context.WithTimeout(ctx, timeout)
	defer cancel()

	cmd := exec.CommandContext(ctx, shell, "-c", command)

	stdout, err := cmd.StdoutPipe()
	if err != nil {
		return fmt.Errorf("failed to get stdout pipe: %w", err)
	}

	stderr, err := cmd.StderrPipe()
	if err != nil {
		return fmt.Errorf("failed to get stderr pipe: %w", err)
	}

	if err := cmd.Start(); err != nil {
		return fmt.Errorf("failed to start hook: %w", err)
	}

	var wg sync.WaitGroup

	// Stream stdout with [hook] prefix
	wg.Add(1)
	go func() {
		defer wg.Done()
		scanner := bufio.NewScanner(stdout)
		for scanner.Scan() {
			if logFn != nil {
				logFn("[hook] " + scanner.Text())
			}
		}
	}()

	// Stream stderr with [hook] prefix
	wg.Add(1)
	go func() {
		defer wg.Done()
		scanner := bufio.NewScanner(stderr)
		for scanner.Scan() {
			if logFn != nil {
				logFn("[hook] " + scanner.Text())
			}
		}
	}()

	// Wait for output goroutines to finish before checking error
	wg.Wait()

	err = cmd.Wait()
	if ctx.Err() == context.DeadlineExceeded {
		return fmt.Errorf("hook timeout after %v", timeout)
	}
	if err != nil {
		return fmt.Errorf("hook failed: %w", err)
	}

	return nil
}
