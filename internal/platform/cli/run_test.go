package cli

import (
	"bytes"
	"context"
	"crypto/sha256"
	"encoding/json"
	"fmt"
	"net"
	"net/http"
	"os"
	"path/filepath"
	"strconv"
	"strings"
	"testing"
	"time"

	"github.com/d31ma/caduceus/internal/platform/buildinfo"
)

func TestRunVersionAndUnknownCommand(t *testing.T) {
	var stdout, stderr bytes.Buffer
	info := buildinfo.New("1.2.3", "abc", "now")
	if code := Run(context.Background(), []string{"version"}, &stdout, &stderr, info); code != ExitSuccess {
		t.Fatalf("version exit = %d: %s", code, stderr.String())
	}
	if !strings.Contains(stdout.String(), "caduceus 1.2.3") {
		t.Fatalf("unexpected version: %s", stdout.String())
	}

	stdout.Reset()
	stderr.Reset()
	if code := Run(context.Background(), []string{"unknown"}, &stdout, &stderr, info); code != ExitUsage {
		t.Fatalf("unknown command exit = %d", code)
	}
	if !strings.Contains(stderr.String(), "unknown command") {
		t.Fatalf("unexpected error: %s", stderr.String())
	}
}

func TestVersionRejectsUnknownOutput(t *testing.T) {
	var stdout, stderr bytes.Buffer
	code := Run(
		context.Background(),
		[]string{"version", "--output", "yaml"},
		&stdout,
		&stderr,
		buildinfo.Current(),
	)
	if code != ExitUsage || !strings.Contains(stderr.String(), "unsupported output") {
		t.Fatalf("exit = %d, stderr = %s", code, stderr.String())
	}
}

func TestDoctorAndMigrateOutputs(t *testing.T) {
	configPath, _ := cliFixture(t)
	for _, test := range []struct {
		command string
		want    string
	}{
		{"doctor", `"status":"ok"`},
		{"migrate", `"mutated":false`},
	} {
		t.Run(test.command, func(t *testing.T) {
			var stdout, stderr bytes.Buffer
			code := Run(
				context.Background(),
				[]string{test.command, "--config", configPath, "--output", "json"},
				&stdout,
				&stderr,
				buildinfo.Current(),
			)
			if code != ExitSuccess || !strings.Contains(stdout.String(), test.want) {
				t.Fatalf("exit = %d, stdout = %s, stderr = %s", code, stdout.String(), stderr.String())
			}
		})
	}
}

func TestDoctorReportsConfigAndOutputErrors(t *testing.T) {
	var stdout, stderr bytes.Buffer
	code := Run(
		context.Background(),
		[]string{"doctor", "--config", filepath.Join(t.TempDir(), "missing.json")},
		&stdout,
		&stderr,
		buildinfo.Current(),
	)
	if code != ExitFailure || !strings.Contains(stderr.String(), "read config") {
		t.Fatalf("exit = %d, stderr = %s", code, stderr.String())
	}

	configPath, _ := cliFixture(t)
	stdout.Reset()
	stderr.Reset()
	code = Run(
		context.Background(),
		[]string{"doctor", "--config", configPath, "--output", "yaml"},
		&stdout,
		&stderr,
		buildinfo.Current(),
	)
	if code != ExitUsage || !strings.Contains(stderr.String(), "unsupported output") {
		t.Fatalf("exit = %d, stderr = %s", code, stderr.String())
	}
}

func TestServeRunsUntilContextCancellation(t *testing.T) {
	configPath, address := cliFixture(t)
	ctx, cancel := context.WithCancel(context.Background())
	var stdout, stderr bytes.Buffer
	done := make(chan int, 1)
	go func() {
		done <- Run(
			ctx,
			[]string{"serve", "--config", configPath},
			&stdout,
			&stderr,
			buildinfo.Current(),
		)
	}()
	waitForCLIHTTP(t, "http://"+address+"/readyz")
	cancel()
	select {
	case code := <-done:
		if code != ExitSuccess {
			t.Fatalf("serve exit = %d: %s", code, stderr.String())
		}
	case <-time.After(3 * time.Second):
		t.Fatal("serve did not stop")
	}
	for _, event := range []string{`"event":"server_started"`, `"event":"server_stopped"`} {
		if !strings.Contains(stderr.String(), event) {
			t.Fatalf("serve log missing %s: %s", event, stderr.String())
		}
	}
}

func cliFixture(t *testing.T) (string, string) {
	t.Helper()
	root := t.TempDir()
	staticRoot := filepath.Join(root, "static")
	schemaRoot := filepath.Join(root, "schemas")
	fyloRoot := filepath.Join(root, "fylo-root")
	for _, directory := range []string{staticRoot, schemaRoot, fyloRoot} {
		if err := os.MkdirAll(directory, 0o700); err != nil {
			t.Fatal(err)
		}
	}
	index := []byte("shell")
	if err := os.WriteFile(filepath.Join(staticRoot, "index.html"), index, 0o600); err != nil {
		t.Fatal(err)
	}
	indexDigest := fmt.Sprintf("%x", sha256.Sum256(index))
	aggregate := sha256.New()
	_, _ = fmt.Fprintf(aggregate, "index.html\x00%d\x00%s\n", len(index), indexDigest)
	writeCLIJSON(t, filepath.Join(staticRoot, ".caduceus-bundle.json"), map[string]any{
		"schemaVersion": 1,
		"source":        "tachyon",
		"files": []map[string]any{{
			"path": "index.html", "bytes": len(index), "sha256": indexDigest,
		}},
		"digest": fmt.Sprintf("%x", aggregate.Sum(nil)),
	})
	if err := os.WriteFile(filepath.Join(schemaRoot, "users.json"), []byte(`{}`), 0o600); err != nil {
		t.Fatal(err)
	}
	fylo := filepath.Join(root, "fylo")
	if err := os.WriteFile(fylo, []byte("#!/bin/sh\nprintf '26.30.04\\n'\n"), 0o700); err != nil {
		t.Fatal(err)
	}
	sesameScript := "#!/bin/sh\nprintf 'sesame 26.33.02\\n'\n"
	sesame := filepath.Join(root, "sesame")
	if err := os.WriteFile(sesame, []byte(sesameScript), 0o700); err != nil {
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
	writeCLIJSON(t, lock, map[string]any{
		"schemaVersion": 1,
		"artifacts": map[string]any{
			"fylo": map[string]any{"releasedVersion": "26.30.04"},
			"sesame": map[string]any{
				"releasedVersion": "26.33.02",
				"releasedArtifacts": []map[string]any{{
					"target": "test",
					"sha256": fmt.Sprintf("%x", sha256.Sum256([]byte(sesameScript))),
				}},
			},
		},
	})
	host := "127.0.0.1"
	port := cliFreePort(t)
	configPath := filepath.Join(root, "config.json")
	values := map[string]string{
		"HOST":                   host,
		"PORT":                   port,
		"METRICS_HOST":           host,
		"METRICS_PORT":           cliFreePort(t),
		"STATIC_DIR":             staticRoot,
		"FYLO_BINARY":            fylo,
		"FYLO_ROOT":              fyloRoot,
		"FYLO_SCHEMA":            schemaRoot,
		"RUNTIME_LOCK":           lock,
		"INBOUND_WEBHOOK_SECRET": "fedcba9876543210",
		"SESAME_BINARY":          sesame,
		"SESAME_DEPLOYMENT":      sesameDeployment,
		"SESAME_ISSUER":          "https://mail.example.test",
		"SHUTDOWN_TIMEOUT":       "2s",
	}
	configValues := make(map[string]any, len(values))
	for key, value := range values {
		configValues[key] = value
		t.Setenv(key, value)
	}
	writeCLIJSON(t, configPath, configValues)
	return configPath, net.JoinHostPort(host, port)
}

func writeCLIJSON(t *testing.T, path string, value any) {
	t.Helper()
	data, err := json.Marshal(value)
	if err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(path, data, 0o600); err != nil {
		t.Fatal(err)
	}
}

func cliFreePort(t *testing.T) string {
	t.Helper()
	listener, err := net.Listen("tcp", "127.0.0.1:0")
	if err != nil {
		t.Fatal(err)
	}
	port := listener.Addr().(*net.TCPAddr).Port
	_ = listener.Close()
	return strconv.Itoa(port)
}

func waitForCLIHTTP(t *testing.T, url string) {
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
	t.Fatalf("endpoint did not become ready: %s", url)
}
