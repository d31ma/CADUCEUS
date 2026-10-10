package supervisor

import (
	"context"
	"errors"
	"strings"
	"sync"
	"testing"
	"time"
)

func fixedClock() func() time.Time {
	return func() time.Time { return time.Date(2026, 10, 10, 12, 0, 0, 0, time.UTC) }
}

func service(name string, optional bool, log *[]string, startErr error) Service {
	return Service{
		Name:     name,
		Optional: optional,
		Start: func(context.Context) (StopFunc, error) {
			*log = append(*log, "start:"+name)
			if startErr != nil {
				return nil, startErr
			}
			return func(context.Context) error {
				*log = append(*log, "stop:"+name)
				return nil
			}, nil
		},
	}
}

func TestStartsInOrderAndStopsInReverse(t *testing.T) {
	var log []string
	supervisor := New(Options{Now: fixedClock()})
	for _, name := range []string{"a", "b", "c"} {
		if err := supervisor.Register(service(name, false, &log, nil)); err != nil {
			t.Fatal(err)
		}
	}
	if err := supervisor.Start(context.Background()); err != nil {
		t.Fatal(err)
	}
	if got := supervisor.Snapshot().State; got != StateReady {
		t.Fatalf("state = %s", got)
	}
	supervisor.Stop(context.Background())
	supervisor.Stop(context.Background())
	want := "start:a start:b start:c stop:c stop:b stop:a"
	if got := strings.Join(log, " "); got != want {
		t.Fatalf("log = %q, want %q", got, want)
	}
	snapshot := supervisor.Snapshot()
	if snapshot.State != StateStopped || snapshot.StoppedAt != "2026-10-10T12:00:00.000Z" {
		t.Fatalf("snapshot = %+v", snapshot)
	}
}

func TestRequiredStartupFailureRollsBack(t *testing.T) {
	var log []string
	supervisor := New(Options{})
	_ = supervisor.Register(service("a", false, &log, nil))
	_ = supervisor.Register(service("b", false, &log, errors.New("bind failed")))
	_ = supervisor.Register(service("c", false, &log, nil))
	err := supervisor.Start(context.Background())
	if err == nil || err.Error() != "bind failed" {
		t.Fatalf("err = %v", err)
	}
	if got := strings.Join(log, " "); got != "start:a start:b stop:a" {
		t.Fatalf("log = %q", got)
	}
	snapshot := supervisor.Snapshot()
	if snapshot.State != StateFailed {
		t.Fatalf("state = %s", snapshot.State)
	}
	if b := snapshot.Components.ByName["b"]; b.State != ComponentFailed || b.Error != "bind failed" {
		t.Fatalf("b = %+v", b)
	}
	if c := snapshot.Components.ByName["c"]; c.State != ComponentPending {
		t.Fatalf("c = %+v", c)
	}
	supervisor.Stop(context.Background())
	if got := supervisor.Snapshot().State; got != StateFailed {
		t.Fatalf("terminal state = %s, want failed", got)
	}
}

func TestOptionalStartupFailureDegrades(t *testing.T) {
	var log []string
	supervisor := New(Options{})
	_ = supervisor.Register(service("acme-renewal", true, &log, errors.New("no network")))
	_ = supervisor.Register(service("inbound-smtp", false, &log, nil))
	if err := supervisor.Start(context.Background()); err != nil {
		t.Fatal(err)
	}
	if got := supervisor.Snapshot().State; got != StateDegraded {
		t.Fatalf("state = %s", got)
	}
}

func TestReportFailure(t *testing.T) {
	var log []string
	var mu sync.Mutex
	failures := make(chan string, 1)
	supervisor := New(Options{OnRequiredFailure: func(name string, err error) {
		mu.Lock()
		defer mu.Unlock()
		failures <- name + ":" + err.Error()
	}})
	_ = supervisor.Register(service("optional", true, &log, nil))
	_ = supervisor.Register(service("required", false, &log, nil))
	if err := supervisor.Start(context.Background()); err != nil {
		t.Fatal(err)
	}
	if err := supervisor.ReportFailure("missing", errors.New("x")); err == nil {
		t.Fatal("unknown service accepted")
	}
	_ = supervisor.ReportFailure("optional", errors.New("renewal failed"))
	if got := supervisor.Snapshot().State; got != StateDegraded {
		t.Fatalf("state = %s", got)
	}
	_ = supervisor.ReportFailure("required", errors.New("socket closed"))
	_ = supervisor.ReportFailure("required", errors.New("again"))
	select {
	case got := <-failures:
		if got != "required:socket closed" {
			t.Fatalf("failure = %q", got)
		}
	case <-time.After(time.Second):
		t.Fatal("OnRequiredFailure not called")
	}
	select {
	case got := <-failures:
		t.Fatalf("second failure reported: %q", got)
	case <-time.After(50 * time.Millisecond):
	}
	if got := supervisor.Snapshot().State; got != StateFailed {
		t.Fatalf("state = %s", got)
	}
	// Failed services are still stopped, as the JavaScript supervisor did.
	supervisor.Stop(context.Background())
	if got := strings.Join(log, " "); !strings.Contains(got, "stop:required") || !strings.Contains(got, "stop:optional") {
		t.Fatalf("log = %q", got)
	}
}

func TestRegisterValidation(t *testing.T) {
	var log []string
	supervisor := New(Options{})
	if err := supervisor.Register(Service{Name: "  "}); err == nil {
		t.Fatal("blank name accepted")
	}
	_ = supervisor.Register(service("a", false, &log, nil))
	if err := supervisor.Register(service("a", false, &log, nil)); err == nil || err.Error() != `Service "a" is already registered` {
		t.Fatalf("err = %v", err)
	}
	_ = supervisor.Start(context.Background())
	if err := supervisor.Register(service("b", false, &log, nil)); err == nil {
		t.Fatal("late registration accepted")
	}
	if err := supervisor.Start(context.Background()); err == nil || err.Error() != "Cannot start services while supervisor is ready" {
		t.Fatalf("err = %v", err)
	}
}

func TestSnapshotJSONKeepsRegistrationOrder(t *testing.T) {
	var log []string
	supervisor := New(Options{Now: fixedClock()})
	_ = supervisor.Register(service("zeta", false, &log, nil))
	_ = supervisor.Register(service("alpha", true, &log, nil))
	encoded, err := Marshal(supervisor.Snapshot())
	if err != nil {
		t.Fatal(err)
	}
	want := `{"state":"idle","components":{"zeta":{"required":true,"state":"pending"},"alpha":{"required":false,"state":"pending"}}}`
	if string(encoded) != want {
		t.Fatalf("json = %s", encoded)
	}
}

func TestPeriodicWorkerDoesNotOverlap(t *testing.T) {
	release := make(chan struct{})
	entered := make(chan struct{}, 4)
	worker, err := NewPeriodicWorker("tick", time.Hour, func(context.Context) error {
		entered <- struct{}{}
		<-release
		return errors.New("HTTP 500")
	})
	if err != nil {
		t.Fatal(err)
	}
	result := make(chan bool, 1)
	go func() { result <- worker.RunNow() }()
	<-entered
	if worker.RunNow() {
		t.Fatal("overlapping run succeeded")
	}
	if snapshot := worker.Snapshot(); !snapshot.InFlight || snapshot.Skipped != 1 {
		t.Fatalf("snapshot = %+v", snapshot)
	}
	close(release)
	if <-result {
		t.Fatal("failed run reported success")
	}
	snapshot := worker.Snapshot()
	if snapshot.Runs != 1 || snapshot.Failed != 1 || snapshot.LastError != "HTTP 500" || snapshot.LastDurationMs == nil || snapshot.InFlight {
		t.Fatalf("snapshot = %+v", snapshot)
	}
}

func TestPeriodicWorkerTicksAndStops(t *testing.T) {
	var mu sync.Mutex
	count := 0
	worker, _ := NewPeriodicWorker("tick", 5*time.Millisecond, func(context.Context) error {
		mu.Lock()
		count++
		mu.Unlock()
		return nil
	})
	stop, err := worker.Start()
	if err != nil {
		t.Fatal(err)
	}
	if _, err := worker.Start(); err == nil {
		t.Fatal("double start accepted")
	}
	deadline := time.Now().Add(2 * time.Second)
	for {
		mu.Lock()
		current := count
		mu.Unlock()
		if current >= 2 {
			break
		}
		if time.Now().After(deadline) {
			t.Fatal("ticker did not run")
		}
		time.Sleep(time.Millisecond)
	}
	if err := stop(context.Background()); err != nil {
		t.Fatal(err)
	}
	snapshot := worker.Snapshot()
	if snapshot.Running || snapshot.Succeeded < 2 || snapshot.LastError != "" {
		t.Fatalf("snapshot = %+v", snapshot)
	}
	encoded, _ := Marshal(WorkerSnapshot{Name: "n"})
	if string(encoded) != `{"name":"n","running":false,"inFlight":false,"runs":0,"succeeded":0,"failed":0,"skipped":0}` {
		t.Fatalf("json = %s", encoded)
	}
}

func TestPeriodicWorkerValidation(t *testing.T) {
	if _, err := NewPeriodicWorker("", time.Second, nil); err == nil {
		t.Fatal("blank name accepted")
	}
	if _, err := NewPeriodicWorker("x", 0, nil); err == nil {
		t.Fatal("zero interval accepted")
	}
}
