package web

import (
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
	"time"

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
