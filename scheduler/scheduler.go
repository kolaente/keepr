package scheduler

import (
	"sync"

	"github.com/robfig/cron/v3"

	"keepr/config"
	"keepr/state"
)

type RunFunc func(config.Server)

type Scheduler struct {
	cron    *cron.Cron
	state   *state.Manager
	entries map[string]cron.EntryID
	running map[string]bool
	mu      sync.Mutex
	started bool
}

func New(sm *state.Manager) *Scheduler {
	return &Scheduler{
		cron:    cron.New(),
		state:   sm,
		entries: make(map[string]cron.EntryID),
		running: make(map[string]bool),
	}
}

func (s *Scheduler) Add(server config.Server, runFn RunFunc) error {
	if server.Schedule == "" {
		return nil
	}

	// Remove any existing entry for this server to prevent duplicates
	s.mu.Lock()
	if existingID, ok := s.entries[server.Name]; ok {
		s.cron.Remove(existingID)
		delete(s.entries, server.Name)
	}
	s.mu.Unlock()

	srv := server // capture for closure

	entryID, err := s.cron.AddFunc(server.Schedule, func() {
		s.mu.Lock()
		if s.running[srv.Name] {
			s.mu.Unlock()
			return
		}
		s.running[srv.Name] = true
		s.mu.Unlock()

		defer func() {
			s.mu.Lock()
			s.running[srv.Name] = false
			s.mu.Unlock()
			s.updateNextRun(srv.Name)
		}()

		runFn(srv)
	})
	if err != nil {
		return err
	}

	s.mu.Lock()
	s.entries[server.Name] = entryID
	s.mu.Unlock()

	// Set initial next run time
	s.updateNextRun(server.Name)

	return nil
}

func (s *Scheduler) updateNextRun(name string) {
	s.mu.Lock()
	entryID, ok := s.entries[name]
	s.mu.Unlock()

	if !ok {
		return
	}

	entry := s.cron.Entry(entryID)
	if entry.ID != 0 {
		s.state.SetNextRun(name, entry.Next)
	}
}

func (s *Scheduler) Start() {
	s.mu.Lock()
	defer s.mu.Unlock()

	if s.started {
		return
	}

	s.cron.Start()
	s.started = true

	// Update next run times for all entries
	for name := range s.entries {
		go s.updateNextRun(name)
	}
}

func (s *Scheduler) Stop() {
	s.mu.Lock()
	defer s.mu.Unlock()

	if !s.started {
		return
	}

	s.cron.Stop()
	s.started = false
}

func (s *Scheduler) IsRunning() bool {
	s.mu.Lock()
	defer s.mu.Unlock()
	return s.started
}
