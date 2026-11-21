package workers

import (
	"encoding/json"
	"os"
	"os/exec"
	"path/filepath"
	"slices"
	"strings"
	"testing"
)

const workerSwitch = `scheduling.WorkerSwitch{ApplicationJson: `

type lambdaProfile struct {
	Server struct {
		Host string `json:"host"`
	} `json:"server"`
	Workers struct {
		Enabled *bool `json:"enabled"`
	} `json:"workers"`
}

func repositoryRoot(t *testing.T) string {
	t.Helper()
	root, err := filepath.Abs(filepath.Join("..", ".."))
	if err != nil {
		t.Fatalf("repository root: %v", err)
	}
	return root
}

func repositoryFile(t *testing.T, relative string) string {
	t.Helper()
	text, err := os.ReadFile(filepath.Join(repositoryRoot(t), relative))
	if err != nil {
		t.Fatalf("read %s: %v", relative, err)
	}
	return string(text)
}

func wantSwitch(t *testing.T, relative string) {
	t.Helper()
	if !strings.Contains(repositoryFile(t, relative), workerSwitch) {
		t.Fatalf("%s does not run its work through %s...}", relative, workerSwitch)
	}
}

func writtenProfile(t *testing.T) lambdaProfile {
	t.Helper()
	app := t.TempDir()
	if err := os.MkdirAll(filepath.Join(app, "server"), 0o755); err != nil {
		t.Fatalf("make server folder: %v", err)
	}
	if err := os.WriteFile(filepath.Join(app, "server", "application.json"), []byte("{}\n"), 0o644); err != nil {
		t.Fatalf("write application.json: %v", err)
	}
	script := repositoryFile(t, filepath.Join("deploy", "lambda", "write-profile.sh"))
	command := exec.Command("sh", "-s", app, "directory", "true")
	command.Stdin = strings.NewReader(script)
	if output, err := command.CombinedOutput(); err != nil {
		t.Fatalf("write-profile.sh: %v\n%s", err, output)
	}
	text, err := os.ReadFile(filepath.Join(app, "server", "application-lambda.json"))
	if err != nil {
		t.Fatalf("read application-lambda.json: %v", err)
	}
	profile := lambdaProfile{}
	if err := json.Unmarshal(text, &profile); err != nil {
		t.Fatalf("application-lambda.json is not JSON: %v\n%s", err, text)
	}
	return profile
}

func TestLambdaProfileTurnsWorkersOff(t *testing.T) {
	profile := writtenProfile(t)
	if profile.Workers.Enabled == nil || *profile.Workers.Enabled {
		t.Fatalf("workers.enabled is %v, want false", profile.Workers.Enabled)
	}
}

func TestLambdaProfileKeepsTheLoopbackHost(t *testing.T) {
	profile := writtenProfile(t)
	if profile.Server.Host != "127.0.0.1" {
		t.Fatalf("server.host is %q, want 127.0.0.1", profile.Server.Host)
	}
}

func TestEveryWorkerIsListed(t *testing.T) {
	found, err := filepath.Glob(filepath.Join(repositoryRoot(t), "apps", "*", "server", "workers", "*.go"))
	if err != nil {
		t.Fatalf("glob: %v", err)
	}
	want := []string{
		"channels/server/workers/outbox-relay.go",
		"devicebox/server/workers/devicebox-expiry.go",
		"devicebox/server/workers/outbox-relay.go",
		"directory/server/workers/directory-removals.go",
		"directory/server/workers/outbox-relay.go",
		"mail-in/server/workers/outbox-relay.go",
		"mail-out/server/workers/deliver.go",
		"messaging/server/workers/changes-cleanup.go",
		"messaging/server/workers/outbox-relay.go",
		"notify/server/workers/notify-jobs.go",
		"realtime/server/workers/sweep.go",
	}
	prefix := filepath.Join(repositoryRoot(t), "apps") + string(filepath.Separator)
	got := strings.Split(strings.ReplaceAll(strings.Join(found, "\n"), prefix, ""), "\n")
	if !slices.Equal(got, want) {
		t.Fatalf("workers are %v; a new worker needs a case in this file (Lambda apps run it through the switch), want %v", got, want)
	}
}

func TestChannelsOutboxRelayUsesTheSwitch(t *testing.T) {
	wantSwitch(t, "apps/channels/server/workers/outbox-relay.go")
}

func TestDeviceboxExpiryUsesTheSwitch(t *testing.T) {
	wantSwitch(t, "apps/devicebox/server/workers/devicebox-expiry.go")
}

func TestDeviceboxOutboxRelayUsesTheSwitch(t *testing.T) {
	wantSwitch(t, "apps/devicebox/server/workers/outbox-relay.go")
}

func TestDirectoryRemovalsUsesTheSwitch(t *testing.T) {
	wantSwitch(t, "apps/directory/server/workers/directory-removals.go")
}

func TestDirectoryOutboxRelayUsesTheSwitch(t *testing.T) {
	wantSwitch(t, "apps/directory/server/workers/outbox-relay.go")
}

func TestMailInOutboxRelayUsesTheSwitch(t *testing.T) {
	wantSwitch(t, "apps/mail-in/server/workers/outbox-relay.go")
}

func TestMailOutDeliverUsesTheSwitch(t *testing.T) {
	wantSwitch(t, "apps/mail-out/server/workers/deliver.go")
}

func TestMessagingChangesCleanupUsesTheSwitch(t *testing.T) {
	wantSwitch(t, "apps/messaging/server/workers/changes-cleanup.go")
}

func TestMessagingOutboxRelayUsesTheSwitch(t *testing.T) {
	wantSwitch(t, "apps/messaging/server/workers/outbox-relay.go")
}

func TestNotifyJobsUsesTheSwitch(t *testing.T) {
	wantSwitch(t, "apps/notify/server/workers/notify-jobs.go")
}
