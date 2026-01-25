package web

import (
	"embed"
	"fmt"
	"html/template"
	"net/http"
	"time"

	"keepr/config"
	"keepr/state"
)

//go:embed templates/*.html
var templates embed.FS

type Server struct {
	state     *state.Manager
	config    *config.Config
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

func New(sm *state.Manager, cfg *config.Config) *Server {
	s := &Server{
		state:  sm,
		config: cfg,
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

	flusher, ok := w.(http.Flusher)
	if !ok {
		http.Error(w, "SSE not supported", http.StatusInternalServerError)
		return
	}

	// Track last log count to send only new logs
	lastCount := 0

	for {
		select {
		case <-r.Context().Done():
			return
		default:
			logs := s.state.GetLogs(name)
			if len(logs) > lastCount {
				for _, line := range logs[lastCount:] {
					fmt.Fprintf(w, "data: %s\n\n", line)
				}
				lastCount = len(logs)
				flusher.Flush()
			}
			time.Sleep(500 * time.Millisecond)
		}
	}
}
