package mailin

import (
	"encoding/json"
	"errors"
	"io"
	"net/http"
	"net/http/httptest"
	"testing"

	"mesail.dev/packages/events"
	"mesail.dev/packages/outbox"
	"mesail.dev/packages/transport"
)

type pushedRequest struct {
	Authorization string
	ContentType   string
	Closed        bool
	Body          string
}

func pushServer(status int, seen chan pushedRequest) *httptest.Server {
	return httptest.NewServer(http.HandlerFunc(func(writer http.ResponseWriter, request *http.Request) {
		body, _ := io.ReadAll(request.Body)
		seen <- pushedRequest{Authorization: request.Header.Get("Authorization"), ContentType: request.Header.Get("Content-Type"), Closed: request.Close, Body: string(body)}
		writer.WriteHeader(status)
	}))
}

func domainToken(domainId string) (string, error) {
	return "token-for-" + domainId, nil
}

func testPusher() EventPusher {
	return EventPusher{Tokens: outbox.DomainTokenFunc(domainToken), Client: transport.SingleUseTransport{}.Client(outbox.HTTPPusher{}.DefaultTimeout())}
}

func pushEnvelope() events.Envelope {
	return events.Envelope{EventID: "evt-1", Type: "email.received", DomainId: "example.com", Producer: "mail-in", Payload: json.RawMessage(`{"inboundId":"in_1"}`)}
}

func pushTo(t *testing.T, status int) (pushedRequest, error) {
	seen := make(chan pushedRequest, 1)
	server := pushServer(status, seen)
	defer server.Close()
	err := testPusher().Push(outbox.Subscriber{Name: "messaging", URL: server.URL}, pushEnvelope())
	return <-seen, err
}

func deliveryFailure(err error) outbox.DeliveryFailure {
	failure := outbox.DeliveryFailure{}
	errors.As(err, &failure)
	return failure
}

func TestThePusherSendsTheEnvelopeWithTheDomainToken(t *testing.T) {
	seen, err := pushTo(t, http.StatusAccepted)
	check(t, err == nil, "the push failed")
	check(t, seen.Authorization == "Bearer token-for-example.com" && seen.ContentType == "application/json", "wrong headers")
	check(t, seen.Body == `{"eventId":"evt-1","type":"email.received","domainId":"example.com","producer":"mail-in","occurredAt":"","payload":{"inboundId":"in_1"}}`, "wrong body")
}

func TestThePusherClosesEachConnection(t *testing.T) {
	seen, _ := pushTo(t, http.StatusNoContent)
	check(t, seen.Closed, "the pusher kept the connection alive")
}

func TestThePusherMarksAClientErrorPermanent(t *testing.T) {
	_, err := pushTo(t, http.StatusBadRequest)
	failure := deliveryFailure(err)
	check(t, failure.Subscriber == "messaging" && failure.Status == 400 && failure.Permanent, "want a permanent failure")
}

func TestThePusherRetriesATimedOutRequest(t *testing.T) {
	_, err := pushTo(t, http.StatusRequestTimeout)
	failure := deliveryFailure(err)
	check(t, failure.Status == 408 && !failure.Permanent, "want a failure to retry")
}

func TestThePusherRetriesWhenAskedToSlowDown(t *testing.T) {
	_, err := pushTo(t, http.StatusTooManyRequests)
	failure := deliveryFailure(err)
	check(t, failure.Status == 429 && !failure.Permanent, "want a failure to retry")
}

func TestThePusherRetriesAServerError(t *testing.T) {
	_, err := pushTo(t, http.StatusServiceUnavailable)
	failure := deliveryFailure(err)
	check(t, failure.Status == 503 && !failure.Permanent, "want a failure to retry")
}

func TestThePusherReportsAnUnreachableSubscriber(t *testing.T) {
	server := httptest.NewServer(http.NotFoundHandler())
	server.Close()
	err := testPusher().Push(outbox.Subscriber{Name: "messaging", URL: server.URL}, pushEnvelope())
	failure := deliveryFailure(err)
	check(t, failure.Subscriber == "messaging" && failure.Status == 0 && !failure.Permanent && failure.Detail != "", "want a delivery failure")
}

func TestThePusherStopsWithoutAToken(t *testing.T) {
	refuse := func(string) (string, error) { return "", errors.New("no token") }
	pusher := EventPusher{Tokens: outbox.DomainTokenFunc(refuse), Client: http.DefaultClient}
	err := pusher.Push(outbox.Subscriber{Name: "messaging", URL: "http://127.0.0.1:1"}, pushEnvelope())
	check(t, err != nil && err.Error() == "no token", "want the token failure")
}

func TestThePusherRefusesAnUnreadablePayload(t *testing.T) {
	envelope := pushEnvelope()
	envelope.Payload = json.RawMessage(`{`)
	err := testPusher().Push(outbox.Subscriber{Name: "messaging", URL: "http://127.0.0.1:1"}, envelope)
	_, isDelivery := err.(outbox.DeliveryFailure)
	check(t, err != nil && !isDelivery, "want the encoding failure")
}

func TestThePusherRefusesABadAddress(t *testing.T) {
	err := testPusher().Push(outbox.Subscriber{Name: "messaging", URL: "http://bad host"}, pushEnvelope())
	_, isDelivery := err.(outbox.DeliveryFailure)
	check(t, err != nil && !isDelivery, "want the address failure")
}
