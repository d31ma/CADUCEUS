package mailout

import (
	"encoding/json"
	"net"
	"net/http"
	"net/http/httptest"
	"strings"
	"sync"
	"testing"
	"time"

	"mesail.dev/packages/events"
	"mesail.dev/packages/outbox"
)

type notifySource struct {
	MessageId     string   `json:"messageId"`
	DomainId      string   `json:"domainId"`
	Body          string   `json:"body"`
	AttachmentIds []string `json:"attachmentIds"`
}

type notifyRoute struct {
	lock        sync.Mutex
	summaries   map[string]notifySource
	requests    []string
	connections int
}

func startNotify(t *testing.T) (*notifyRoute, *httptest.Server) {
	route := &notifyRoute{summaries: map[string]notifySource{}}
	mux := http.NewServeMux()
	mux.HandleFunc("GET /api/v1/notify/messages/{message_id}/source", route.source)
	server := httptest.NewUnstartedServer(mux)
	server.Config.ConnState = route.count
	server.Start()
	t.Cleanup(server.Close)
	return route, server
}

func (route *notifyRoute) count(connection net.Conn, state http.ConnState) {
	route.lock.Lock()
	defer route.lock.Unlock()
	if state == http.StateNew {
		route.connections++
	}
}

func (route *notifyRoute) store(domainId string, storedId string, body string) string {
	route.lock.Lock()
	defer route.lock.Unlock()
	route.summaries[domainId+" "+storedId] = notifySource{MessageId: "sum_" + storedId, DomainId: domainId, Body: body, AttachmentIds: []string{}}
	return "sum_" + storedId
}

func (route *notifyRoute) source(writer http.ResponseWriter, request *http.Request) {
	route.lock.Lock()
	defer route.lock.Unlock()
	route.requests = append(route.requests, request.Method+" "+request.URL.Path+" "+request.Header.Get("Authorization"))
	if !strings.HasPrefix(request.Header.Get("Authorization"), "Bearer token-") {
		http.Error(writer, "Sign in to call this route.", http.StatusUnauthorized)
		return
	}
	storedId, prefixed := strings.CutPrefix(request.PathValue("message_id"), "sum_")
	summary, found := route.summaries[tokenRealm(request.Header.Get("Authorization"))+" "+storedId]
	if !prefixed || !found {
		writer.WriteHeader(http.StatusNotFound)
		return
	}
	_ = json.NewEncoder(writer).Encode(summary)
}

func (route *notifyRoute) seen() ([]string, int) {
	route.lock.Lock()
	defer route.lock.Unlock()
	return append([]string{}, route.requests...), route.connections
}

func summaryEnvelope(w *world, domainId string, messageId string, rcpt string) sendEnvelope {
	request := events.EmailSendRequested{
		MessageId: messageId, From: "notifications@" + domainId, Rcpt: rcpt, Role: "to", To: []string{rcpt}, Cc: []string{},
		Subject: "1 unread message", References: []string{}, AttachmentIds: []string{}, IdempotencyKey: messageId,
	}
	return sendEnvelope{EventId: "evt_" + w.unique(), Type: "email.send.requested", DomainId: domainId, Producer: "notify", OccurredAt: Clock{}.Format(time.Now()), Payload: request}
}

func TestANotifySummaryIsSentWithTheBodyNotifyGives(t *testing.T) {
	route, server := startNotify(t)
	w := newWorld(t, func(w *world) { w.settings.NotifyUrl = server.URL + "/" })
	messageId := route.store("example.com", "a1b2", "You have 1 unread message in #general.")

	_, err := w.send(summaryEnvelope(w, "example.com", messageId, "dave@partner.test"))
	report := w.run(time.Now())

	requests, _ := route.seen()
	messages := w.relay.Messages()
	equal(t, nil, err)
	equal(t, QueueReport{Picked: 1, Sent: 1}, report)
	equal(t, []string{"GET /api/v1/notify/messages/sum_a1b2/source Bearer token-example.com"}, requests)
	equal(t, 1, len(messages))
	equal(t, "notifications@example.com", messages[0].From)
	equal(t, true, strings.Contains(string(messages[0].Data), "You have 1 unread message in #general."))
	equal(t, "<sum_a1b2@example.com>", header(string(messages[0].Data), "Message-ID"))
}

func TestNotifySourceUsesANewConnectionForEachCall(t *testing.T) {
	route, server := startNotify(t)
	w := newWorld(t, func(w *world) { w.settings.NotifyUrl = server.URL })
	messageId := route.store("example.com", "c3d4", "2 unread")
	item := Item{DomainId: "example.com", Producer: "notify", MessageId: messageId}

	first, firstErr := w.desk.Intake.Sources.Source(item)
	_, secondErr := w.desk.Intake.Sources.Source(item)

	_, connections := route.seen()
	equal(t, nil, firstErr)
	equal(t, nil, secondErr)
	equal(t, MessageSource{MessageId: messageId, DomainId: "example.com", Body: "2 unread", AttachmentIds: []string{}}, first)
	equal(t, 2, connections)
}

func TestNotifySourceOfAnotherDomainIsNotFound(t *testing.T) {
	route, server := startNotify(t)
	w := newWorld(t, func(w *world) { w.settings.NotifyUrl = server.URL })
	messageId := route.store("example.com", "e5f6", "3 unread")

	_, err := w.desk.Intake.Sources.Source(Item{DomainId: "example.org", Producer: "notify", MessageId: messageId})

	equal(t, "the producer answered 404 for the message source", err.Error())
}

func TestNotifySourceWithoutATokenIsRefused(t *testing.T) {
	_, server := startNotify(t)
	sources := Sources{Client: ServiceClient{Client: http.DefaultClient, Authorize: func(string) (string, error) { return "", nil }}, Bases: map[string]string{"notify": server.URL}}

	_, err := sources.Source(Item{DomainId: "example.com", Producer: "notify", MessageId: "sum_1"})

	equal(t, "the producer answered 401 for the message source", err.Error())
}

func TestSubscriberPusherSortsAnswers(t *testing.T) {
	answers := []int{http.StatusAccepted, http.StatusNotFound, http.StatusTooManyRequests, http.StatusServiceUnavailable}
	index := 0
	server := httptest.NewServer(http.HandlerFunc(func(writer http.ResponseWriter, request *http.Request) {
		writer.WriteHeader(answers[index])
		index++
	}))
	defer server.Close()
	pusher := SubscriberPusher{Client: ServiceClient{Client: http.DefaultClient, Authorize: fixedTokens}}
	subscriber := outbox.Subscriber{Name: "messaging", URL: server.URL}
	envelope := events.Envelope{DomainId: "example.com", Payload: json.RawMessage(`{}`)}

	accepted := pusher.Push(subscriber, envelope)
	missing := pusher.Push(subscriber, envelope)
	busy := pusher.Push(subscriber, envelope)
	down := pusher.Push(subscriber, envelope)
	unreachable := pusher.Push(outbox.Subscriber{Name: "messaging", URL: "http://127.0.0.1:1"}, envelope)

	equal(t, nil, accepted)
	equal(t, outbox.DeliveryFailure{Subscriber: "messaging", Status: 404, Permanent: true, Detail: "Not Found"}, missing)
	equal(t, outbox.DeliveryFailure{Subscriber: "messaging", Status: 429, Detail: "Too Many Requests"}, busy)
	equal(t, outbox.DeliveryFailure{Subscriber: "messaging", Status: 503, Detail: "Service Unavailable"}, down)
	equal(t, "messaging", unreachable.(outbox.DeliveryFailure).Subscriber)
}
