package httpserver

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"io"
	"log/slog"
	"net"
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"testing"
	"time"

	"github.com/d31ma/caduceus/internal/platform/buildinfo"
	"github.com/d31ma/caduceus/internal/platform/config"
	"github.com/d31ma/caduceus/internal/platform/preflight"
)

func TestSystemContractAndProblemShape(t *testing.T) {
	root := t.TempDir()
	if err := os.WriteFile(filepath.Join(root, "index.html"), []byte("shell"), 0o600); err != nil {
		t.Fatal(err)
	}
	cfg := config.Defaults()
	cfg.StaticDir = root
	cfg.FYLOBinary = "/tmp/fylo"
	cfg.FYLORoot = "/tmp/root"
	cfg.InboundWebhookSecret = "fedcba9876543210"
	cfg.SESAMEBinary = "/tmp/sesame"
	cfg.SESAMEDeployment = "/tmp/sesame-deployment"
	cfg.SESAMEIssuer = "https://mail.example.com"
	runtime := New(cfg, preflight.Report{FYLOVersion: "1"}, buildinfo.New("1", "abc", "now"), slog.New(slog.NewJSONHandler(io.Discard, nil)))
	runtime.state.MarkReady(true)

	server := httptest.NewServer(instrument(runtime.logger, runtime.metrics, runtime.publicHandler()))
	defer server.Close()

	request, _ := http.NewRequestWithContext(context.Background(), http.MethodGet, server.URL+"/api/v1/system", nil)
	request.Header.Set("X-Request-ID", "request-12345678")
	response, err := http.DefaultClient.Do(request)
	if err != nil {
		t.Fatal(err)
	}
	defer response.Body.Close()
	var systemDocument map[string]any
	if err := json.NewDecoder(response.Body).Decode(&systemDocument); err != nil {
		t.Fatal(err)
	}
	if systemDocument["requestId"] != "request-12345678" || systemDocument["apiVersion"] != "v1" {
		t.Fatalf("unexpected system document: %#v", systemDocument)
	}

	missing, err := http.Get(server.URL + "/api/v1/missing")
	if err != nil {
		t.Fatal(err)
	}
	defer missing.Body.Close()
	var problemDocument map[string]any
	if err := json.NewDecoder(missing.Body).Decode(&problemDocument); err != nil {
		t.Fatal(err)
	}
	if missing.StatusCode != http.StatusNotFound || problemDocument["code"] != "resource_not_found" {
		t.Fatalf("unexpected problem document: %#v", problemDocument)
	}
}

func TestReadinessTurnsOff(t *testing.T) {
	state := New(config.Defaults(), preflight.Report{}, buildinfo.Current(), slog.Default())
	state.state.MarkReady(true)
	state.state.MarkReady(false)
	request := httptest.NewRequest(http.MethodGet, "/readyz", nil)
	response := httptest.NewRecorder()
	state.ready(response, request)
	if response.Code != http.StatusServiceUnavailable {
		t.Fatalf("ready status = %d", response.Code)
	}
}

func TestAcceptedRequestIDRejectsUnboundedInput(t *testing.T) {
	if got := acceptedRequestID("short"); len(got) != 32 {
		t.Fatalf("generated request ID = %q", got)
	}
	if got := acceptedRequestID("customer-request-1234"); got != "customer-request-1234" {
		t.Fatalf("valid request ID changed: %q", got)
	}
}

func TestSystemUptimeIsNonNegative(t *testing.T) {
	state := New(config.Defaults(), preflight.Report{}, buildinfo.Current(), slog.Default())
	if got := state.state.Snapshot(time.Time{}).UptimeMS; got != 0 {
		t.Fatalf("negative uptime was not clamped: %d", got)
	}
}

func TestStaticHandlerServesAssetsSPAAndRejectsHiddenFiles(t *testing.T) {
	root := t.TempDir()
	if err := os.WriteFile(filepath.Join(root, "index.html"), []byte("shell"), 0o600); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(root, "app.js"), []byte("application"), 0o600); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(root, ".secret"), []byte("hidden"), 0o600); err != nil {
		t.Fatal(err)
	}
	handler := instrument(slog.Default(), newMetrics(), staticHandler(root, []string{"index.html", "app.js"}))

	for path, body := range map[string]string{"/": "shell", "/inbox": "shell", "/app.js": "application"} {
		request := httptest.NewRequest(http.MethodGet, path, nil)
		response := httptest.NewRecorder()
		handler.ServeHTTP(response, request)
		if response.Code != http.StatusOK || response.Body.String() != body {
			t.Errorf("GET %s = %d %q", path, response.Code, response.Body.String())
		}
	}
	hidden := httptest.NewRecorder()
	handler.ServeHTTP(hidden, httptest.NewRequest(http.MethodGet, "/.secret", nil))
	if hidden.Code != http.StatusNotFound || bytes.Contains(hidden.Body.Bytes(), []byte("hidden")) {
		t.Fatalf("hidden file response = %d %q", hidden.Code, hidden.Body.String())
	}
	unmanifested := httptest.NewRecorder()
	if err := os.WriteFile(filepath.Join(root, "later.txt"), []byte("not allowed"), 0o600); err != nil {
		t.Fatal(err)
	}
	handler.ServeHTTP(unmanifested, httptest.NewRequest(http.MethodGet, "/later.txt", nil))
	if unmanifested.Code != http.StatusNotFound || bytes.Contains(unmanifested.Body.Bytes(), []byte("not allowed")) {
		t.Fatalf("unmanifested asset was served: %d %q", unmanifested.Code, unmanifested.Body.String())
	}
	method := httptest.NewRecorder()
	handler.ServeHTTP(method, httptest.NewRequest(http.MethodPost, "/", nil))
	if method.Code != http.StatusMethodNotAllowed {
		t.Fatalf("POST static = %d", method.Code)
	}
}

func TestMetricsHaveBoundedLabels(t *testing.T) {
	metrics := newMetrics()
	metrics.begin()
	metrics.end("/api/v1/system", http.MethodGet, http.StatusOK, 250*time.Millisecond)
	var output bytes.Buffer
	metrics.writePrometheus(&output, true)
	body := output.String()
	for _, expected := range []string{
		"caduceus_ready 1",
		`route="/api/v1/system"`,
		`status="200"`,
		"caduceus_http_request_duration_seconds_total",
	} {
		if !bytes.Contains(output.Bytes(), []byte(expected)) {
			t.Errorf("metrics missing %q:\n%s", expected, body)
		}
	}
}

func TestRunBindsBothListenersAndStopsOnContext(t *testing.T) {
	cfg := config.Defaults()
	cfg.Host = "127.0.0.1"
	cfg.Port = freePort(t)
	cfg.MetricsHost = "127.0.0.1"
	cfg.MetricsPort = freePort(t)
	cfg.ShutdownTimeout = "2s"
	state := New(cfg, preflight.Report{FYLOVersion: "1"}, buildinfo.Current(), slog.New(slog.NewJSONHandler(io.Discard, nil)))
	ctx, cancel := context.WithCancel(context.Background())
	done := make(chan error, 1)
	go func() { done <- state.Run(ctx) }()
	waitHTTP(t, "http://"+cfg.HTTPAddress()+"/readyz")
	cancel()
	select {
	case err := <-done:
		if err != nil {
			t.Fatalf("Run error = %v", err)
		}
	case <-time.After(3 * time.Second):
		t.Fatal("Run did not honor cancellation")
	}
	if state.state.Ready() {
		t.Fatal("runtime remained ready after Run returned")
	}
}

func TestRunClosesFirstListenerWhenSecondBindFails(t *testing.T) {
	blocked, err := net.Listen("tcp", "127.0.0.1:0")
	if err != nil {
		t.Fatal(err)
	}
	defer blocked.Close()
	cfg := config.Defaults()
	cfg.Host = "127.0.0.1"
	cfg.Port = freePort(t)
	cfg.MetricsHost = "127.0.0.1"
	cfg.MetricsPort = fmt.Sprint(blocked.Addr().(*net.TCPAddr).Port)
	state := New(cfg, preflight.Report{}, buildinfo.Current(), slog.Default())
	if err := state.Run(context.Background()); err == nil || !bytes.Contains([]byte(err.Error()), []byte("metrics listener")) {
		t.Fatalf("Run error = %v", err)
	}
	listener, err := net.Listen("tcp", cfg.HTTPAddress())
	if err != nil {
		t.Fatalf("public listener leaked after metrics bind failure: %v", err)
	}
	_ = listener.Close()
}

func freePort(t *testing.T) string {
	t.Helper()
	listener, err := net.Listen("tcp", "127.0.0.1:0")
	if err != nil {
		t.Fatal(err)
	}
	port := listener.Addr().(*net.TCPAddr).Port
	_ = listener.Close()
	return fmt.Sprint(port)
}

func waitHTTP(t *testing.T, url string) {
	t.Helper()
	deadline := time.Now().Add(3 * time.Second)
	for time.Now().Before(deadline) {
		response, err := http.Get(url)
		if err == nil {
			_ = response.Body.Close()
			if response.StatusCode == http.StatusOK {
				return
			}
		}
		time.Sleep(10 * time.Millisecond)
	}
	t.Fatalf("HTTP endpoint did not become ready: %s", url)
}
