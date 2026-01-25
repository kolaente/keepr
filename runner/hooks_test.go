package runner

import (
	"context"
	"strings"
	"testing"
	"time"
)

func TestRunHook_Success(t *testing.T) {
	var logs []string
	logFn := func(line string) {
		logs = append(logs, line)
	}

	err := RunHook(context.Background(), "echo hello", 5*time.Second, logFn)
	if err != nil {
		t.Errorf("RunHook failed: %v", err)
	}

	found := false
	for _, log := range logs {
		if strings.Contains(log, "hello") {
			found = true
			break
		}
	}
	if !found {
		t.Errorf("Expected 'hello' in logs, got: %v", logs)
	}
}

func TestRunHook_Failure(t *testing.T) {
	err := RunHook(context.Background(), "exit 1", 5*time.Second, nil)
	if err == nil {
		t.Error("RunHook should fail for 'exit 1'")
	}
}

func TestRunHook_Empty(t *testing.T) {
	err := RunHook(context.Background(), "", 5*time.Second, nil)
	if err != nil {
		t.Errorf("RunHook should succeed for empty command, got: %v", err)
	}
}

func TestRunHook_Timeout(t *testing.T) {
	err := RunHook(context.Background(), "sleep 10", 100*time.Millisecond, nil)
	if err == nil {
		t.Error("RunHook should fail with timeout")
	}
	if !strings.Contains(err.Error(), "timeout") && !strings.Contains(err.Error(), "killed") {
		t.Errorf("Expected timeout error, got: %v", err)
	}
}
