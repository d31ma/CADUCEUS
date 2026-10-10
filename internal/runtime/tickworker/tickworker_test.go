package tickworker

import (
	"io"
	"net/http"
	"net/http/httptest"
	"strings"
	"sync"
	"testing"

	"github.com/d31ma/caduceus/internal/runtime/supervisor"
)

type recorded struct {
	path, signature, contentType, body string
}

func server(t *testing.T, status int, answer string) (*httptest.Server, *[]recorded, *sync.Mutex) {
	t.Helper()
	var mu sync.Mutex
	var requests []recorded
	srv := httptest.NewServer(http.HandlerFunc(func(writer http.ResponseWriter, request *http.Request) {
		body, _ := io.ReadAll(request.Body)
		mu.Lock()
		requests = append(requests, recorded{
			path: request.Method + " " + request.URL.Path, signature: request.Header.Get("X-Caduceus-Signature"),
			contentType: request.Header.Get("Content-Type"), body: string(body),
		})
		mu.Unlock()
		writer.WriteHeader(status)
		_, _ = io.WriteString(writer, answer)
	}))
	t.Cleanup(srv.Close)
	return srv, &requests, &mu
}

func TestSignatureMatchesNodeHMAC(t *testing.T) {
	// createHmac('sha256', 'secret').update('{}').digest('hex')
	if got := Sign("secret", []byte("{}")); got != "77325902caca812dc259733aacd046b73817372c777b8d95b402647474516e13" {
		t.Fatalf("signature = %q", got)
	}
}

func TestOutboundQueueTick(t *testing.T) {
	srv, requests, mu := server(t, 200, `{"delivered":2,"retried":"1","bounced":-1,"queue":{"depth":5,"queued":4,"sending":1,"due":3,"oldestAgeMs":1200,"attempts":7}}`)
	worker, err := NewOutboundQueue(Options{APIURL: srv.URL + "/", Secret: "s3cret"})
	if err != nil {
		t.Fatal(err)
	}
	if !worker.RunNow() {
		t.Fatalf("tick failed: %+v", worker.Snapshot())
	}
	mu.Lock()
	got := (*requests)[0]
	mu.Unlock()
	if got.path != "POST /send/tick" || got.body != "{}" || got.contentType != "application/json" || got.signature != Sign("s3cret", []byte("{}")) {
		t.Fatalf("request = %+v", got)
	}
	encoded, _ := supervisor.Marshal(worker.Snapshot())
	for _, fragment := range []string{
		`"name":"outbound-queue"`,
		`"lastDelivery":{"delivered":2,"retried":1,"bounced":0}`,
		`"queue":{"depth":5,"queued":4,"sending":1,"due":3,"oldestAgeMs":1200,"attempts":7}`,
	} {
		if !strings.Contains(string(encoded), fragment) {
			t.Fatalf("snapshot %s lacks %s", encoded, fragment)
		}
	}
}

func TestTickFailureMessages(t *testing.T) {
	srv, _, _ := server(t, 503, `{}`)
	options := Options{APIURL: srv.URL, Secret: "x"}
	outbound, _ := NewOutboundQueue(options)
	snooze, _ := NewSnooze(options)
	reminders, _ := NewFollowUpReminders(options)
	provider, _ := NewProviderNotification(options)
	outbound.RunNow()
	snooze.RunNow()
	reminders.RunNow()
	provider.RunNow()
	for got, want := range map[string]string{
		outbound.Snapshot().LastError:  "Queue tick failed with HTTP 503",
		snooze.Snapshot().LastError:    "Snooze tick failed with HTTP 503",
		reminders.Snapshot().LastError: "Reminder tick failed with HTTP 503",
		provider.Snapshot().LastError:  "Provider notification tick failed with HTTP 503",
	} {
		if got != want {
			t.Fatalf("lastError = %q, want %q", got, want)
		}
	}
}

func TestRoutesAndReminderValidation(t *testing.T) {
	srv, requests, mu := server(t, 200, `{"due":1,"surfaced":1,"reconciled":0,"failed":0,"extra":"kept"}`)
	options := Options{APIURL: srv.URL, Secret: "x"}
	snooze, _ := NewSnooze(options)
	reminders, _ := NewFollowUpReminders(options)
	provider, _ := NewProviderNotification(options)
	if !snooze.RunNow() || !reminders.RunNow() || !provider.RunNow() {
		t.Fatal("tick failed")
	}
	mu.Lock()
	paths := []string{(*requests)[0].path, (*requests)[1].path, (*requests)[2].path}
	mu.Unlock()
	if strings.Join(paths, ",") != "POST /snooze/tick,POST /reminders/tick,POST /notifications/provider/tick" {
		t.Fatalf("paths = %v", paths)
	}
	encoded, _ := supervisor.Marshal(reminders.Snapshot())
	if !strings.Contains(string(encoded), `"lastResult":{"due":1,"surfaced":1,"reconciled":0,"failed":0,"extra":"kept"}`) {
		t.Fatalf("snapshot = %s", encoded)
	}

	invalid, _, _ := server(t, 200, `{"due":1,"surfaced":-1,"reconciled":0,"failed":0}`)
	bad, _ := NewFollowUpReminders(Options{APIURL: invalid.URL, Secret: "x"})
	if bad.RunNow() || bad.Snapshot().LastError != "Reminder tick returned an invalid result" {
		t.Fatalf("snapshot = %+v", bad.Snapshot())
	}
}
