package main

import (
	"context"
	"encoding/json"
	"io"
	"net/http"
	"net/http/httptest"
	"slices"
	"strings"
	"sync"
	"sync/atomic"
	"testing"
	"time"
)

type cycleLambdas struct {
	router      string
	innerStatus atomic.Int32
}

func (lambdas *cycleLambdas) serve(writer http.ResponseWriter, request *http.Request) {
	var event HttpEvent
	body, _ := io.ReadAll(request.Body)
	_ = json.Unmarshal(body, &event)
	next := map[string]string{"/api/v1/messaging/messages": "/api/v1/mail-out/events", "/api/v1/mail-out/events": "/api/v1/messaging/events"}[event.RawPath]
	if next != "" {
		status := lambdas.push(next)
		if event.RawPath == "/api/v1/mail-out/events" {
			lambdas.innerStatus.Store(int32(status))
		}
	}
	_, _ = writer.Write([]byte(`{"statusCode":200,"body":"ok"}`))
}

func (lambdas *cycleLambdas) push(path string) int {
	response, err := http.Post(lambdas.router+path, "application/json", strings.NewReader("{}"))
	if err != nil {
		return 0
	}
	defer response.Body.Close()
	return response.StatusCode
}

func cycleSettings(endpoint string) Settings {
	return Settings{
		Listen:         ":0",
		Services:       []string{"messaging", "mail-out"},
		SingleWriters:  []string{"messaging", "mail-out"},
		FunctionPrefix: "mesail-",
		LambdaEndpoint: endpoint,
		Credentials:    Credentials{AccessKey: "test", SecretKey: "test", Region: "us-east-1"},
		Timeout:        5 * time.Second,
		BodyLimit:      1024,
		RateLimit:      100,
	}
}

func TestEventPushBackIntoABusyWriterIsRefusedAtOnce(t *testing.T) {
	lambdas := &cycleLambdas{}
	functions := httptest.NewServer(http.HandlerFunc(lambdas.serve))
	t.Cleanup(functions.Close)
	router := httptest.NewServer(handlerFor(t, cycleSettings(functions.URL)))
	t.Cleanup(router.Close)
	lambdas.router = router.URL
	client := &http.Client{Timeout: 3 * time.Second}
	started := time.Now()
	response, err := client.Post(router.URL+"/api/v1/messaging/messages", "application/json", strings.NewReader("{}"))
	if err != nil {
		t.Fatalf("the send never answered: %v", err)
	}
	response.Body.Close()
	if response.StatusCode != http.StatusOK || lambdas.innerStatus.Load() != http.StatusTooManyRequests || time.Since(started) > 2*time.Second {
		t.Fatalf("send %d, push back %d, took %s", response.StatusCode, lambdas.innerStatus.Load(), time.Since(started))
	}
}

type slowLambda struct {
	running atomic.Int32
	most    atomic.Int32
}

func (lambda *slowLambda) serve(writer http.ResponseWriter, request *http.Request) {
	now := lambda.running.Add(1)
	lambda.most.Store(max(lambda.most.Load(), now))
	time.Sleep(300 * time.Millisecond)
	lambda.running.Add(-1)
	_, _ = writer.Write([]byte(`{"statusCode":200,"body":"ok"}`))
}

func TestWriterSlotIsHeldUntilTheFunctionAnswersEvenWhenTheCallerLeaves(t *testing.T) {
	lambda := &slowLambda{}
	functions := httptest.NewServer(http.HandlerFunc(lambda.serve))
	t.Cleanup(functions.Close)
	handler := handlerFor(t, cycleSettings(functions.URL))
	leaving, cancel := context.WithTimeout(context.Background(), 50*time.Millisecond)
	defer cancel()
	first := httptest.NewRequest(http.MethodPost, "/api/v1/messaging/messages", strings.NewReader("{}")).WithContext(leaving)
	var group sync.WaitGroup
	group.Add(1)
	go func() {
		defer group.Done()
		handler.ServeHTTP(httptest.NewRecorder(), first)
	}()
	time.Sleep(100 * time.Millisecond)
	second := call(t, handler, http.MethodPost, "/api/v1/messaging/messages", "{}")
	group.Wait()
	if second.Code != http.StatusOK || lambda.most.Load() != 1 {
		t.Fatalf("second %d, most at once %d", second.Code, lambda.most.Load())
	}
}

func eventFor(method string, rawPath string) HttpEvent {
	return HttpEvent{RawPath: rawPath, RequestContext: EventContext{Http: EventMethod{Method: method}}}
}

func TestEventPushesTakeASlotOnlyWhenOneIsFree(t *testing.T) {
	if (EntryRule{}).For(eventFor(http.MethodPost, "/api/v1/messaging/events")) != (ImmediateEntry{}) {
		t.Fatal("an event push must not wait")
	}
}

func TestOtherCallsWaitForTheirSlot(t *testing.T) {
	rule := EntryRule{}
	queued := []SlotEntry{
		rule.For(eventFor(http.MethodPost, "/api/v1/messaging/messages")),
		rule.For(eventFor(http.MethodGet, "/api/v1/messaging/events")),
		rule.For(eventFor(http.MethodPost, "/api/v1/messaging/events/extra")),
		rule.For(eventFor(http.MethodPost, "/api/v1/messaging/outbox/relay")),
		rule.For(eventFor(http.MethodPost, "/api/v1/directory/outbox/dead/requeue")),
		rule.For(eventFor(http.MethodPost, "/api/v1/directory/jobs/removals")),
		rule.For(eventFor(http.MethodGet, "/api/v1/directory/outbox/dead")),
	}
	want := []SlotEntry{QueuedEntry{}, QueuedEntry{}, QueuedEntry{}, QueuedEntry{Background: true}, QueuedEntry{Background: true}, QueuedEntry{Background: true}, QueuedEntry{}}
	if !slices.Equal(queued, want) {
		t.Fatalf("got %#v", queued)
	}
}

func TestImmediateEntryRefusesAFullSlot(t *testing.T) {
	gate := FunctionGate{}.New(1)
	release, err := ImmediateEntry{}.Take(context.Background(), gate, time.Now)
	_, refused := ImmediateEntry{}.Take(context.Background(), gate, time.Now)
	release()
	again, againErr := ImmediateEntry{}.Take(context.Background(), gate, time.Now)
	if err != nil || refused == nil || againErr != nil {
		t.Fatalf("first %v, refused %v, again %v", err, refused, againErr)
	}
	again()
}
