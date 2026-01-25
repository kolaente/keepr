package runner

import (
	"bufio"
	"context"
	"fmt"
	"os/exec"
	"time"
)

// LogFunc is a callback for streaming log output
type LogFunc func(line string)

// RunHook executes a shell command with timeout and streams output
func RunHook(ctx context.Context, command string, timeout time.Duration, logFn LogFunc) error {
	// Empty command is a no-op
	if command == "" {
		return nil
	}

	// Create context with timeout
	ctx, cancel := context.WithTimeout(ctx, timeout)
	defer cancel()

	cmd := exec.CommandContext(ctx, "sh", "-c", command)

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

	// Stream stdout with [hook] prefix
	go func() {
		scanner := bufio.NewScanner(stdout)
		for scanner.Scan() {
			if logFn != nil {
				logFn("[hook] " + scanner.Text())
			}
		}
	}()

	// Stream stderr with [hook] prefix
	go func() {
		scanner := bufio.NewScanner(stderr)
		for scanner.Scan() {
			if logFn != nil {
				logFn("[hook] " + scanner.Text())
			}
		}
	}()

	err = cmd.Wait()
	if ctx.Err() == context.DeadlineExceeded {
		return fmt.Errorf("hook timeout after %v", timeout)
	}
	if err != nil {
		return fmt.Errorf("hook failed: %w", err)
	}

	return nil
}
