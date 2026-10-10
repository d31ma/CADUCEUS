package entry

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"io"
	"net"
	"net/http"
	"net/smtp"
	"os"
	"os/signal"
	"path/filepath"
	"strings"
	"sync"
	"syscall"
	"testing"
	"time"

	"github.com/d31ma/caduceus/internal/platform/fylo"
	"github.com/d31ma/caduceus/internal/platform/fylobridge"
	"github.com/d31ma/caduceus/internal/runtime/tickworker"
)

const helperEnv = "CADUCEUS_ENTRY_TEST_HELPER"

// TestMain lets the test binary stand in for `caduceus-go` (fylo-bridge,
// auth init) and for `ty serve` when the entry point spawns it.
func TestMain(m *testing.M) {
	if os.Getenv(helperEnv) == "1" && len(os.Args) > 1 {
		switch os.Args[1] {
		case "auth":
			appendLog("auth " + strings.Join(os.Args[2:], " "))
			os.Exit(0)
		case "fylo-bridge":
			os.Exit(fakeBridge())
		case "serve":
			os.Exit(fakeTachyon())
		}
	}
	os.Exit(m.Run())
}

func appendLog(line string) {
	file, err := os.OpenFile(os.Getenv("CADUCEUS_ENTRY_TEST_LOG"), os.O_APPEND|os.O_CREATE|os.O_WRONLY, 0o600)
	if err != nil {
		return
	}
	defer file.Close()
	_, _ = fmt.Fprintln(file, line)
}

type recordingCaller struct{}

func (recordingCaller) Call(_ context.Context, request map[string]any) (fylo.Response, error) {
	op, _ := request["op"].(string)
	if op == "createCollection" {
		appendLog(fmt.Sprintf("createCollection %v %v", request["collection"], request["kind"]))
	} else {
		appendLog(op)
	}
	result := json.RawMessage(`{}`)
	if op == "handshake" {
		result = json.RawMessage(`{"capabilities":{"handshake":true}}`)
	}
	return fylo.Response{ProtocolVersion: 1, OK: true, Operation: op, Result: result}, nil
}

func fakeBridge() int {
	ctx, stop := signal.NotifyContext(context.Background(), syscall.SIGTERM, os.Interrupt)
	defer stop()
	dir, _ := os.MkdirTemp("", "bridge")
	defer os.RemoveAll(dir)
	bridge := fylobridge.New(fylobridge.Options{
		Address: os.Getenv("FYLO_BRIDGE_ADDRESS"), Secret: os.Getenv("FYLO_BRIDGE_SECRET"), MaxFrameBytes: 1 << 20,
		RequestTimeout: 5 * time.Second, IncomingRoot: dir, DataRoot: dir,
	}, recordingCaller{})
	listener, err := bridge.Listen()
	if err != nil {
		fmt.Fprintln(os.Stderr, err)
		return 1
	}
	_ = bridge.Serve(ctx, listener)
	return 0
}

func fakeTachyon() int {
	appendLog("ty " + strings.Join(os.Args[1:], " "))
	secret := os.Getenv("INBOUND_WEBHOOK_SECRET")
	mux := http.NewServeMux()
	mux.HandleFunc("/health", func(w http.ResponseWriter, _ *http.Request) { _, _ = io.WriteString(w, "ok") })
	tick := func(answer string) http.HandlerFunc {
		return func(w http.ResponseWriter, request *http.Request) {
			body, _ := io.ReadAll(request.Body)
			if request.Header.Get("X-Caduceus-Signature") != tickworker.Sign(secret, body) {
				http.Error(w, "unsigned", http.StatusUnauthorized)
				return
			}
			_, _ = io.WriteString(w, answer)
		}
	}
	mux.HandleFunc("/send/tick", tick(`{"delivered":1,"retried":0,"bounced":0,"queue":{"depth":0}}`))
	mux.HandleFunc("/snooze/tick", tick(`{}`))
	mux.HandleFunc("/reminders/tick", tick(`{"due":0,"surfaced":0,"reconciled":0,"failed":0}`))
	mux.HandleFunc("/notifications/provider/tick", tick(`{}`))
	mux.HandleFunc("/inbound/webhook", func(w http.ResponseWriter, request *http.Request) {
		body, _ := io.ReadAll(request.Body)
		if request.Header.Get("X-Caduceus-Signature") != tickworker.Sign(secret, body) {
			http.Error(w, "unsigned", http.StatusUnauthorized)
			return
		}
		var payload map[string]any
		_ = json.Unmarshal(body, &payload)
		appendLog(fmt.Sprintf("webhook %v %v %v", payload["recipient"], payload["sender"], payload["subject"]))
		_, _ = io.WriteString(w, `{"accepted":true}`)
	})
	server := &http.Server{Addr: "127.0.0.1:" + os.Getenv("PORT"), Handler: mux}
	go func() { _ = server.ListenAndServe() }()
	signals := make(chan os.Signal, 1)
	signal.Notify(signals, syscall.SIGTERM, os.Interrupt)
	<-signals
	_ = server.Close()
	return 0
}

type syncBuffer struct {
	mu sync.Mutex
	b  bytes.Buffer
}

func (buffer *syncBuffer) Write(p []byte) (int, error) {
	buffer.mu.Lock()
	defer buffer.mu.Unlock()
	return buffer.b.Write(p)
}

func (buffer *syncBuffer) String() string {
	buffer.mu.Lock()
	defer buffer.mu.Unlock()
	return buffer.b.String()
}

func freePort(t *testing.T) string {
	t.Helper()
	listener, err := net.Listen("tcp", "127.0.0.1:0")
	if err != nil {
		t.Fatal(err)
	}
	defer listener.Close()
	return fmt.Sprint(listener.Addr().(*net.TCPAddr).Port)
}

func waitFor(t *testing.T, what string, condition func() bool) {
	t.Helper()
	deadline := time.Now().Add(20 * time.Second)
	for !condition() {
		if time.Now().After(deadline) {
			t.Fatalf("timed out waiting for %s", what)
		}
		time.Sleep(20 * time.Millisecond)
	}
}

func configure(t *testing.T) (dir, logPath string) {
	t.Helper()
	dir = t.TempDir()
	logPath = filepath.Join(dir, "helpers.log")
	t.Chdir(dir)
	self, err := os.Executable()
	if err != nil {
		t.Fatal(err)
	}
	for name, value := range map[string]string{
		helperEnv:                          "1",
		"CADUCEUS_ENTRY_TEST_LOG":          logPath,
		"CADUCEUS_GO_BINARY":               self,
		"TACHYON_BIN":                      self,
		"SESAME_BINARY":                    "/usr/local/bin/sesame",
		"SESAME_DEPLOYMENT":                filepath.Join(dir, "sesame"),
		"INBOUND_WEBHOOK_SECRET":           "webhook-secret",
		"EXTERNAL_ACCOUNT_CREDENTIAL_KEYS": "k1:abc",
		"EXTERNAL_ACCOUNT_ACTIVE_KEY":      "k1",
		"FYLO_ROOT":                        "data/caduceus",
		"FYLO_BRIDGE_ADDRESS":              "127.0.0.1:" + freePort(t),
		"FYLO_BRIDGE_SECRET":               "0123456789abcdef0123456789abcdef",
		"FYLO_INCOMING_ROOT":               "",
		"PORT":                             freePort(t),
		"MAIL_SERVER_ENABLED":              "true",
		"SMTP_INBOUND_HOST":                "127.0.0.1",
		"SMTP_INBOUND_PORT":                freePort(t),
		"MAIL_HOSTNAME":                    "mx.entry.test",
		"MAIL_AUTH_VERIFY":                 "off",
		"MAIL_STATUS_INTERVAL_MS":          "0",
		"MAIL_SUBMISSION_ENABLED":          "",
		"MAIL_ACME_ENABLED":                "",
		"CADUCEUS_BOOTSTRAP_DOMAINS":       "",
		"CADUCEUS_ADMIN_EMAIL":             "",
		"MAIL_TLS_KEY":                     "",
		"MAIL_TLS_CERT":                    "",
		"MAIL_TLS_KEY_FILE":                "",
		"MAIL_TLS_CERT_FILE":               "",
		"VAPID_PUBLIC_KEY":                 "",
		"NODE_ENV":                         "",
		"SMTP_ADAPTER":                     "",
		"YON_APPLICATION_JSON":             "",
	} {
		t.Setenv(name, value)
	}
	return dir, logPath
}

func TestServeRunsTheWholeRuntime(t *testing.T) {
	dir, logPath := configure(t)
	stdout, stderr := &syncBuffer{}, &syncBuffer{}
	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()
	code := make(chan int, 1)
	go func() { code <- Run(ctx, nil, stdout, stderr, Options{}) }()

	waitFor(t, "the ready status", func() bool { return strings.Contains(stdout.String(), `[mail:status] {"event":"ready"`) })
	err := smtp.SendMail("127.0.0.1:"+os.Getenv("SMTP_INBOUND_PORT"), nil, "alice@sender.test", []string{"bob@hosted.test"},
		[]byte("From: alice@sender.test\r\nSubject: entry smoke\r\n\r\nhello\r\n"))
	if err != nil {
		t.Fatal(err)
	}
	logged := func() string { data, _ := os.ReadFile(logPath); return string(data) }
	waitFor(t, "the webhook delivery", func() bool {
		return strings.Contains(logged(), "webhook bob@hosted.test alice@sender.test entry smoke")
	})
	cancel()

	select {
	case got := <-code:
		if got != 0 {
			t.Fatalf("exit = %d\nstdout:\n%s\nstderr:\n%s", got, stdout.String(), stderr.String())
		}
	case <-time.After(60 * time.Second):
		t.Fatal("entry point did not stop")
	}

	out := stdout.String()
	for _, fragment := range []string{
		"║         CADUCEUS mail server           ║",
		"  SESAME identity         ✓",
		"  SMTP → console (logs only, no delivery)",
		"  Inbound SMTP → standalone MX on :" + os.Getenv("SMTP_INBOUND_PORT"),
		"  Submission   → disabled (set MAIL_SUBMISSION_ENABLED=true)",
		"[smtp-in] inbound SMTP server listening on 127.0.0.1:" + os.Getenv("SMTP_INBOUND_PORT"),
		`"components":{"inbound-smtp":{"required":true,"state":"ready"`,
		`"outbound-queue":`, `"snooze-due":`, `"follow-up-reminders":`, `"provider-new-mail":`,
		`[mail:status] {"event":"stopped"`,
	} {
		if !strings.Contains(out, fragment) {
			t.Fatalf("stdout lacks %q:\n%s", fragment, out)
		}
	}
	lines := strings.Split(strings.TrimSpace(logged()), "\n")
	if lines[0] != "auth init" || lines[1] != "handshake" {
		t.Fatalf("helper log starts %q", lines[:2])
	}
	for index, name := range Collections {
		if lines[2+index] != "createCollection "+name+" document" {
			t.Fatalf("line %d = %q, want collection %s", 2+index, lines[2+index], name)
		}
	}
	if lines[2+len(Collections)] != "createCollection attachment-files file" || lines[3+len(Collections)] != "createCollection raw-message-files file" {
		t.Fatalf("buckets = %q", lines[2+len(Collections):4+len(Collections)])
	}
	if lines[4+len(Collections)] != "ty serve" {
		t.Fatalf("ty line = %q", lines[4+len(Collections)])
	}
	dotenv, err := os.ReadFile(filepath.Join(dir, ".env"))
	if err != nil {
		t.Fatal(err)
	}
	text := string(dotenv)
	if !strings.HasPrefix(text, "# Written by scripts/entry.mjs from the process environment.\n") ||
		!strings.Contains(text, "YON_FYLO_ROOT="+filepath.Join(dir, "data/caduceus")+"\n") ||
		!strings.Contains(text, "YON_FYLO_INCOMING_ROOT="+filepath.Join(dir, "data/caduceus/.caduceus-incoming")+"\n") ||
		!strings.Contains(text, "YON_INBOUND_WEBHOOK_SECRET=webhook-secret\n") ||
		!strings.Contains(text, "YON_FYLO_BRIDGE_SECRET=0123456789abcdef0123456789abcdef\n") {
		t.Fatalf(".env =\n%s", text)
	}
	if info, err := os.Stat(filepath.Join(dir, "data/caduceus/.caduceus-incoming")); err != nil || info.Mode().Perm() != 0o700 {
		t.Fatalf("incoming root = %v %v", info, err)
	}
}

func TestServeRefusesWithoutSecrets(t *testing.T) {
	configure(t)
	t.Setenv("INBOUND_WEBHOOK_SECRET", "")
	stdout, stderr := &syncBuffer{}, &syncBuffer{}
	if code := Run(context.Background(), []string{"serve"}, stdout, stderr, Options{}); code != 1 {
		t.Fatalf("exit = %d", code)
	}
	if !strings.Contains(stdout.String(), "  INBOUND_WEBHOOK_SECRET  ✗ MISSING (required)") ||
		!strings.Contains(stderr.String(), "ERROR: SESAME_BINARY, SESAME_DEPLOYMENT, and INBOUND_WEBHOOK_SECRET are required.") {
		t.Fatalf("stdout:\n%s\nstderr:\n%s", stdout.String(), stderr.String())
	}
}

func TestServeRefusesAForeignDotenv(t *testing.T) {
	dir, _ := configure(t)
	if err := os.WriteFile(filepath.Join(dir, ".env"), []byte("MINE=1\n"), 0o600); err != nil {
		t.Fatal(err)
	}
	stdout, stderr := &syncBuffer{}, &syncBuffer{}
	if code := Run(context.Background(), nil, stdout, stderr, Options{}); code != 1 {
		t.Fatalf("exit = %d", code)
	}
	if !strings.Contains(stderr.String(), "was not written by CADUCEUS; move it aside") {
		t.Fatalf("stderr = %s", stderr.String())
	}
	if data, _ := os.ReadFile(filepath.Join(dir, ".env")); string(data) != "MINE=1\n" {
		t.Fatal(".env was replaced")
	}
}

func TestKeyringRequired(t *testing.T) {
	configure(t)
	t.Setenv("EXTERNAL_ACCOUNT_ACTIVE_KEY", "")
	stderr := &syncBuffer{}
	if code := Run(context.Background(), nil, io.Discard, stderr, Options{}); code != 1 ||
		!strings.Contains(stderr.String(), "EXTERNAL_ACCOUNT_CREDENTIAL_KEYS and EXTERNAL_ACCOUNT_ACTIVE_KEY are required; see docs/CONFIGURATION.md") {
		t.Fatalf("code = %d stderr = %s", code, stderr.String())
	}
}

func TestCommands(t *testing.T) {
	var stdout, stderr bytes.Buffer
	if code := Run(context.Background(), []string{"help"}, &stdout, &stderr, Options{}); code != 0 || !strings.HasPrefix(stdout.String(), "Caduceus container commands:\n") {
		t.Fatalf("help = %d %q", code, stdout.String())
	}
	stdout.Reset()
	if code := Run(context.Background(), []string{"bogus"}, &stdout, &stderr, Options{}); code != 64 ||
		stderr.String() != "Unsupported Caduceus container command: bogus\nAllowed commands: serve, admin:create, domain:migrate\n" {
		t.Fatalf("bogus = %d %q", code, stderr.String())
	}
}

func TestRequiredServiceFailureStopsTheRuntime(t *testing.T) {
	_, logPath := configure(t)
	busy, err := net.Listen("tcp", "127.0.0.1:"+os.Getenv("SMTP_INBOUND_PORT"))
	if err != nil {
		t.Fatal(err)
	}
	defer busy.Close()
	stdout, stderr := &syncBuffer{}, &syncBuffer{}
	code := make(chan int, 1)
	go func() { code <- Run(context.Background(), nil, stdout, stderr, Options{}) }()
	select {
	case got := <-code:
		if got != 1 {
			t.Fatalf("exit = %d", got)
		}
	case <-time.After(60 * time.Second):
		t.Fatal("entry point did not stop")
	}
	if !strings.Contains(stderr.String(), "[mail] background mail services failed to start: listen tcp 127.0.0.1:") {
		t.Fatalf("stderr = %s", stderr.String())
	}
	if !strings.Contains(stdout.String(), `"inbound-smtp":{"required":true,"state":"failed","error":"listen tcp`) {
		t.Fatalf("stdout = %s", stdout.String())
	}
	if data, _ := os.ReadFile(logPath); !strings.Contains(string(data), "ty serve") {
		t.Fatal("Tachyon never started")
	}
}
