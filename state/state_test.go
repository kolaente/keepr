package state

import (
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
