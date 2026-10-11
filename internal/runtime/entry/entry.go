// Package entry is the CADUCEUS production entry point: it starts the Go
// FYLO owner, creates the collections, hands the settings to Tachyon Yon as
// YON_* in `.env`, runs `ty serve`, and supervises the background mail
// services (inbound MX, submission, ACME, and the tick workers).
//
// It ports scripts/entry.mjs; the commands, settings names, defaults and log
// lines are the JavaScript ones.
package entry

import (
	"context"
	"crypto/rand"
	"encoding/hex"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"net"
	"net/http"
	"os"
	"os/signal"
	"path/filepath"
	"regexp"
	"strconv"
	"strings"
	"sync"
	"syscall"
	"time"

	"github.com/d31ma/caduceus/internal/platform/bridgeclient"
	"github.com/d31ma/caduceus/internal/platform/fylo"
	"github.com/d31ma/caduceus/internal/runtime/settings"
	"github.com/d31ma/caduceus/internal/runtime/supervisor"
	"github.com/d31ma/caduceus/internal/runtime/tickworker"
)

// Exit codes of the container commands.
const (
	exitOK      = 0
	exitFailure = 1
	exitUsage   = 64
)

// Collections are the FYLO document collections the entry point creates, in
// the order Fylo.createDb created them (internal/platform/fylo owns the list).
var Collections = fylo.DocumentCollections()

// Buckets are the FYLO file collections (Fylo.Buckets).
var Buckets = fylo.FileCollections()

// Options are the seams tests replace.
type Options struct {
	// Self is the Go binary that serves `fylo-bridge` and `auth init`
	// (default CADUCEUS_GO_BINARY, else this executable).
	Self string
	// Services adds the mail services; nil uses the production set.
	Services func(*Session) []supervisor.Service
	// APITimeout overrides the /health poll timeout
	// (default CADUCEUS_API_START_TIMEOUT_MS, else 15 minutes).
	APITimeout time.Duration
}

// Run executes one container command: serve (default), admin:create,
// domain:migrate or help.
func Run(ctx context.Context, args []string, stdout, stderr io.Writer, options Options) int {
	command := "serve"
	if len(args) > 0 && args[0] != "" {
		command, args = args[0], args[1:]
	} else if len(args) > 0 {
		args = args[1:]
	}
	runner := &runner{stdout: stdout, stderr: stderr, options: options}
	switch command {
	case "serve":
		return runner.serve(ctx, args)
	case "admin:create":
		return runner.operator(ctx, "scripts/create-admin.mjs", args)
	case "domain:migrate":
		return runner.operator(ctx, "scripts/migrate-domain.mjs", args)
	case "help", "--help", "-h":
		fmt.Fprintln(stdout, strings.Join([]string{
			"Caduceus container commands:",
			"  serve           Start the Caduceus API and frontend server",
			"  admin:create    Create the first admin for a domain",
			"  domain:migrate  Promote users from one domain suffix to another",
			"",
			"Examples:",
			"  docker run ghcr.io/d31ma/caduceus:latest",
			"  docker run -v caduceus-data:/data ghcr.io/d31ma/caduceus:latest admin:create --email=admin@example.com --domain=example.com",
			"  docker run -v caduceus-data:/data ghcr.io/d31ma/caduceus:latest domain:migrate --from=old.example --to=new.example --apply",
		}, "\n"))
		return exitOK
	default:
		fmt.Fprintf(stderr, "Unsupported Caduceus container command: %s\n", command)
		fmt.Fprintln(stderr, "Allowed commands: serve, admin:create, domain:migrate")
		return exitUsage
	}
}

type runner struct {
	stdout, stderr io.Writer
	options        Options
}

func env(name string) string { return os.Getenv(name) }

func check(ok bool, yes, no string) string {
	if ok {
		return yes
	}
	return no
}

func (runner *runner) summary() {
	out := runner.stdout
	smtp := env("SMTP_ADAPTER")
	if smtp == "" {
		smtp = check(env("NODE_ENV") == "production", "direct", "console")
	}
	pushDisabled := env("WEB_PUSH_DISABLED") == "true"
	pushKeys := env("VAPID_PUBLIC_KEY") != "" && env("VAPID_PRIVATE_KEY") != ""
	hasIdentity := env("SESAME_DEPLOYMENT") != "" && env("SESAME_BINARY") != ""
	hasInbound := env("INBOUND_WEBHOOK_SECRET") != ""
	mailServer := env("MAIL_SERVER_ENABLED") == "true"
	inboundPort := env("SMTP_INBOUND_PORT")
	if inboundPort == "" {
		inboundPort = "25"
	}
	fmt.Fprintln(out, "")
	fmt.Fprintln(out, "╔══════════════════════════════════════╗")
	fmt.Fprintln(out, "║         CADUCEUS mail server           ║")
	fmt.Fprintln(out, "╚══════════════════════════════════════╝")
	fmt.Fprintln(out, "")
	fmt.Fprintln(out, "Secrets:")
	fmt.Fprintf(out, "  SESAME identity         %s\n", check(hasIdentity, "✓", "✗ MISSING (required)"))
	fmt.Fprintf(out, "  INBOUND_WEBHOOK_SECRET  %s\n", check(hasInbound, "✓", "✗ MISSING (required)"))
	fmt.Fprintln(out, "")
	fmt.Fprintln(out, "Delivery:")
	fmt.Fprintf(out, "  SMTP → %s%s\n", smtp, check(smtp == "console", " (logs only, no delivery)", ""))
	fmt.Fprintf(out, "  Inbound SMTP → %s\n", check(mailServer, "standalone MX on :"+inboundPort, "disabled (set MAIL_SERVER_ENABLED=true)"))
	submission := "disabled (set MAIL_SUBMISSION_ENABLED=true)"
	if env("MAIL_SUBMISSION_ENABLED") == "true" {
		port := env("SMTP_SUBMISSION_PORT")
		if port == "" {
			port = "587"
		}
		submission = "on :" + port
		if tlsPort := env("SMTP_SUBMISSION_TLS_PORT"); tlsPort != "" {
			submission += "/" + tlsPort
		}
	}
	fmt.Fprintf(out, "  Submission   → %s\n", submission)
	fmt.Fprintf(out, "  Outbound queue → %s\n", check(env("OUTBOUND_QUEUE_WORKER") == "off", "worker off", "retry/bounce worker on"))
	fmt.Fprintf(out, "  Follow-up reminders → %s\n", check(env("REMINDER_WORKER") == "off", "worker off", "due worker on"))
	fmt.Fprintf(out, "  Snooze return → %s\n", check(env("SNOOZE_WORKER") == "off", "worker off", "due worker on"))
	fmt.Fprintf(out, "  Provider alerts → %s\n", check(env("PROVIDER_NOTIFICATION_WORKER") == "off", "worker off", "durable push worker on"))
	certs := "manual (MAIL_TLS_CERT/KEY) or none"
	if env("MAIL_ACME_ENABLED") == "true" {
		certs = "automatic (Let's Encrypt" + check(env("MAIL_ACME_STAGING") == "true", " staging", "") + ")"
	}
	fmt.Fprintf(out, "  TLS certs    → %s\n", certs)
	if smtp == "console" {
		fmt.Fprintln(out, "  Console delivery is intended only for development and tests.")
	}
	fmt.Fprintln(out, "")
	fmt.Fprintln(out, "Push notifications:")
	push := "Disabled (no VAPID keys set)"
	if pushDisabled {
		push = "Disabled (WEB_PUSH_DISABLED=true)"
	} else if pushKeys {
		push = "Enabled (VAPID keys configured)"
	}
	fmt.Fprintf(out, "  %s\n", push)
	fmt.Fprintln(out, "")
}

func (runner *runner) self() string {
	if binary := env("CADUCEUS_GO_BINARY"); binary != "" {
		return binary
	}
	if runner.options.Self != "" {
		return runner.options.Self
	}
	executable, err := os.Executable()
	if err != nil {
		return "/usr/local/bin/caduceus-go"
	}
	return executable
}

func setDefault(name, value string) {
	if os.Getenv(name) == "" {
		_ = os.Setenv(name, value)
	}
}

// startPersistenceBridge prepares the data roots and the identity
// deployment, then starts `fylo-bridge` and waits until it binds.
func (runner *runner) startPersistenceBridge() (*child, error) {
	binary := runner.self()
	setDefault("FYLO_BRIDGE_ADDRESS", "127.0.0.1:9876")
	secret := make([]byte, 32)
	if _, err := rand.Read(secret); err != nil {
		return nil, err
	}
	setDefault("FYLO_BRIDGE_SECRET", hex.EncodeToString(secret))
	root := env("FYLO_ROOT")
	if root == "" {
		root = "/data/caduceus"
	}
	setDefault("FYLO_INCOMING_ROOT", root+"/.caduceus-incoming")
	if env("EXTERNAL_ACCOUNT_CREDENTIAL_KEYS") == "" || env("EXTERNAL_ACCOUNT_ACTIVE_KEY") == "" {
		return nil, errors.New("EXTERNAL_ACCOUNT_CREDENTIAL_KEYS and EXTERNAL_ACCOUNT_ACTIVE_KEY are required; see docs/CONFIGURATION.md")
	}
	// The image pre-creates the data directories, but a bind mount over /data
	// hides them; creating a missing one is safe.
	for _, directory := range []string{env("FYLO_ROOT"), env("FYLO_INCOMING_ROOT"), env("SESAME_DEPLOYMENT")} {
		if directory != "" {
			if err := os.MkdirAll(directory, 0o700); err != nil {
				return nil, err
			}
		}
	}
	// `auth init` is idempotent and refuses to touch an existing deployment,
	// so a restart never mints a new signing key.
	if env("SESAME_DEPLOYMENT") != "" {
		if code := spawn(runner.stdout, runner.stderr, binary, "auth", "init").result(); code != 0 {
			return nil, fmt.Errorf("Identity deployment initialization failed with code %d", code)
		}
	}
	bridge := spawn(runner.stdout, runner.stderr, binary, "fylo-bridge")
	if err := waitForBridge(bridge, env("FYLO_BRIDGE_ADDRESS"), 30*time.Second); err != nil {
		return nil, err
	}
	return bridge, nil
}

var tcpAddress = regexp.MustCompile(`^\[([^\]]+)\]:(\d+)$|^([^:]+):(\d+)$`)

func waitForBridge(bridge *child, address string, timeout time.Duration) error {
	match := tcpAddress.FindStringSubmatch(address)
	if match == nil {
		bridge.stop()
		return fmt.Errorf("Invalid Go FYLO owner address: %s", address)
	}
	host, port := match[1]+match[3], match[2]+match[4]
	if number, err := strconv.Atoi(port); err != nil || number < 1 || number > 65535 {
		bridge.stop()
		return fmt.Errorf("Invalid Go FYLO owner address: %s", address)
	}
	deadline := time.Now().Add(timeout)
	for time.Now().Before(deadline) {
		if bridge.startErr != nil {
			return fmt.Errorf("Go FYLO owner failed to start: %v", bridge.startErr)
		}
		if !bridge.running() {
			return fmt.Errorf("Go FYLO owner exited with code %d", bridge.result())
		}
		conn, err := net.DialTimeout("tcp", net.JoinHostPort(host, port), 500*time.Millisecond)
		if err == nil {
			_ = conn.Close()
			return nil
		}
		time.Sleep(100 * time.Millisecond)
	}
	bridge.kill(syscall.SIGTERM)
	return fmt.Errorf("Go FYLO owner did not bind %s within %dms", address, timeout.Milliseconds())
}

// ensureCollections creates every collection and bucket through the bridge,
// sequentially, as Fylo.createDb did on each open. Collections are created
// once by the process that starts the server; routes never create them.
func ensureCollections(ctx context.Context) error {
	client := bridgeclient.New(env("FYLO_BRIDGE_ADDRESS"), env("FYLO_BRIDGE_SECRET"))
	identity, err := client.Call(ctx, map[string]any{"op": "handshake"})
	if err != nil {
		return err
	}
	if settings.Value("FYLO_POSIX_ACCESS") == "true" {
		var handshake struct {
			Capabilities struct {
				Handshake bool `json:"handshake"`
			} `json:"capabilities"`
		}
		_ = json.Unmarshal(identity, &handshake)
		if !handshake.Capabilities.Handshake {
			return errors.New("FYLO_POSIX_ACCESS requires a Fylo binary with the issue-68 machine-access protocol. " +
				"Build the local FYLO checkout and set FYLO_BINARY to its dist-bin/fylo executable.")
		}
	}
	for _, name := range Collections {
		if _, err := client.Call(ctx, map[string]any{"op": "createCollection", "collection": name, "kind": "document"}); err != nil {
			return fmt.Errorf("create FYLO collection %s: %w", name, err)
		}
	}
	for _, name := range Buckets {
		if _, err := client.Call(ctx, map[string]any{"op": "createCollection", "collection": name, "kind": "file"}); err != nil {
			return fmt.Errorf("create FYLO collection %s: %w", name, err)
		}
	}
	return nil
}

func scriptRuntime() string {
	if runtime := env("CADUCEUS_SCRIPT_RUNTIME"); runtime != "" {
		return runtime
	}
	return "bun"
}

// runBootstrapIfConfigured runs the JavaScript create-admin script when a
// bootstrap is configured. It still owns the users and domains documents.
func (runner *runner) runBootstrapIfConfigured() error {
	multi := env("CADUCEUS_BOOTSTRAP_DOMAINS") != ""
	single := env("CADUCEUS_ADMIN_EMAIL") != "" && env("CADUCEUS_ADMIN_DOMAIN") != ""
	if !multi && !single {
		return nil
	}
	setDefault("CADUCEUS_BINARY", runner.self())
	if code := spawn(runner.stdout, runner.stderr, scriptRuntime(), "scripts/create-admin.mjs").result(); code != 0 {
		return fmt.Errorf("bootstrap (scripts/create-admin.mjs) failed with code %d", code)
	}
	return nil
}

var absoluteSetting = regexp.MustCompile(`(_ROOT|_FILE|_BINARY|_EXEC_PATH)$`)

// writeHandlerSettings hands Tachyon's handler processes, which get no
// application variables, the settings this process holds, with paths made
// absolute because a handler runs in its own working directory.
func (runner *runner) writeHandlerSettings() error {
	target, err := filepath.Abs(".env")
	if err != nil {
		return err
	}
	if existing, err := os.ReadFile(target); err == nil && !strings.HasPrefix(string(existing), settings.GeneratedMarker) {
		return fmt.Errorf("%s was not written by CADUCEUS; move it aside so the entry point can write the handler settings", target)
	}
	environment := map[string]string{}
	for _, entry := range os.Environ() {
		if index := strings.IndexByte(entry, '='); index > 0 {
			environment[entry[:index]] = entry[index+1:]
		}
	}
	for _, name := range settings.HandlerSettings {
		if value := environment[name]; value != "" && absoluteSetting.MatchString(name) {
			if absolute, err := filepath.Abs(value); err == nil {
				environment[name] = absolute
			}
		}
	}
	text, skipped := settings.Dotenv(environment)
	for _, name := range skipped {
		fmt.Fprintf(runner.stderr, "[runtime] %s spans several lines; set %s_FILE for the API handlers instead\n", name, name)
	}
	return os.WriteFile(target, []byte(text), 0o600)
}

func waitForAPI(ctx context.Context, apiURL string, timeout time.Duration) error {
	deadline := time.Now().Add(timeout)
	client := &http.Client{Timeout: time.Second}
	for time.Now().Before(deadline) {
		if ctx.Err() != nil {
			return ctx.Err()
		}
		response, err := client.Get(apiURL + "/health")
		if err == nil {
			_, _ = io.Copy(io.Discard, response.Body)
			response.Body.Close()
			if response.StatusCode >= 200 && response.StatusCode <= 299 {
				return nil
			}
		}
		time.Sleep(300 * time.Millisecond)
	}
	return fmt.Errorf("API server not reachable at %s after %dms", apiURL, timeout.Milliseconds())
}

func (runner *runner) operator(ctx context.Context, script string, args []string) int {
	bridge, err := runner.startPersistenceBridge()
	if err != nil {
		fmt.Fprintf(runner.stderr, "[runtime] %v\n", err)
		return exitFailure
	}
	defer bridge.stop()
	if err := ensureCollections(ctx); err != nil {
		fmt.Fprintf(runner.stderr, "[runtime] %v\n", err)
		return exitFailure
	}
	setDefault("CADUCEUS_BINARY", runner.self())
	return spawn(runner.stdout, runner.stderr, scriptRuntime(), append([]string{script}, args...)...).result()
}

func tachyonBinary() string {
	if binary := env("TACHYON_BIN"); binary != "" {
		return binary
	}
	return "ty"
}

// serveArguments answers the `ty serve` command line. Tachyon binds
// 127.0.0.1:8080 unless told otherwise and reads neither PORT nor HOST, so
// the entry point passes PORT as --port and a non-loopback HOST (the image
// sets 0.0.0.0) as --host with --allow-non-loopback. Arguments given to
// `serve` keep precedence.
func serveArguments(port string, args []string) []string {
	given := map[string]bool{}
	for _, arg := range args {
		given[strings.SplitN(arg, "=", 2)[0]] = true
	}
	command := []string{"serve"}
	if !given["--port"] {
		command = append(command, "--port", port)
	}
	host := env("HOST")
	if host != "" && !given["--host"] && !given["--hostname"] && !loopbackHost(host) {
		command = append(command, "--host", host)
		if !given["--allow-non-loopback"] {
			command = append(command, "--allow-non-loopback")
		}
	}
	return append(command, args...)
}

func loopbackHost(host string) bool {
	if host == "localhost" {
		return true
	}
	address := net.ParseIP(strings.Trim(host, "[]"))
	return address != nil && address.IsLoopback()
}

// Session is one `serve` run: the Tachyon child, the supervisor, and the
// workers whose metrics the status log reports.
type Session struct {
	runner *runner
	apiURL string

	mu         sync.Mutex
	requested  *int
	stopping   bool
	shutdown   chan struct{}
	once       sync.Once
	ty         *child
	supervisor *supervisor.Supervisor
	statusStop chan struct{}
	outbound   *tickworker.OutboundQueue
	snooze     *tickworker.Snooze
	reminders  *tickworker.FollowUpReminders
	provider   *tickworker.ProviderNotification
}

// APIURL answers the local API the background services reach.
func (session *Session) APIURL() string { return session.apiURL }

// Stdout answers the log destination.
func (session *Session) Stdout() io.Writer { return session.runner.stdout }

// Stderr answers the error log destination.
func (session *Session) Stderr() io.Writer { return session.runner.stderr }

// Supervisor answers the supervisor once services are registered.
func (session *Session) Supervisor() *supervisor.Supervisor {
	session.mu.Lock()
	defer session.mu.Unlock()
	return session.supervisor
}

// Shutdown stops the background services and then Tachyon. Repeated calls
// share one shutdown; the exit code only ever rises. It answers a channel
// closed when the shutdown is done.
func (session *Session) Shutdown(signal os.Signal, exitCode int) <-chan struct{} {
	session.mu.Lock()
	code := exitCode
	if session.requested != nil && *session.requested > code {
		code = *session.requested
	}
	session.requested = &code
	session.stopping = true
	session.mu.Unlock()
	session.once.Do(func() {
		go func() {
			defer close(session.shutdown)
			session.mu.Lock()
			if session.statusStop != nil {
				close(session.statusStop)
				session.statusStop = nil
			}
			current := session.supervisor
			session.mu.Unlock()
			if current != nil {
				current.Stop(context.Background())
			}
			if session.ty != nil && session.ty.running() {
				session.ty.kill(signal)
			}
		}()
	})
	return session.shutdown
}

func (session *Session) requestedCode() (int, bool) {
	session.mu.Lock()
	defer session.mu.Unlock()
	if session.requested == nil {
		return 0, false
	}
	return *session.requested, true
}

func (runner *runner) serve(ctx context.Context, args []string) int {
	runner.summary()
	hasIdentity := env("SESAME_DEPLOYMENT") != "" && env("SESAME_BINARY") != ""
	if !hasIdentity || env("INBOUND_WEBHOOK_SECRET") == "" {
		fmt.Fprintln(runner.stderr, "ERROR: SESAME_BINARY, SESAME_DEPLOYMENT, and INBOUND_WEBHOOK_SECRET are required.")
		fmt.Fprintln(runner.stderr, "Set them as environment variables before starting CADUCEUS.")
		return exitFailure
	}
	bridge, err := runner.startPersistenceBridge()
	if err != nil {
		fmt.Fprintf(runner.stderr, "[runtime] %v\n", err)
		return exitFailure
	}
	prepare := func() error {
		if err := ensureCollections(ctx); err != nil {
			return err
		}
		if err := runner.runBootstrapIfConfigured(); err != nil {
			return err
		}
		return runner.writeHandlerSettings()
	}
	if err := prepare(); err != nil {
		fmt.Fprintf(runner.stderr, "[runtime] %v\n", err)
		bridge.stop()
		return exitFailure
	}

	port := env("PORT")
	if port == "" {
		port = "8080"
	}
	session := &Session{runner: runner, apiURL: "http://127.0.0.1:" + port, shutdown: make(chan struct{})}

	// Start the standalone Tachyon runtime as a child process.
	session.ty = spawn(runner.stdout, runner.stderr, tachyonBinary(), serveArguments(port, args)...)
	if session.ty.startErr != nil {
		fmt.Fprintf(runner.stderr, "[runtime] Tachyon failed to start: %v\n", session.ty.startErr)
		one := 1
		session.requested = &one
	}

	go func() {
		<-bridge.done
		bridge.mu.Lock()
		code, signaled, name := bridge.code, bridge.signaled, bridge.signal
		bridge.mu.Unlock()
		if _, requested := session.requestedCode(); code == 0 && !signaled && requested {
			return
		}
		reason := name
		if reason == "" && code != 0 {
			reason = strconv.Itoa(code)
		}
		if reason == "" {
			reason = "unknown"
		}
		fmt.Fprintf(runner.stderr, "[runtime] Go FYLO owner exited (%s)\n", reason)
		session.Shutdown(syscall.SIGTERM, 1)
	}()

	signals := make(chan os.Signal, 2)
	signal.Notify(signals, syscall.SIGINT, syscall.SIGTERM)
	defer signal.Stop(signals)
	go func() {
		seen := map[os.Signal]bool{}
		handle := func(received os.Signal) {
			if seen[received] {
				// The second identical signal kills, as Node's default
				// handler did once the one-shot listener had run.
				session.ty.kill(os.Kill)
				bridge.kill(os.Kill)
				os.Exit(128 + int(received.(syscall.Signal)))
			}
			seen[received] = true
			session.Shutdown(received, 0)
		}
		done := ctx.Done()
		for {
			select {
			case received := <-signals:
				handle(received)
			case <-done:
				// main's NotifyContext sees the same signal; forward the one
				// that arrived rather than assuming SIGTERM.
				done = nil
				select {
				case received := <-signals:
					handle(received)
				case <-time.After(50 * time.Millisecond):
					seen[syscall.SIGTERM] = true
					session.Shutdown(syscall.SIGTERM, 0)
				}
			case <-session.ty.done:
				return
			}
		}
	}()

	// Background helpers reach the API over localhost, so the API server
	// stays the single writer of the store.
	go session.startBackground(ctx)

	childCode := session.ty.result()
	exitCode := childCode
	if requested, ok := session.requestedCode(); ok {
		exitCode = requested
	}
	<-session.Shutdown(syscall.SIGTERM, exitCode)
	bridge.stop()
	session.logStatus("stopped")
	requested, _ := session.requestedCode()
	return requested
}

func (session *Session) startBackground(ctx context.Context) {
	runner := session.runner
	timeout := runner.options.APITimeout
	if timeout == 0 {
		// A cold `ty serve` bundles the client and compiles each Go handler
		// before /health answers, which takes minutes, not the 30 s the
		// JavaScript server needed.
		timeout = milliseconds(settings.EnvPositive("CADUCEUS_API_START_TIMEOUT_MS", 900_000))
	}
	fail := func(err error) {
		fmt.Fprintf(runner.stderr, "[mail] background mail services failed to start: %v\n", err)
		session.Shutdown(syscall.SIGTERM, 1)
	}
	if err := waitForAPI(ctx, session.apiURL, timeout); err != nil {
		fail(err)
		return
	}
	current := supervisor.New(supervisor.Options{
		OnRequiredFailure: func(name string, err error) {
			fmt.Fprintf(runner.stderr, "[mail] required service %q failed: %s\n", name, err.Error())
			session.Shutdown(syscall.SIGTERM, 1)
		},
	})
	services := productionServices
	if runner.options.Services != nil {
		services = runner.options.Services
	}
	for _, service := range services(session) {
		if err := current.Register(service); err != nil {
			fail(err)
			return
		}
	}
	session.mu.Lock()
	if session.stopping {
		session.mu.Unlock()
		return
	}
	session.supervisor = current
	session.mu.Unlock()
	if err := current.Start(ctx); err != nil {
		fail(err)
		return
	}
	session.logStatus("ready")
	interval := settings.EnvNonNegative("MAIL_STATUS_INTERVAL_MS", 300_000)
	if interval <= 0 {
		return
	}
	session.mu.Lock()
	if session.stopping {
		session.mu.Unlock()
		return
	}
	stop := make(chan struct{})
	session.statusStop = stop
	session.mu.Unlock()
	go func() {
		ticker := time.NewTicker(time.Duration(interval * float64(time.Millisecond)))
		defer ticker.Stop()
		for {
			select {
			case <-stop:
				return
			case <-ticker.C:
				session.logStatus("periodic")
			}
		}
	}()
}

type statusRecord struct {
	Event             string                       `json:"event"`
	At                string                       `json:"at"`
	Runtime           any                          `json:"runtime"`
	OutboundQueue     *tickworker.OutboundSnapshot `json:"outboundQueue,omitempty"`
	Snooze            *supervisor.WorkerSnapshot   `json:"snooze,omitempty"`
	FollowUpReminders *tickworker.ReminderSnapshot `json:"followUpReminders,omitempty"`
	ProviderNewMail   *supervisor.WorkerSnapshot   `json:"providerNewMail,omitempty"`
}

// logStatus emits the stable, secret-free status record operators collect
// from container logs.
func (session *Session) logStatus(event string) {
	session.mu.Lock()
	record := statusRecord{Event: event, At: supervisor.Timestamp(time.Now())}
	current, outbound, snooze, reminders, provider := session.supervisor, session.outbound, session.snooze, session.reminders, session.provider
	session.mu.Unlock()
	if current != nil {
		record.Runtime = current.Snapshot()
	} else {
		record.Runtime = struct {
			State      string   `json:"state"`
			Components struct{} `json:"components"`
		}{State: "not-started"}
	}
	if outbound != nil {
		snapshot := outbound.Snapshot()
		record.OutboundQueue = &snapshot
	}
	if snooze != nil {
		snapshot := snooze.Snapshot()
		record.Snooze = &snapshot
	}
	if reminders != nil {
		snapshot := reminders.Snapshot()
		record.FollowUpReminders = &snapshot
	}
	if provider != nil {
		snapshot := provider.Snapshot()
		record.ProviderNewMail = &snapshot
	}
	encoded, err := supervisor.Marshal(record)
	if err != nil {
		return
	}
	fmt.Fprintf(session.runner.stdout, "[mail:status] %s\n", encoded)
}
