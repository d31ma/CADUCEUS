package main

import (
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"slices"
	"sync"
	"testing"
	"time"
)

type orderedLambda struct {
	mutex sync.Mutex
	order []string
	hold  chan struct{}
}

func (lambda *orderedLambda) serve(writer http.ResponseWriter, request *http.Request) {
	var event HttpEvent
	_ = json.NewDecoder(request.Body).Decode(&event)
	lambda.mutex.Lock()
	lambda.order = append(lambda.order, event.RequestContext.Http.Method+" "+event.RawPath)
	first := len(lambda.order) == 1
	lambda.mutex.Unlock()
	if first {
		<-lambda.hold
	}
	_, _ = writer.Write([]byte(`{"statusCode":200,"body":"ok"}`))
}

func (lambda *orderedLambda) calls() []string {
	lambda.mutex.Lock()
	defer lambda.mutex.Unlock()
	return slices.Clone(lambda.order)
}

type testClock struct {
	mutex sync.Mutex
	now   time.Time
}

func (clock *testClock) Now() time.Time {
	clock.mutex.Lock()
	defer clock.mutex.Unlock()
	return clock.now
}

func (clock *testClock) Add(step time.Duration) {
	clock.mutex.Lock()
	defer clock.mutex.Unlock()
	clock.now = clock.now.Add(step)
}

type queueRun struct {
	t       *testing.T
	lambda  *orderedLambda
	invoker LambdaInvoker
	group   sync.WaitGroup
}

func newQueueRun(t *testing.T, clock *testClock) *queueRun {
	lambda := &orderedLambda{hold: make(chan struct{})}
	server := httptest.NewServer(http.HandlerFunc(lambda.serve))
	t.Cleanup(server.Close)
	invoker := LambdaInvoker{
		Endpoint: server.URL,
		Client:   server.Client(),
		Signer:   Signer{Credentials: Credentials{AccessKey: "test", SecretKey: "secret", Region: "us-east-1"}, Service: "lambda"},
		Now:      clock.Now,
		Limits:   ConcurrencyLimits{}.Of([]string{"directory"}, []string{"directory"}, "mesail-", 1000),
	}
	return &queueRun{t: t, lambda: lambda, invoker: invoker}
}

func (run *queueRun) start(method string, path string) {
	run.group.Add(1)
	go func() {
		defer run.group.Done()
		event := HttpEvent{RawPath: path, RequestContext: EventContext{Http: EventMethod{Method: method, Path: path}}}
		if _, err := run.invoker.Invoke(context.Background(), "mesail-directory", event); err != nil {
			run.t.Errorf("%s %s: %v", method, path, err)
		}
	}()
	time.Sleep(100 * time.Millisecond)
}

func (run *queueRun) finish() []string {
	close(run.lambda.hold)
	run.group.Wait()
	return run.lambda.calls()
}

func TestUserRequestsGoBeforeQueuedBackgroundCalls(t *testing.T) {
	run := newQueueRun(t, &testClock{now: time.Date(2026, 11, 12, 5, 0, 0, 0, time.UTC)})
	run.start(http.MethodGet, "/api/v1/directory/people/first")
	run.start(http.MethodPost, "/api/v1/directory/jobs/removals")
	run.start(http.MethodPost, "/api/v1/directory/outbox/relay")
	run.start(http.MethodPost, "/api/v1/directory/outbox/dead/requeue")
	run.start(http.MethodGet, "/api/v1/directory/people/second")
	got := run.finish()
	want := []string{
		"GET /api/v1/directory/people/first",
		"GET /api/v1/directory/people/second",
		"POST /api/v1/directory/jobs/removals",
		"POST /api/v1/directory/outbox/relay",
		"POST /api/v1/directory/outbox/dead/requeue",
	}
	if !slices.Equal(got, want) {
		t.Fatalf("got %q", got)
	}
}

func TestBackgroundCallThatWaitedOverSixtySecondsGoesNextOnce(t *testing.T) {
	clock := &testClock{now: time.Date(2026, 11, 12, 5, 0, 0, 0, time.UTC)}
	run := newQueueRun(t, clock)
	run.start(http.MethodGet, "/api/v1/directory/people/first")
	run.start(http.MethodPost, "/api/v1/directory/jobs/removals")
	run.start(http.MethodPost, "/api/v1/directory/outbox/relay")
	clock.Add(61 * time.Second)
	run.start(http.MethodGet, "/api/v1/directory/people/second")
	run.start(http.MethodGet, "/api/v1/directory/people/third")
	got := run.finish()
	want := []string{
		"GET /api/v1/directory/people/first",
		"POST /api/v1/directory/jobs/removals",
		"GET /api/v1/directory/people/second",
		"POST /api/v1/directory/outbox/relay",
		"GET /api/v1/directory/people/third",
	}
	if !slices.Equal(got, want) {
		t.Fatalf("got %q", got)
	}
}

func TestBackgroundCallUnderSixtySecondsWaitsForUserRequests(t *testing.T) {
	clock := &testClock{now: time.Date(2026, 11, 12, 5, 0, 0, 0, time.UTC)}
	run := newQueueRun(t, clock)
	run.start(http.MethodGet, "/api/v1/directory/people/first")
	run.start(http.MethodPost, "/api/v1/directory/jobs/removals")
	clock.Add(59 * time.Second)
	run.start(http.MethodGet, "/api/v1/directory/people/second")
	run.start(http.MethodGet, "/api/v1/directory/people/third")
	got := run.finish()
	want := []string{
		"GET /api/v1/directory/people/first",
		"GET /api/v1/directory/people/second",
		"GET /api/v1/directory/people/third",
		"POST /api/v1/directory/jobs/removals",
	}
	if !slices.Equal(got, want) {
		t.Fatalf("got %q", got)
	}
}

func TestAWaiterThatGivesUpLeavesTheQueue(t *testing.T) {
	start := time.Date(2026, 11, 12, 5, 0, 0, 0, time.UTC)
	gate := FunctionGate{}.New(1)
	holder := gate.Enter(false, start)
	quitter := gate.Enter(false, start)
	stayer := gate.Enter(true, start)
	gate.Leave(quitter, start)
	gate.Release(start)
	select {
	case <-stayer.Ready:
	default:
		t.Fatal("the background call must get the slot once the user request left")
	}
	if holder.Held() != 1 || quitter.Held() != 0 || len(gate.users) != 0 || gate.busy != 1 {
		t.Fatalf("holder %d, quitter %d, users %d, busy %d", holder.Held(), quitter.Held(), len(gate.users), gate.busy)
	}
}

func TestAWaiterThatGivesUpAfterItsTurnCameFreesTheSlot(t *testing.T) {
	start := time.Date(2026, 11, 12, 5, 0, 0, 0, time.UTC)
	gate := FunctionGate{}.New(1)
	holder := gate.Enter(false, start)
	gate.Leave(holder, start)
	if !gate.TryEnter() || gate.TryEnter() {
		t.Fatalf("busy %d after the holder left", gate.busy)
	}
}

func TestACancelledCallerGetsItsErrorAndKeepsNoSlot(t *testing.T) {
	gate := FunctionGate{}.New(1)
	release, _ := QueuedEntry{}.Take(context.Background(), gate, time.Now)
	waiting, cancel := context.WithTimeout(context.Background(), 20*time.Millisecond)
	defer cancel()
	_, err := QueuedEntry{Background: true}.Take(waiting, gate, time.Now)
	release()
	if err != context.DeadlineExceeded || gate.busy != 0 || len(gate.background) != 0 {
		t.Fatalf("err %v, busy %d, background %d", err, gate.busy, len(gate.background))
	}
}
