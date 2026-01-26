package state

import (
	"strings"
	"testing"
	"time"
)

func TestStateManager(t *testing.T) {
	mgr := New()

	// Initial state should be idle
	s := mgr.Get("server1")
	if s.Status != StatusIdle {
		t.Errorf("Initial status = %v, want %v", s.Status, StatusIdle)
	}

	// SetRunning updates status and StartedAt
	mgr.SetRunning("server1")
	s = mgr.Get("server1")
	if s.Status != StatusRunning {
		t.Errorf("Status after SetRunning = %v, want %v", s.Status, StatusRunning)
	}
	if s.StartedAt.IsZero() {
		t.Error("StartedAt should not be zero after SetRunning")
	}

	// SetSuccess updates status and LastRun
	mgr.SetSuccess("server1")
	s = mgr.Get("server1")
	if s.Status != StatusSuccess {
		t.Errorf("Status after SetSuccess = %v, want %v", s.Status, StatusSuccess)
	}
	if s.LastRun.IsZero() {
		t.Error("LastRun should not be zero after SetSuccess")
	}
}

func TestStateManagerAllServers(t *testing.T) {
	mgr := New()

	mgr.SetRunning("server1")
	mgr.SetSuccess("server2")
	mgr.SetFailed("server3", StatusFailedBackup)

	all := mgr.All()
	if len(all) != 3 {
		t.Errorf("len(All()) = %d, want 3", len(all))
	}

	// Verify each server's state
	found := make(map[string]bool)
	for _, s := range all {
		found[s.Name] = true
	}
	for _, name := range []string{"server1", "server2", "server3"} {
		if !found[name] {
			t.Errorf("Server %s not found in All()", name)
		}
	}
}

func TestSetNextRun(t *testing.T) {
	mgr := New()
	nextRun := time.Now().Add(time.Hour)
	mgr.SetNextRun("server1", nextRun)

	s := mgr.Get("server1")
	if !s.NextRun.Equal(nextRun) {
		t.Errorf("NextRun = %v, want %v", s.NextRun, nextRun)
	}
}

func TestLogBuffer(t *testing.T) {
	mgr := New()

	// AppendLog adds lines
	mgr.AppendLog("server1", "line 1")
	mgr.AppendLog("server1", "line 2")
	mgr.AppendLog("server1", "line 3")

	// GetLogs returns all lines in order (with timestamps)
	logs := mgr.GetLogs("server1")
	if len(logs) != 3 {
		t.Fatalf("len(logs) = %d, want 3", len(logs))
	}
	if !strings.Contains(logs[0], "line 1") || !strings.Contains(logs[1], "line 2") || !strings.Contains(logs[2], "line 3") {
		t.Errorf("logs = %v, want lines containing [line 1, line 2, line 3]", logs)
	}

	// ClearLogs removes all lines
	mgr.ClearLogs("server1")
	logs = mgr.GetLogs("server1")
	if len(logs) != 0 {
		t.Errorf("len(logs) after clear = %d, want 0", len(logs))
	}
}

func TestLogBufferMaxSize(t *testing.T) {
	mgr := NewWithLogSize(3)

	// Add 5 lines to a buffer of size 3
	mgr.AppendLog("server1", "line 1")
	mgr.AppendLog("server1", "line 2")
	mgr.AppendLog("server1", "line 3")
	mgr.AppendLog("server1", "line 4")
	mgr.AppendLog("server1", "line 5")

	// Oldest lines should be dropped
	logs := mgr.GetLogs("server1")
	if len(logs) != 3 {
		t.Fatalf("len(logs) = %d, want 3", len(logs))
	}
	// Should have lines 3, 4, 5 (oldest dropped, with timestamps)
	if !strings.Contains(logs[0], "line 3") || !strings.Contains(logs[1], "line 4") || !strings.Contains(logs[2], "line 5") {
		t.Errorf("logs = %v, want lines containing [line 3, line 4, line 5]", logs)
	}
}
