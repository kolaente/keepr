package state

import (
	"sort"
	"sync"
	"time"
)

type Status string

const (
	StatusIdle           Status = "idle"
	StatusRunning        Status = "running"
	StatusSuccess        Status = "success"
	StatusFailedPreHook  Status = "failed_pre_hook"
	StatusFailedBackup   Status = "failed_backup"
	StatusFailedPostHook Status = "failed_post_hook"
)

const DefaultLogSize = 1000

type ServerState struct {
	Name      string
	Status    Status
	StartedAt time.Time
	LastRun   time.Time
	NextRun   time.Time
}

type Manager struct {
	mu      sync.RWMutex
	states  map[string]*ServerState
	logs    map[string][]string
	logSize int
}

func New() *Manager {
	return NewWithLogSize(DefaultLogSize)
}

func NewWithLogSize(size int) *Manager {
	return &Manager{
		states:  make(map[string]*ServerState),
		logs:    make(map[string][]string),
		logSize: size,
	}
}

func (m *Manager) get(name string) *ServerState {
	if s, ok := m.states[name]; ok {
		return s
	}
	s := &ServerState{Name: name, Status: StatusIdle}
	m.states[name] = s
	return s
}

func (m *Manager) Get(name string) ServerState {
	m.mu.RLock()
	defer m.mu.RUnlock()
	return *m.get(name)
}

func (m *Manager) All() []ServerState {
	m.mu.RLock()
	defer m.mu.RUnlock()
	result := make([]ServerState, 0, len(m.states))
	for _, s := range m.states {
		result = append(result, *s)
	}
	sort.Slice(result, func(i, j int) bool {
		return result[i].Name < result[j].Name
	})
	return result
}

func (m *Manager) SetRunning(name string) {
	m.mu.Lock()
	defer m.mu.Unlock()
	s := m.get(name)
	s.Status = StatusRunning
	s.StartedAt = time.Now()
}

func (m *Manager) SetSuccess(name string) {
	m.mu.Lock()
	defer m.mu.Unlock()
	s := m.get(name)
	s.Status = StatusSuccess
	s.LastRun = time.Now()
}

func (m *Manager) SetFailed(name string, status Status) {
	m.mu.Lock()
	defer m.mu.Unlock()
	s := m.get(name)
	s.Status = status
	s.LastRun = time.Now()
}

func (m *Manager) SetNextRun(name string, t time.Time) {
	m.mu.Lock()
	defer m.mu.Unlock()
	s := m.get(name)
	s.NextRun = t
}

func (m *Manager) AppendLog(name, line string) {
	m.mu.Lock()
	defer m.mu.Unlock()

	m.logs[name] = append(m.logs[name], line)

	// Ring buffer: drop oldest if exceeding size
	if len(m.logs[name]) > m.logSize {
		m.logs[name] = m.logs[name][len(m.logs[name])-m.logSize:]
	}
}

func (m *Manager) GetLogs(name string) []string {
	m.mu.RLock()
	defer m.mu.RUnlock()

	logs := m.logs[name]
	if logs == nil {
		return []string{}
	}
	// Return a copy to avoid race conditions
	result := make([]string, len(logs))
	copy(result, logs)
	return result
}

func (m *Manager) ClearLogs(name string) {
	m.mu.Lock()
	defer m.mu.Unlock()
	delete(m.logs, name)
}
