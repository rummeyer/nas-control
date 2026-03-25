package main

import (
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"testing"
)

// ---------- writeJSON ----------

func TestWriteJSON(t *testing.T) {
	w := httptest.NewRecorder()
	writeJSON(w, http.StatusOK, Response{"ok", "hello"})

	if w.Code != http.StatusOK {
		t.Fatalf("expected status 200, got %d", w.Code)
	}
	if ct := w.Header().Get("Content-Type"); ct != "application/json" {
		t.Fatalf("expected Content-Type application/json, got %s", ct)
	}

	var resp Response
	if err := json.NewDecoder(w.Body).Decode(&resp); err != nil {
		t.Fatal(err)
	}
	if resp.Status != "ok" || resp.Message != "hello" {
		t.Fatalf("unexpected response body: %+v", resp)
	}
}

func TestWriteJSONErrorStatus(t *testing.T) {
	w := httptest.NewRecorder()
	writeJSON(w, http.StatusBadGateway, Response{"error", "fail"})

	if w.Code != http.StatusBadGateway {
		t.Fatalf("expected status 502, got %d", w.Code)
	}
}

// ---------- isPrivateIP ----------

func TestIsPrivateIP(t *testing.T) {
	tests := []struct {
		addr    string
		private bool
	}{
		{"127.0.0.1:1234", true},
		{"10.0.1.5:80", true},
		{"192.168.1.1:443", true},
		{"172.16.0.1:8080", true},
		{"172.31.255.255:80", true},
		{"8.8.8.8:53", false},
		{"1.2.3.4:80", false},
		{"[::1]:80", true},
		// bare IPs (no port)
		{"192.168.0.1", true},
		{"8.8.4.4", false},
		// invalid
		{"not-an-ip", false},
	}

	for _, tt := range tests {
		got := isPrivateIP(tt.addr)
		if got != tt.private {
			t.Errorf("isPrivateIP(%q) = %v, want %v", tt.addr, got, tt.private)
		}
	}
}

// ---------- localOnly middleware ----------

func TestLocalOnlyAllowsPrivate(t *testing.T) {
	handler := localOnly(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusOK)
	}))

	req := httptest.NewRequest("GET", "/", nil)
	req.RemoteAddr = "192.168.1.10:12345"
	w := httptest.NewRecorder()
	handler.ServeHTTP(w, req)

	if w.Code != http.StatusOK {
		t.Fatalf("expected 200 for private IP, got %d", w.Code)
	}
}

func TestLocalOnlyBlocksPublic(t *testing.T) {
	handler := localOnly(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusOK)
	}))

	req := httptest.NewRequest("GET", "/", nil)
	req.RemoteAddr = "8.8.8.8:12345"
	w := httptest.NewRecorder()
	handler.ServeHTTP(w, req)

	if w.Code != http.StatusForbidden {
		t.Fatalf("expected 403 for public IP, got %d", w.Code)
	}
}

// ---------- handleInfo ----------

func TestHandleInfo(t *testing.T) {
	config.NAS.URL = "http://10.0.1.201:10000"

	req := httptest.NewRequest("GET", "/info", nil)
	w := httptest.NewRecorder()
	handleInfo(w, req)

	if w.Code != http.StatusOK {
		t.Fatalf("expected 200, got %d", w.Code)
	}

	var body map[string]string
	if err := json.NewDecoder(w.Body).Decode(&body); err != nil {
		t.Fatal(err)
	}
	if body["nas_ip"] != "10.0.1.201" {
		t.Fatalf("expected nas_ip=10.0.1.201, got %s", body["nas_ip"])
	}
}

// ---------- handleIndex ----------

func TestHandleIndexServesFile(t *testing.T) {
	dir := t.TempDir()
	htmlPath := filepath.Join(dir, "index.html")
	os.WriteFile(htmlPath, []byte("<h1>Test</h1>"), 0644)

	origDir, _ := os.Getwd()
	os.Chdir(dir)
	defer os.Chdir(origDir)

	req := httptest.NewRequest("GET", "/", nil)
	w := httptest.NewRecorder()
	handleIndex(w, req)

	if w.Code != http.StatusOK {
		t.Fatalf("expected 200, got %d", w.Code)
	}
	if ct := w.Header().Get("Content-Type"); ct != "text/html" {
		t.Fatalf("expected text/html, got %s", ct)
	}
	if w.Body.String() != "<h1>Test</h1>" {
		t.Fatalf("unexpected body: %s", w.Body.String())
	}
}

func TestHandleIndexNotFound(t *testing.T) {
	dir := t.TempDir()
	origDir, _ := os.Getwd()
	os.Chdir(dir)
	defer os.Chdir(origDir)

	req := httptest.NewRequest("GET", "/", nil)
	w := httptest.NewRecorder()
	handleIndex(w, req)

	if w.Code != http.StatusInternalServerError {
		t.Fatalf("expected 500 when index.html missing, got %d", w.Code)
	}
}

func TestHandleIndex404ForOtherPaths(t *testing.T) {
	req := httptest.NewRequest("GET", "/nonexistent", nil)
	w := httptest.NewRecorder()
	handleIndex(w, req)

	if w.Code != http.StatusNotFound {
		t.Fatalf("expected 404, got %d", w.Code)
	}
}

// ---------- handleState ----------

func TestHandleStateReturnsValidJSON(t *testing.T) {
	// Use an unreachable IP so we get a quick "offline" without network calls.
	config.NAS.URL = "http://192.0.2.1:1" // RFC 5737 TEST-NET, guaranteed unreachable

	req := httptest.NewRequest("GET", "/state", nil)
	w := httptest.NewRecorder()
	handleState(w, req)

	if w.Code != http.StatusOK {
		t.Fatalf("expected 200, got %d", w.Code)
	}

	var resp Response
	if err := json.NewDecoder(w.Body).Decode(&resp); err != nil {
		t.Fatal(err)
	}
	if resp.Status != "ok" {
		t.Fatalf("expected status ok, got %s", resp.Status)
	}
	if resp.Message != "online" && resp.Message != "offline" {
		t.Fatalf("unexpected message: %s", resp.Message)
	}
}
