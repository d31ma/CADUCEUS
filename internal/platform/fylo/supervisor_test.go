package fylo

import (
	"context"
	"errors"
	"os"
	"os/exec"
	"testing"
	"time"
)

func TestStopChildRemainsBoundedWhenProcessReapingStalls(t *testing.T) {
	command := exec.Command(os.Args[0], "-test.run=TestStalledChildHelper")
	command.Env = append(os.Environ(), "CADUCEUS_STALLED_CHILD_HELPER=1")
	if err := command.Start(); err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() {
		_ = command.Process.Kill()
		_ = command.Wait()
	})

	process := &childProcess{
		command: command,
		done:    make(chan error),
	}
	supervisor := New(Options{})
	supervisor.setChild(process)

	ctx, cancel := context.WithTimeout(context.Background(), 20*time.Millisecond)
	defer cancel()
	started := time.Now()
	err := supervisor.stopChild(ctx, process, true)

	if !errors.Is(err, context.DeadlineExceeded) {
		t.Fatalf("stopChild error = %v, want deadline exceeded", err)
	}
	if elapsed := time.Since(started); elapsed > 500*time.Millisecond {
		t.Fatalf("stopChild exceeded its bound: %s", elapsed)
	}
	if supervisor.currentChild() != nil {
		t.Fatal("timed-out child remained authoritative")
	}
}

func TestCallDeadlineIncludesTimeWaitingForSerialization(t *testing.T) {
	supervisor := New(Options{})
	if err := supervisor.acquire(context.Background()); err != nil {
		t.Fatal(err)
	}
	defer supervisor.release()

	ctx, cancel := context.WithTimeout(context.Background(), 20*time.Millisecond)
	defer cancel()
	started := time.Now()
	_, err := supervisor.Call(ctx, map[string]any{"op": "getDoc"})

	var operation *OperationError
	if !errors.As(err, &operation) || operation.Code != "ETIMEOUT" || operation.Ambiguous {
		t.Fatalf("Call error = %v, want unambiguous ETIMEOUT", err)
	}
	if elapsed := time.Since(started); elapsed > 500*time.Millisecond {
		t.Fatalf("Call exceeded its queue bound: %s", elapsed)
	}
}

func TestMutationClassificationIncludesDurableQueueSettlements(t *testing.T) {
	for _, operation := range []string{"queuePublish", "queueAck", "queueNack", "queueExtend"} {
		if !IsMutation(operation) {
			t.Fatalf("%s must be treated as an ambiguous durable mutation", operation)
		}
	}
	for _, operation := range []string{"queueClaim", "queueStats", "queueDeadLetters"} {
		if IsMutation(operation) {
			t.Fatalf("%s must remain retryable", operation)
		}
	}
}

func TestIdentityRejectsLockedDependencyDrift(t *testing.T) {
	supervisor := New(Options{
		ExpectedVersion:      "26.33.02",
		ExpectedDependencies: map[string]string{"chex": "26.32.02", "ttid": "26.32.03"},
		MaxRequestBytes:      DefaultMaxRequestBytes,
		MaxResponseBytes:     DefaultMaxResponseBytes,
	})
	identity := Identity{
		RuntimeVersion:  "26.33.02",
		ProtocolVersion: 1,
		Commit:          "e236162508f6789f980fa7ca136a6671938030cb",
		BuildTarget:     platformTarget(),
		BuildKind:       "release",
		Dependencies: map[string]Dependency{
			"chex": {RequiredVersion: "26.32.01", Available: true},
			"ttid": {RequiredVersion: "26.32.03", Available: true},
		},
		Machine: MachineContract{
			Framing: "ndjson", Encoding: "utf-8", Delimiter: "LF",
			MaxRequestBytes: DefaultMaxRequestBytes, MaxResponseBytes: DefaultMaxResponseBytes,
			DuplicateKeys: "rejected",
		},
		Capabilities: map[string]any{
			"handshake": true, "exclusiveRoot": true, "queryPagination": true,
		},
	}
	if err := supervisor.validateIdentity(identity); err == nil ||
		err.Error() != `FYLO dependency chex version "26.32.01" does not match "26.32.02"` {
		t.Fatalf("validateIdentity error = %v", err)
	}
}

func TestStalledChildHelper(t *testing.T) {
	if os.Getenv("CADUCEUS_STALLED_CHILD_HELPER") != "1" {
		return
	}
	time.Sleep(30 * time.Second)
}
