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
