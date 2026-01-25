package runner

import (
	"testing"

	"keepr/config"
	"keepr/state"
)

func TestRunner_UpdatesState(t *testing.T) {
	cfg := &config.Config{
		BackupBasePath: "/backups",
	}
	sm := state.New()

	r := New(cfg, sm)

	if r.config != cfg {
		t.Error("Runner should store config")
	}
	if r.state != sm {
		t.Error("Runner should store state")
	}
}

func TestRunner_Run_SetsRunningState(t *testing.T) {
	cfg := &config.Config{
		BackupBasePath: "/tmp/backups-test",
	}
	sm := state.New()
	r := New(cfg, sm)

	server := config.Server{
		Name: "testserver",
		Type: "local",
		Paths: []config.Path{
			{Remote: "/nonexistent", Local: "/tmp/backups-test/testserver/data"},
		},
	}

	// Run will fail because paths don't exist, but state should be updated
	_ = r.Run(server)

	s := sm.Get("testserver")
	// After run, status should be either success or one of the failed states
	if s.Status == state.StatusIdle || s.Status == state.StatusRunning {
		t.Errorf("Status should be updated after Run, got %v", s.Status)
	}
}
