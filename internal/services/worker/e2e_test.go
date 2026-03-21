package worker

import (
	"bytes"
	"context"
	"encoding/json"
	"io"
	"net"
	"net/http"
	"strconv"
	"strings"
	"testing"
	"time"
)

// TestE2E_Health requires a running Docker daemon to start the server (Docker client is created at startup).
func TestE2E_Health(t *testing.T) {
	dockerCli, err := NewDockerClient(context.Background())
	if err != nil {
		t.Skipf("Docker not available, skip E2E: %v", err)
	}
	defer func() { _ = dockerCli.Close() }()

	listener, err := net.Listen("tcp", "127.0.0.1:0")
	if err != nil {
		t.Fatalf("listen: %v", err)
	}
	defer func() { _ = listener.Close() }()

	port := listener.Addr().(*net.TCPAddr).Port
	srv := NewServer("", dockerCli, 0)
	go func() { _ = srv.ServeListener(listener) }()
	time.Sleep(100 * time.Millisecond)

	baseURL := "http://127.0.0.1:" + strconv.Itoa(port)

	resp, err := http.Get(baseURL + "/health")
	if err != nil {
		t.Fatalf("GET /health: %v", err)
	}
	defer func() { _ = resp.Body.Close() }()

	if resp.StatusCode != http.StatusOK {
		t.Errorf("GET /health: status = %d, want 200", resp.StatusCode)
	}
	body, _ := io.ReadAll(resp.Body)
	var payload map[string]string
	if err := json.Unmarshal(body, &payload); err != nil {
		t.Fatalf("GET /health: decode body: %v; raw=%q", err, body)
	}
	if payload["status"] != "healthy" {
		t.Errorf("GET /health: status body = %q, want %q", payload["status"], "healthy")
	}
}

// TestE2E_Execute runs a real job (alpine, echo hello) and checks the response. Requires Docker.
func TestE2E_Execute(t *testing.T) {
	dockerCli, err := NewDockerClient(context.Background())
	if err != nil {
		t.Skipf("Docker not available, skip E2E: %v", err)
	}
	defer func() { _ = dockerCli.Close() }()

	listener, err := net.Listen("tcp", "127.0.0.1:0")
	if err != nil {
		t.Fatalf("listen: %v", err)
	}
	defer func() { _ = listener.Close() }()

	port := listener.Addr().(*net.TCPAddr).Port
	srv := NewServer("", dockerCli, 0)
	go func() { _ = srv.ServeListener(listener) }()
	time.Sleep(100 * time.Millisecond)

	baseURL := "http://127.0.0.1:" + strconv.Itoa(port)

	// POST /execute with valid job (script runs via sh -c, so one line = one shell command)
	body := []byte(`{"name":"e2e","image":"alpine:latest","script":["echo hello"]}`)
	resp, err := http.Post(baseURL+"/execute", "application/json", bytes.NewReader(body))
	if err != nil {
		t.Fatalf("POST /execute: %v", err)
	}
	defer func() { _ = resp.Body.Close() }()

	bodyBytes, _ := io.ReadAll(resp.Body)
	if resp.StatusCode != http.StatusOK {
		t.Errorf("POST /execute: status = %d, want 200; body = %s", resp.StatusCode, string(bodyBytes))
		return
	}

	var result struct {
		Logs string `json:"logs"`
	}
	if err := json.Unmarshal(bodyBytes, &result); err != nil {
		t.Fatalf("decode response: %v", err)
	}
	if !strings.Contains(result.Logs, "hello") {
		t.Errorf("logs should contain 'hello', got %q", result.Logs)
	}
}

// TestE2E_Execute_failedCommand verifies non-zero container exits are treated as job failures.
func TestE2E_Execute_failedCommand(t *testing.T) {
	dockerCli, err := NewDockerClient(context.Background())
	if err != nil {
		t.Skipf("Docker not available, skip E2E: %v", err)
	}
	defer func() { _ = dockerCli.Close() }()

	listener, err := net.Listen("tcp", "127.0.0.1:0")
	if err != nil {
		t.Fatalf("listen: %v", err)
	}
	defer func() { _ = listener.Close() }()

	port := listener.Addr().(*net.TCPAddr).Port
	srv := NewServer("", dockerCli, 0)
	go func() { _ = srv.ServeListener(listener) }()
	time.Sleep(100 * time.Millisecond)

	baseURL := "http://127.0.0.1:" + strconv.Itoa(port)

	body := []byte(`{"name":"e2e-fail","image":"alpine:latest","script":["exit 7"]}`)
	resp, err := http.Post(baseURL+"/execute", "application/json", bytes.NewReader(body))
	if err != nil {
		t.Fatalf("POST /execute: %v", err)
	}
	defer func() { _ = resp.Body.Close() }()

	bodyBytes, _ := io.ReadAll(resp.Body)
	if resp.StatusCode != http.StatusInternalServerError {
		t.Fatalf("POST /execute: status = %d, want 500; body = %s", resp.StatusCode, string(bodyBytes))
	}
	if !strings.Contains(string(bodyBytes), "status 7") {
		t.Fatalf("expected error body to mention exit status, got %s", string(bodyBytes))
	}
}

// TestE2E_Execute_invalidJSON returns 400 for bad request body.
func TestE2E_Execute_invalidJSON(t *testing.T) {
	dockerCli, err := NewDockerClient(context.Background())
	if err != nil {
		t.Skipf("Docker not available, skip E2E: %v", err)
	}
	defer func() { _ = dockerCli.Close() }()

	listener, err := net.Listen("tcp", "127.0.0.1:0")
	if err != nil {
		t.Fatalf("listen: %v", err)
	}
	defer func() { _ = listener.Close() }()

	port := listener.Addr().(*net.TCPAddr).Port
	srv := NewServer("", dockerCli, 0)
	go func() { _ = srv.ServeListener(listener) }()
	time.Sleep(100 * time.Millisecond)

	baseURL := "http://127.0.0.1:" + strconv.Itoa(port)

	resp, err := http.Post(baseURL+"/execute", "application/json", bytes.NewReader([]byte("not json")))
	if err != nil {
		t.Fatalf("POST /execute: %v", err)
	}
	defer func() { _ = resp.Body.Close() }()

	if resp.StatusCode != http.StatusBadRequest {
		t.Errorf("POST /execute invalid JSON: status = %d, want 400", resp.StatusCode)
	}
}
