package web

import (
	"embed"
	"encoding/json"
	"fmt"
	"html/template"
	"net/http"
	"strconv"
	"time"

	"keepr/config"
	"keepr/state"
)

// BackupRunner is the interface for triggering backups
type BackupRunner interface {
	Run(server config.Server) error
}

//go:embed templates/*.html
var templates embed.FS

type Server struct {
	state     *state.Manager
	config    *config.Config
	runner    BackupRunner
	mux       *http.ServeMux
	templates *template.Template
}

type DashboardData struct {
	Servers []ServerView
}

type ServerView struct {
	Name    string
	Status  state.Status
	LastRun time.Time
	NextRun time.Time
}

type LogsData struct {
	Name   string
	Status state.Status
	Logs   []string
}

func New(sm *state.Manager, cfg *config.Config, runner BackupRunner) *Server {
	s := &Server{
		state:  sm,
		config: cfg,
		runner: runner,
		mux:    http.NewServeMux(),
	}

	var err error
	s.templates, err = template.ParseFS(templates, "templates/*.html")
	if err != nil {
		panic(fmt.Sprintf("failed to parse templates: %v", err))
	}

	s.mux.HandleFunc("/", s.handleDashboard)
	s.mux.HandleFunc("/logs/", s.handleLogs)
	s.mux.HandleFunc("/api/logs/", s.handleLogsAPI)
	s.mux.HandleFunc("/api/run/", s.handleRunAPI)
	s.mux.HandleFunc("/api/status/", s.handleStatusAPI)
	s.mux.HandleFunc("/api/dashboard", s.handleDashboardAPI)

	return s
}

func (s *Server) ServeHTTP(w http.ResponseWriter, r *http.Request) {
	s.mux.ServeHTTP(w, r)
}

func (s *Server) handleDashboard(w http.ResponseWriter, r *http.Request) {
	if r.URL.Path != "/" {
		http.NotFound(w, r)
		return
	}

	servers := s.state.All()
	views := make([]ServerView, len(servers))
	for i, srv := range servers {
		views[i] = ServerView{
			Name:    srv.Name,
			Status:  srv.Status,
			LastRun: srv.LastRun,
			NextRun: srv.NextRun,
		}
	}

	data := DashboardData{Servers: views}
	if err := s.templates.ExecuteTemplate(w, "dashboard.html", data); err != nil {
		http.Error(w, err.Error(), http.StatusInternalServerError)
	}
}

func (s *Server) handleDashboardAPI(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodGet {
		http.Error(w, "method not allowed", http.StatusMethodNotAllowed)
		return
	}

	servers := s.state.All()
	views := make([]map[string]interface{}, len(servers))
	for i, srv := range servers {
		views[i] = map[string]interface{}{
			"name":     srv.Name,
			"status":   string(srv.Status),
			"last_run": formatTime(srv.LastRun),
			"next_run": formatTime(srv.NextRun),
		}
	}

	w.Header().Set("Content-Type", "application/json")
	_ = json.NewEncoder(w).Encode(map[string]interface{}{
		"servers": views,
	})
}

func formatTime(t time.Time) string {
	if t.IsZero() {
		return "-"
	}
	return t.Format("2006-01-02 15:04:05")
}

func (s *Server) handleLogs(w http.ResponseWriter, r *http.Request) {
	// Extract server name from /logs/{name}
	name := r.URL.Path[len("/logs/"):]
	if name == "" {
		http.NotFound(w, r)
		return
	}

	srv := s.state.Get(name)
	logs := s.state.GetLogs(name)

	data := LogsData{
		Name:   name,
		Status: srv.Status,
		Logs:   logs,
	}

	if err := s.templates.ExecuteTemplate(w, "logs.html", data); err != nil {
		http.Error(w, err.Error(), http.StatusInternalServerError)
	}
}

func (s *Server) handleLogsAPI(w http.ResponseWriter, r *http.Request) {
	// Extract server name from /api/logs/{name}/stream
	path := r.URL.Path[len("/api/logs/"):]
	name := path
	if idx := len(path) - len("/stream"); idx > 0 && path[idx:] == "/stream" {
		name = path[:idx]
	}

	if name == "" {
		http.NotFound(w, r)
		return
	}

	// Set SSE headers
	w.Header().Set("Content-Type", "text/event-stream")
	w.Header().Set("Cache-Control", "no-cache")
	w.Header().Set("Connection", "keep-alive")
	w.Header().Set("X-Accel-Buffering", "no") // Disable nginx buffering for SSE

	flusher, ok := w.(http.Flusher)
	if !ok {
		http.Error(w, "SSE not supported", http.StatusInternalServerError)
		return
	}

	// Send initial SSE comment to establish the connection
	_, _ = fmt.Fprintf(w, ": connected\n\n")
	flusher.Flush()

	// Parse 'from' query parameter to skip already-rendered logs
	lastCount := 0
	if fromStr := r.URL.Query().Get("from"); fromStr != "" {
		if from, err := strconv.Atoi(fromStr); err == nil && from >= 0 {
			lastCount = from
		}
	}

	for {
		select {
		case <-r.Context().Done():
			return
		default:
			logs := s.state.GetLogs(name)
			if len(logs) > lastCount {
				for _, line := range logs[lastCount:] {
					_, _ = fmt.Fprintf(w, "data: %s\n\n", line)
				}
				lastCount = len(logs)
				flusher.Flush()
			}
			time.Sleep(500 * time.Millisecond)
		}
	}
}

func (s *Server) handleRunAPI(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodPost {
		http.Error(w, "method not allowed", http.StatusMethodNotAllowed)
		return
	}

	// Check API secret
	if s.config.Web.APISecret == "" {
		http.Error(w, "api not configured", http.StatusServiceUnavailable)
		return
	}

	authHeader := r.Header.Get("Authorization")
	expectedAuth := "Bearer " + s.config.Web.APISecret
	if authHeader != expectedAuth {
		http.Error(w, "unauthorized", http.StatusUnauthorized)
		return
	}

	// Extract server name from /api/run/{name}
	name := r.URL.Path[len("/api/run/"):]
	if name == "" {
		http.Error(w, "server name required", http.StatusBadRequest)
		return
	}

	// Find server in config
	var server *config.Server
	for i := range s.config.Servers {
		if s.config.Servers[i].Name == name {
			server = &s.config.Servers[i]
			break
		}
	}
	if server == nil {
		http.Error(w, "server not found", http.StatusNotFound)
		return
	}

	// Run backup in background
	go func() {
		_ = s.runner.Run(*server)
	}()

	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(http.StatusAccepted)
	_ = json.NewEncoder(w).Encode(map[string]string{
		"status":  "started",
		"server":  name,
		"message": "backup started in background",
	})
}

func (s *Server) handleStatusAPI(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodGet {
		http.Error(w, "method not allowed", http.StatusMethodNotAllowed)
		return
	}

	// Check API secret
	if s.config.Web.APISecret == "" {
		http.Error(w, "api not configured", http.StatusServiceUnavailable)
		return
	}

	authHeader := r.Header.Get("Authorization")
	expectedAuth := "Bearer " + s.config.Web.APISecret
	if authHeader != expectedAuth {
		http.Error(w, "unauthorized", http.StatusUnauthorized)
		return
	}

	// Extract server name from /api/status/{name}
	name := r.URL.Path[len("/api/status/"):]
	if name == "" {
		http.Error(w, "server name required", http.StatusBadRequest)
		return
	}

	srv := s.state.Get(name)

	w.Header().Set("Content-Type", "application/json")
	_ = json.NewEncoder(w).Encode(map[string]interface{}{
		"name":       srv.Name,
		"status":     srv.Status,
		"started_at": srv.StartedAt,
		"last_run":   srv.LastRun,
	})
}
