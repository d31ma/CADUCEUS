package blackbox_test

import (
	"bytes"
	"crypto/sha256"
	"encoding/json"
	"fmt"
	"io"
	"net"
	"net/http"
	"os"
	"os/exec"
	"path/filepath"
	"runtime"
	"strconv"
	"strings"
	"sync"
	"syscall"
	"testing"
	"time"
)

var (
	buildOnce sync.Once
	binary    string
	buildErr  error
)

type runtimeConfig struct {
	HOST                   string `json:"HOST"`
	PORT                   string `json:"PORT"`
	METRICS_HOST           string `json:"METRICS_HOST"`
	METRICS_PORT           string `json:"METRICS_PORT"`
	STATIC_DIR             string `json:"STATIC_DIR"`
	FYLO_BINARY            string `json:"FYLO_BINARY"`
	FYLO_ROOT              string `json:"FYLO_ROOT"`
	FYLO_SCHEMA            string `json:"FYLO_SCHEMA"`
	RUNTIME_LOCK           string `json:"RUNTIME_LOCK"`
	INBOUND_WEBHOOK_SECRET string `json:"INBOUND_WEBHOOK_SECRET"`
	SESAME_BINARY          string `json:"SESAME_BINARY"`
	SESAME_DEPLOYMENT      string `json:"SESAME_DEPLOYMENT"`
	SESAME_ISSUER          string `json:"SESAME_ISSUER"`
	SHUTDOWN_TIMEOUT       string `json:"SHUTDOWN_TIMEOUT"`
}

func TestVersionJSON(t *testing.T) {
	result := command(t, "version", "--output", "json")
	if result.err != nil {
		t.Fatalf("version failed: %v\n%s", result.err, result.stderr)
	}
	var got map[string]any
	decodeJSON(t, result.stdout, &got)
	for _, key := range []string{"name", "version", "commit", "built_at", "go_version", "os", "arch"} {
		if strings.TrimSpace(fmt.Sprint(got[key])) == "" {
			t.Errorf("version output has empty %q: %s", key, result.stdout)
		}
	}
	if got["name"] != "caduceus" {
		t.Errorf("name = %v, want caduceus", got["name"])
	}
}

func TestUnknownConfigurationFailsBeforeBinding(t *testing.T) {
	port := availablePort(t)
	path := writeJSON(t, map[string]any{
		"HOST":            "127.0.0.1",
		"PORT":            strconv.Itoa(port),
		"UNREVIEWED_MODE": true,
	})
	result := command(t, "serve", "--config", path)
	if result.err == nil {
		t.Fatalf("serve unexpectedly accepted unknown config: %s", result.stdout)
	}
	if !strings.Contains(result.stderr, "unknown field") {
		t.Fatalf("stderr does not identify unknown field: %s", result.stderr)
	}
	listener, err := net.Listen("tcp", fmt.Sprintf("127.0.0.1:%d", port))
	if err != nil {
		t.Fatalf("invalid config left listener bound: %v", err)
	}
	_ = listener.Close()
}

func TestDoctorRejectsMissingDependencyWithoutLeakingSecrets(t *testing.T) {
	cfg := validConfig(t)
	cfg.FYLO_BINARY = filepath.Join(t.TempDir(), "missing-fylo")
	path := writeJSON(t, cfg)
	result := command(t, "doctor", "--config", path, "--output", "json")
	if result.err == nil {
		t.Fatalf("doctor unexpectedly succeeded: %s", result.stdout)
	}
	if !strings.Contains(result.stderr, "FYLO binary") {
		t.Fatalf("stderr does not identify dependency: %s", result.stderr)
	}
	if strings.Contains(result.stderr, cfg.INBOUND_WEBHOOK_SECRET) {
		t.Fatal("doctor leaked a configured secret")
	}
}

func TestMissingDependencyFailsServeBeforeBinding(t *testing.T) {
	cfg := validConfig(t)
	cfg.FYLO_BINARY = filepath.Join(t.TempDir(), "missing-fylo")
	path := writeJSON(t, cfg)
	result := command(t, "serve", "--config", path)
	if result.err == nil || !strings.Contains(result.stderr, "FYLO binary") {
		t.Fatalf("serve error = %v, stderr = %s", result.err, result.stderr)
	}
	listener, err := net.Listen("tcp", net.JoinHostPort(cfg.HOST, cfg.PORT))
	if err != nil {
		t.Fatalf("dependency failure left HTTP listener bound: %v", err)
	}
	_ = listener.Close()
}

func TestServeSystemStaticMetricsAndGracefulShutdown(t *testing.T) {
	cfg := validConfig(t)
	path := writeJSON(t, cfg)
	process := startCommand(t, "serve", "--config", path)
	base := "http://" + net.JoinHostPort(cfg.HOST, cfg.PORT)
	ops := "http://" + net.JoinHostPort(cfg.METRICS_HOST, cfg.METRICS_PORT)

	waitFor(t, base+"/readyz", http.StatusOK)

	root := get(t, base+"/")
	if !strings.Contains(string(root.body), "phase-one-login-shell") {
		t.Fatalf("root did not serve synchronized frontend: %s", root.body)
	}
	if root.header.Get("X-Request-ID") == "" {
		t.Fatal("root response has no X-Request-ID")
	}
	if root.header.Get("X-Content-Type-Options") != "nosniff" {
		t.Fatalf("missing security headers: %#v", root.header)
	}

	system := get(t, base+"/api/v1/system")
	var document map[string]any
	decodeJSON(t, string(system.body), &document)
	if document["apiVersion"] != "v1" || document["status"] != "ready" {
		t.Fatalf("unexpected system document: %s", system.body)
	}
	if document["requestId"] != system.header.Get("X-Request-ID") {
		t.Fatalf("system request ID does not match response header: %s", system.body)
	}

	metrics := get(t, ops+"/metrics")
	if !strings.Contains(string(metrics.body), "caduceus_http_requests_total") {
		t.Fatalf("metrics missing request counter: %s", metrics.body)
	}

	if err := process.cmd.Process.Signal(syscall.SIGTERM); err != nil {
		t.Fatalf("send SIGTERM: %v", err)
	}
	done := make(chan error, 1)
	go func() { done <- process.cmd.Wait() }()
	select {
	case err := <-done:
		if err != nil {
			t.Fatalf("serve exited unsuccessfully: %v\n%s", err, process.stderr.String())
		}
	case <-time.After(5 * time.Second):
		_ = process.cmd.Process.Kill()
		t.Fatal("serve exceeded graceful shutdown bound")
	}
	if !strings.Contains(process.stderr.String(), `"event":"server_stopped"`) {
		t.Fatalf("shutdown log missing stable event: %s", process.stderr.String())
	}
	if strings.Contains(process.stderr.String(), cfg.INBOUND_WEBHOOK_SECRET) {
		t.Fatal("serve logs leaked a configured secret")
	}
}

func TestMigrateIsReadOnlyPreflight(t *testing.T) {
	cfg := validConfig(t)
	path := writeJSON(t, cfg)
	result := command(t, "migrate", "--config", path, "--output", "json")
	if result.err != nil {
		t.Fatalf("migrate failed: %v\n%s", result.err, result.stderr)
	}
	var got map[string]any
	decodeJSON(t, result.stdout, &got)
	if got["mode"] != "preflight" || got["mutated"] != false {
		t.Fatalf("unexpected migrate result: %s", result.stdout)
	}
	if got["schemaFiles"].(float64) < 1 {
		t.Fatalf("migrate did not discover schemas: %s", result.stdout)
	}
}

type commandResult struct {
	stdout string
	stderr string
	err    error
}

func command(t *testing.T, args ...string) commandResult {
	t.Helper()
	cmd := exec.Command(testBinary(t), args...)
	cmd.Env = minimalEnvironment()
	var stdout, stderr bytes.Buffer
	cmd.Stdout = &stdout
	cmd.Stderr = &stderr
	err := cmd.Run()
	return commandResult{stdout: stdout.String(), stderr: stderr.String(), err: err}
}

type runningCommand struct {
	cmd    *exec.Cmd
	stderr *lockedBuffer
}

func startCommand(t *testing.T, args ...string) runningCommand {
	t.Helper()
	cmd := exec.Command(testBinary(t), args...)
	cmd.Env = minimalEnvironment()
	stderr := &lockedBuffer{}
	cmd.Stdout = io.Discard
	cmd.Stderr = stderr
	if err := cmd.Start(); err != nil {
		t.Fatalf("start caduceus: %v", err)
	}
	t.Cleanup(func() {
		if cmd.ProcessState == nil {
			_ = cmd.Process.Kill()
			_, _ = cmd.Process.Wait()
		}
	})
	return runningCommand{cmd: cmd, stderr: stderr}
}

type lockedBuffer struct {
	mu sync.Mutex
	b  bytes.Buffer
}

func (b *lockedBuffer) Write(value []byte) (int, error) {
	b.mu.Lock()
	defer b.mu.Unlock()
	return b.b.Write(value)
}

func (b *lockedBuffer) String() string {
	b.mu.Lock()
	defer b.mu.Unlock()
	return b.b.String()
}

func testBinary(t *testing.T) string {
	t.Helper()
	buildOnce.Do(func() {
		dir, err := os.MkdirTemp("", "caduceus-blackbox-*")
		if err != nil {
			buildErr = err
			return
		}
		binary = filepath.Join(dir, "caduceus")
		if runtime.GOOS == "windows" {
			binary += ".exe"
		}
		cmd := exec.Command("go", "build", "-buildvcs=false", "-o", binary, "./cmd/caduceus")
		cmd.Dir = projectRoot()
		cmd.Env = os.Environ()
		output, err := cmd.CombinedOutput()
		if err != nil {
			buildErr = fmt.Errorf("%w: %s", err, output)
		}
	})
	if buildErr != nil {
		t.Fatalf("build caduceus: %v", buildErr)
	}
	return binary
}

func validConfig(t *testing.T) runtimeConfig {
	t.Helper()
	root := t.TempDir()
	staticDir := filepath.Join(root, "static")
	schemaDir := filepath.Join(root, "schemas")
	fyloRoot := filepath.Join(root, "fylo-root")
	for _, dir := range []string{staticDir, schemaDir, fyloRoot} {
		if err := os.MkdirAll(dir, 0o700); err != nil {
			t.Fatal(err)
		}
	}
	if err := os.WriteFile(filepath.Join(staticDir, "index.html"), []byte("<!doctype html><title>phase-one-login-shell</title>"), 0o600); err != nil {
		t.Fatal(err)
	}
	writeFrontendManifest(t, staticDir, "index.html")
	if err := os.WriteFile(filepath.Join(schemaDir, "users.json"), []byte(`{"type":"object"}`), 0o600); err != nil {
		t.Fatal(err)
	}
	fyloBinary := filepath.Join(root, "fylo")
	script := "#!/bin/sh\nprintf '26.30.04\\n'\n"
	if err := os.WriteFile(fyloBinary, []byte(script), 0o700); err != nil {
		t.Fatal(err)
	}
	sesameBinary := filepath.Join(root, "sesame")
	sesameScript := "#!/bin/sh\nprintf 'sesame 26.33.02\\n'\n"
	if err := os.WriteFile(sesameBinary, []byte(sesameScript), 0o700); err != nil {
		t.Fatal(err)
	}
	sesameDeployment := filepath.Join(root, "sesame-deployment")
	if err := os.MkdirAll(filepath.Join(sesameDeployment, "keys"), 0o700); err != nil {
		t.Fatal(err)
	}
	for name, contents := range map[string]string{
		"config.json":      `{"config_version":1}`,
		"keys/signing.key": "signing",
		"keys/secrets.key": "secrets",
	} {
		if err := os.WriteFile(filepath.Join(sesameDeployment, name), []byte(contents), 0o600); err != nil {
			t.Fatal(err)
		}
	}
	lock := filepath.Join(root, "runtime.lock.json")
	lockBody := fmt.Sprintf(
		`{"schemaVersion":1,"artifacts":{"fylo":{"releasedVersion":"26.30.04"},`+
			`"sesame":{"releasedVersion":"26.33.02",`+
			`"releasedArtifacts":[{"target":"test","sha256":"%x"}]}}}`,
		sha256.Sum256([]byte(sesameScript)),
	)
	if err := os.WriteFile(lock, []byte(lockBody), 0o600); err != nil {
		t.Fatal(err)
	}
	return runtimeConfig{
		HOST:                   "127.0.0.1",
		PORT:                   strconv.Itoa(availablePort(t)),
		METRICS_HOST:           "127.0.0.1",
		METRICS_PORT:           strconv.Itoa(availablePort(t)),
		STATIC_DIR:             staticDir,
		FYLO_BINARY:            fyloBinary,
		FYLO_ROOT:              fyloRoot,
		FYLO_SCHEMA:            schemaDir,
		RUNTIME_LOCK:           lock,
		INBOUND_WEBHOOK_SECRET: "phase-one-webhook-secret",
		SESAME_BINARY:          sesameBinary,
		SESAME_DEPLOYMENT:      sesameDeployment,
		SESAME_ISSUER:          "https://mail.example.test",
		SHUTDOWN_TIMEOUT:       "2s",
	}
}

func writeFrontendManifest(t *testing.T, root, name string) {
	t.Helper()
	data, err := os.ReadFile(filepath.Join(root, name))
	if err != nil {
		t.Fatal(err)
	}
	digest := fmt.Sprintf("%x", sha256.Sum256(data))
	aggregate := sha256.New()
	_, _ = fmt.Fprintf(aggregate, "%s\x00%d\x00%s\n", name, len(data), digest)
	manifest := map[string]any{
		"schemaVersion": 1,
		"source":        "tachyon",
		"files": []map[string]any{{
			"path":   name,
			"bytes":  len(data),
			"sha256": digest,
		}},
		"digest": fmt.Sprintf("%x", aggregate.Sum(nil)),
	}
	manifestData, err := json.Marshal(manifest)
	if err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(root, ".caduceus-bundle.json"), manifestData, 0o600); err != nil {
		t.Fatal(err)
	}
}

func availablePort(t *testing.T) int {
	t.Helper()
	listener, err := net.Listen("tcp", "127.0.0.1:0")
	if err != nil {
		t.Fatal(err)
	}
	port := listener.Addr().(*net.TCPAddr).Port
	_ = listener.Close()
	return port
}

func writeJSON(t *testing.T, value any) string {
	t.Helper()
	path := filepath.Join(t.TempDir(), "config.json")
	data, err := json.Marshal(value)
	if err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(path, data, 0o600); err != nil {
		t.Fatal(err)
	}
	return path
}

type response struct {
	header http.Header
	body   []byte
}

func get(t *testing.T, url string) response {
	t.Helper()
	client := &http.Client{Timeout: 2 * time.Second}
	result, err := client.Get(url)
	if err != nil {
		t.Fatalf("GET %s: %v", url, err)
	}
	defer result.Body.Close()
	body, err := io.ReadAll(result.Body)
	if err != nil {
		t.Fatal(err)
	}
	if result.StatusCode != http.StatusOK {
		t.Fatalf("GET %s = %d: %s", url, result.StatusCode, body)
	}
	return response{header: result.Header, body: body}
}

func waitFor(t *testing.T, url string, status int) {
	t.Helper()
	deadline := time.Now().Add(10 * time.Second)
	for time.Now().Before(deadline) {
		client := &http.Client{Timeout: 200 * time.Millisecond}
		result, err := client.Get(url)
		if err == nil {
			_ = result.Body.Close()
			if result.StatusCode == status {
				return
			}
		}
		time.Sleep(25 * time.Millisecond)
	}
	t.Fatalf("server did not become ready at %s", url)
}

func decodeJSON(t *testing.T, value string, target any) {
	t.Helper()
	if err := json.Unmarshal([]byte(value), target); err != nil {
		t.Fatalf("decode JSON %q: %v", value, err)
	}
}

func projectRoot() string {
	_, file, _, _ := runtime.Caller(0)
	return filepath.Clean(filepath.Join(filepath.Dir(file), "..", ".."))
}

func minimalEnvironment() []string {
	values := []string{"PATH=" + os.Getenv("PATH")}
	for _, key := range []string{"HOME", "SystemRoot", "TMPDIR", "TEMP", "TMP", "GOTOOLCHAIN", "GOCACHE", "GOMODCACHE"} {
		if value := os.Getenv(key); value != "" {
			values = append(values, key+"="+value)
		}
	}
	return values
}
