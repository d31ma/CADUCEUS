package outbox

import (
	"bytes"
	"encoding/json"
	"errors"
	"net/http"
	"net/http/httptest"
	"sync"
	"testing"
	"time"

	"mesail.dev/packages/events"
	"mesail.dev/packages/oidc"
)

type consumerApp struct {
	mutex    sync.Mutex
	receiver events.Receiver
	applied  map[string]int
	domains  map[string]string
	answers  []int
}

func newConsumerApp() *consumerApp {
	app := &consumerApp{applied: map[string]int{}, domains: map[string]string{}}
	app.receiver = events.NewReceiver("messaging", events.NewMemorySeen(), app)
	return app
}

func (app *consumerApp) Handle(envelope events.Envelope) error {
	app.applied[envelope.EventID]++
	app.domains[envelope.EventID] = envelope.DomainId
	return nil
}

func (app *consumerApp) ServeHTTP(writer http.ResponseWriter, request *http.Request) {
	app.mutex.Lock()
	defer app.mutex.Unlock()
	status, body := app.answer(request)
	app.answers = append(app.answers, status)
	writer.Header().Set("Content-Type", "application/json")
	writer.WriteHeader(status)
	_, _ = writer.Write(body)
}

func (app *consumerApp) answer(request *http.Request) (int, []byte) {
	if request.URL.Path != "/api/v1/messaging/events" {
		return http.StatusNotFound, []byte(`{}`)
	}
	caller := oidc.Caller{DomainId: "mesail-services", MemberId: "relay"}
	if request.Header.Get("Authorization") == "Bearer service-token" {
		caller.Roles = []string{"Service"}
	}
	var envelope events.Envelope
	if err := json.NewDecoder(request.Body).Decode(&envelope); err != nil {
		return http.StatusBadRequest, []byte(`{}`)
	}
	receipt, err := app.receiver.Receive(caller, envelope)
	if err != nil {
		return statusFor(err), []byte(`{}`)
	}
	body, _ := json.Marshal(receipt)
	return http.StatusOK, body
}

func statusFor(err error) int {
	var notService events.NotService
	if errors.As(err, &notService) {
		return http.StatusForbidden
	}
	var invalid events.InvalidEnvelope
	var notSubscribed events.NotSubscribed
	if errors.As(err, &invalid) || errors.As(err, &notSubscribed) {
		return http.StatusBadRequest
	}
	return http.StatusServiceUnavailable
}

type producerApp struct {
	store   *MemoryStore
	outbox  Outbox
	relay   Relay
	factory events.EnvelopeFactory
}

func newProducerApp(consumerURL string, token string) producerApp {
	settings := Settings{
		Producer:    "channels",
		Subscribers: []Subscriber{{Name: "messaging", URL: consumerURL + "/api/v1/messaging/events"}},
	}
	clock := func() time.Time { return time.Date(2026, 11, 10, 12, 0, 0, 0, time.UTC) }
	store := NewMemoryStore()
	return producerApp{
		store:   store,
		outbox:  NewOutbox(settings),
		relay:   NewRelay(store, NewHTTPPusher(StaticToken{Value: token}, 5*time.Second), settings, clock),
		factory: events.NewEnvelopeFactory("channels", clock, bytes.NewReader(bytes.Repeat([]byte{7, 9, 11}, 64))),
	}
}

func (app producerApp) createChannel(t *testing.T, ref Ref, domainId string) events.Envelope {
	t.Helper()
	envelope, err := app.factory.New(events.Types{}.ChannelCreated(), domainId, map[string]string{"channelId": "chn_" + ref.Id})
	if err != nil {
		t.Fatal(err)
	}
	fields, err := app.outbox.Stage(Fields{}, envelope)
	if err != nil {
		t.Fatal(err)
	}
	app.store.Put(ref, fields)
	return envelope
}

func TestTwoAppsDeliverAnEventExactlyOnce(t *testing.T) {
	consumer := newConsumerApp()
	server := httptest.NewServer(consumer)
	defer server.Close()
	producer := newProducerApp(server.URL, "service-token")
	ref := Ref{Collection: "channels.example_com", Id: "1"}
	envelope := producer.createChannel(t, ref, "example.com")

	report, err := producer.relay.RelayDocument(ref)
	if err != nil || report.Delivered != 1 {
		t.Fatalf("first relay: %+v, %v", report, err)
	}
	if _, err := producer.relay.RunOnce(10); err != nil {
		t.Fatal(err)
	}
	if consumer.applied[envelope.EventID] != 1 || consumer.domains[envelope.EventID] != "example.com" {
		t.Fatalf("applied %v in %v", consumer.applied, consumer.domains)
	}
	if fields := producer.store.Fields(ref); fields.OutboxPending != 0 || len(fields.PendingEvents) != 0 {
		t.Fatalf("still pending: %+v", fields)
	}
}

func TestTwoAppsApplyARedeliveredEventOnlyOnce(t *testing.T) {
	consumer := newConsumerApp()
	server := httptest.NewServer(consumer)
	defer server.Close()
	producer := newProducerApp(server.URL, "service-token")
	ref := Ref{Collection: "channels.example_org", Id: "2"}
	envelope := producer.createChannel(t, ref, "example.org")
	producer.store.FailNextSave(errors.New("the producer stopped after the push"))

	if _, err := producer.relay.RunOnce(10); err == nil {
		t.Fatal("the save failure should be reported")
	}
	report, err := producer.relay.RunOnce(10)
	if err != nil || report.Delivered != 1 {
		t.Fatalf("second relay: %+v, %v", report, err)
	}
	if consumer.applied[envelope.EventID] != 1 {
		t.Fatalf("applied %d times", consumer.applied[envelope.EventID])
	}
	if len(consumer.answers) != 2 {
		t.Fatalf("pushed %d times, want 2", len(consumer.answers))
	}
	if producer.store.Fields(ref).OutboxPending != 0 {
		t.Fatal("the event is still pending after the redelivery")
	}
}

func TestTwoAppsDeadLetterAnEventTheConsumerRefuses(t *testing.T) {
	consumer := newConsumerApp()
	server := httptest.NewServer(consumer)
	defer server.Close()
	producer := newProducerApp(server.URL, "not-a-service-token")
	ref := Ref{Collection: "channels.example_com", Id: "3"}
	envelope := producer.createChannel(t, ref, "example.com")

	report, err := producer.relay.RunOnce(10)
	if err != nil || report.DeadLettered != 1 {
		t.Fatalf("relay: %+v, %v", report, err)
	}
	if consumer.applied[envelope.EventID] != 0 {
		t.Fatal("a refused event was applied")
	}
	dead, err := DeadLetters{}.List(producer.store.Fields(ref))
	if err != nil || len(dead) != 1 || dead[0].Subscriber != "messaging" {
		t.Fatalf("dead letters %+v, %v", dead, err)
	}
}

func TestTwoAppsKeepEachDomainsEventsApart(t *testing.T) {
	consumer := newConsumerApp()
	server := httptest.NewServer(consumer)
	defer server.Close()
	producer := newProducerApp(server.URL, "service-token")
	first := producer.createChannel(t, Ref{Collection: "channels.example_com", Id: "4"}, "example.com")
	second := producer.createChannel(t, Ref{Collection: "channels.example_org", Id: "5"}, "example.org")

	if _, err := producer.relay.RunOnce(10); err != nil {
		t.Fatal(err)
	}
	if consumer.domains[first.EventID] != "example.com" || consumer.domains[second.EventID] != "example.org" {
		t.Fatalf("domains %v", consumer.domains)
	}
}
