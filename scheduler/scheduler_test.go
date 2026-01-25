package scheduler

import (
	"testing"
	"time"

	"keepr/config"
	"keepr/state"
)

func TestScheduler_NextRun(t *testing.T) {
	sm := state.New()
	sched := New(sm)

	server := config.Server{
		Name:     "server1",
		Schedule: "0 2 * * *", // Every day at 2am
	}

	called := false
	runFn := func(srv config.Server) {
		called = true
	}

	if err := sched.Add(server, runFn); err != nil {
		t.Fatalf("Add failed: %v", err)
	}

	sched.Start()
	defer sched.Stop()

	// Give scheduler time to update next run
	time.Sleep(100 * time.Millisecond)

	s := sm.Get("server1")
	if s.NextRun.IsZero() {
		t.Error("NextRun should not be zero after adding scheduled server")
	}

	// NextRun should be in the future
	if !s.NextRun.After(time.Now()) {
		t.Error("NextRun should be in the future")
	}

	// Suppress unused variable warning
	_ = called
}

func TestScheduler_NoSchedule(t *testing.T) {
	sm := state.New()
	sched := New(sm)

	server := config.Server{
		Name: "server1",
		// No schedule
	}

	runFn := func(srv config.Server) {}

	// Should not error when adding server without schedule
	if err := sched.Add(server, runFn); err != nil {
		t.Fatalf("Add failed: %v", err)
	}

	// NextRun should remain zero for unscheduled server
	s := sm.Get("server1")
	if !s.NextRun.IsZero() {
		t.Error("NextRun should be zero for unscheduled server")
	}
}

func TestScheduler_IsRunning(t *testing.T) {
	sm := state.New()
	sched := New(sm)

	if sched.IsRunning() {
		t.Error("Scheduler should not be running before Start")
	}

	sched.Start()
	if !sched.IsRunning() {
		t.Error("Scheduler should be running after Start")
	}

	sched.Stop()
	if sched.IsRunning() {
		t.Error("Scheduler should not be running after Stop")
	}
}
