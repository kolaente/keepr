package web

import (
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
	"time"

	"keepr/config"
	"keepr/state"
)

func TestDashboard(t *testing.T) {
	sm := state.New()
	sm.SetSuccess("server1")

	srv := New(sm, nil, nil)

	req := httptest.NewRequest("GET", "/", nil)
	w := httptest.NewRecorder()

	srv.ServeHTTP(w, req)

	if w.Code != http.StatusOK {
		t.Errorf("Status = %d, want %d", w.Code, http.StatusOK)
	}

	body := w.Body.String()
	if !strings.Contains(body, "server1") {
		t.Error("Response should contain 'server1'")
	}
}

func TestLogs(t *testing.T) {
	sm := state.New()
	sm.SetSuccess("server1")
	sm.AppendLog("server1", "test log line")

	srv := New(sm, nil, nil)

	req := httptest.NewRequest("GET", "/logs/server1", nil)
	w := httptest.NewRecorder()

	srv.ServeHTTP(w, req)

	if w.Code != http.StatusOK {
		t.Errorf("Status = %d, want %d", w.Code, http.StatusOK)
	}

	body := w.Body.String()
	if !strings.Contains(body, "server1") {
		t.Error("Response should contain 'server1'")
	}
	if !strings.Contains(body, "test log line") {
		t.Error("Response should contain log line")
	}
}

func TestDashboardAPI(t *testing.T) {
	sm := state.New()
	sm.SetSuccess("server1")
	sm.SetNextRun("server1", time.Now().Add(time.Hour))

	srv := New(sm, nil, nil)

	req := httptest.NewRequest("GET", "/api/dashboard", nil)
	w := httptest.NewRecorder()

	srv.ServeHTTP(w, req)

	if w.Code != http.StatusOK {
		t.Errorf("Status = %d, want %d", w.Code, http.StatusOK)
	}

	contentType := w.Header().Get("Content-Type")
	if contentType != "application/json" {
		t.Errorf("Content-Type = %q, want %q", contentType, "application/json")
	}

	var response struct {
		Servers []struct {
			Name    string `json:"name"`
			Status  string `json:"status"`
			LastRun string `json:"last_run"`
			NextRun string `json:"next_run"`
		} `json:"servers"`
	}

	if err := json.NewDecoder(w.Body).Decode(&response); err != nil {
		t.Fatalf("Failed to decode JSON: %v", err)
	}

	if len(response.Servers) != 1 {
		t.Errorf("Expected 1 server, got %d", len(response.Servers))
	}

	if response.Servers[0].Name != "server1" {
		t.Errorf("Server name = %q, want %q", response.Servers[0].Name, "server1")
	}
}

func TestLogsAPIWithFromParameter(t *testing.T) {
	sm := state.New()
	cfg := &config.Config{}
	srv := New(sm, cfg, nil)

	// Add some logs
	sm.AppendLog("test-server", "line 1")
	sm.AppendLog("test-server", "line 2")
	sm.AppendLog("test-server", "line 3")

	// Request with from=2 should only get line 3
	req := httptest.NewRequest("GET", "/api/logs/test-server/stream?from=2", nil)

	// Use a context with timeout to avoid hanging
	ctx, cancel := context.WithTimeout(context.Background(), 100*time.Millisecond)
	defer cancel()
	req = req.WithContext(ctx)

	rec := httptest.NewRecorder()
	srv.ServeHTTP(rec, req)

	body := rec.Body.String()

	// Should contain line 3 but not lines 1 and 2
	if !strings.Contains(body, "line 3") {
		t.Errorf("expected body to contain 'line 3', got: %s", body)
	}
	if strings.Contains(body, "line 1") {
		t.Errorf("expected body to NOT contain 'line 1', got: %s", body)
	}
	if strings.Contains(body, "line 2") {
		t.Errorf("expected body to NOT contain 'line 2', got: %s", body)
	}
}

func TestLogsAPIFromParameterEdgeCases(t *testing.T) {
	sm := state.New()
	cfg := &config.Config{}
	srv := New(sm, cfg, nil)

	// Add logs
	sm.AppendLog("test-server", "line 1")
	sm.AppendLog("test-server", "line 2")

	tests := []struct {
		name          string
		from          string
		shouldHave    []string
		shouldNotHave []string
	}{
		{
			name:          "from=0 gets all logs",
			from:          "0",
			shouldHave:    []string{"line 1", "line 2"},
			shouldNotHave: nil,
		},
		{
			name:          "from=1 skips first log",
			from:          "1",
			shouldHave:    []string{"line 2"},
			shouldNotHave: []string{"line 1"},
		},
		{
			name:          "from=2 gets no logs",
			from:          "2",
			shouldHave:    nil,
			shouldNotHave: []string{"line 1", "line 2"},
		},
		{
			name:          "from=100 (beyond length) gets no logs",
			from:          "100",
			shouldHave:    nil,
			shouldNotHave: []string{"line 1", "line 2"},
		},
		{
			name:          "invalid from defaults to 0",
			from:          "invalid",
			shouldHave:    []string{"line 1", "line 2"},
			shouldNotHave: nil,
		},
		{
			name:          "negative from defaults to 0",
			from:          "-5",
			shouldHave:    []string{"line 1", "line 2"},
			shouldNotHave: nil,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			url := "/api/logs/test-server/stream?from=" + tt.from
			req := httptest.NewRequest("GET", url, nil)
			ctx, cancel := context.WithTimeout(context.Background(), 100*time.Millisecond)
			defer cancel()
			req = req.WithContext(ctx)

			rec := httptest.NewRecorder()
			srv.ServeHTTP(rec, req)
			body := rec.Body.String()

			for _, s := range tt.shouldHave {
				if !strings.Contains(body, s) {
					t.Errorf("expected body to contain %q, got: %s", s, body)
				}
			}
			for _, s := range tt.shouldNotHave {
				if strings.Contains(body, s) {
					t.Errorf("expected body to NOT contain %q, got: %s", s, body)
				}
			}
		})
	}
}
