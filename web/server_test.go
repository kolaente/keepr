package web

import (
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

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
