package outbox

import (
	"encoding/json"
	"errors"
	"io"
	"net/http"
	"net/http/httptest"
	"testing"
	"time"

	"mesail.dev/packages/events"
)

type failingToken struct{}

func (failingToken) Token() (string, error) {
	return "", errors.New("no token")
}

type recordedRequest struct {
	authorization string
	contentType   string
	method        string
	envelope      events.Envelope
}

func answeringServer(t *testing.T, status int, seen *recordedRequest) *httptest.Server {
	t.Helper()
	server := httptest.NewServer(http.HandlerFunc(func(writer http.ResponseWriter, request *http.Request) {
		body, _ := io.ReadAll(request.Body)
		_ = json.Unmarshal(body, &seen.envelope)
		seen.authorization = request.Header.Get("Authorization")
		seen.contentType = request.Header.Get("Content-Type")
		seen.method = request.Method
		writer.WriteHeader(status)
	}))
	t.Cleanup(server.Close)
	return server
}

func pushWithStatus(t *testing.T, status int) (error, recordedRequest) {
	t.Helper()
	seen := recordedRequest{}
	server := answeringServer(t, status, &seen)
	pusher := NewHTTPPusher(StaticToken{Value: "service-token"}, HTTPPusher{}.DefaultTimeout())
	err := pusher.Push(Subscriber{Name: "messaging", URL: server.URL}, channelCreated("evt_1"))
	return err, seen
}

func TestHTTPPusherSendsTheEnvelopeWithABearerToken(t *testing.T) {
	err, seen := pushWithStatus(t, http.StatusAccepted)

	if err != nil || seen.method != http.MethodPost || seen.authorization != "Bearer service-token" || seen.contentType != "application/json" {
		t.Fatalf("request %+v, error %v", seen, err)
	}
	if seen.envelope.EventID != "evt_1" || string(seen.envelope.Payload) != `{"channelId":"chn_1"}` {
		t.Fatalf("envelope %+v", seen.envelope)
	}
}

func TestHTTPPusherTreatsABadRequestAsPermanent(t *testing.T) {
	err, _ := pushWithStatus(t, http.StatusBadRequest)

	failure := DeliveryFailure{}
	if !errors.As(err, &failure) || !failure.Permanent || failure.Status != http.StatusBadRequest || failure.Subscriber != "messaging" {
		t.Fatalf("error %v", err)
	}
}

func TestHTTPPusherTreatsBusyAnswersAsTransient(t *testing.T) {
	statuses := []int{http.StatusServiceUnavailable, http.StatusTooManyRequests, http.StatusRequestTimeout}
	for _, status := range statuses {
		err, _ := pushWithStatus(t, status)
		failure := DeliveryFailure{}
		if !errors.As(err, &failure) || failure.Permanent || failure.Status != status {
			t.Fatalf("status %d gave %v", status, err)
		}
	}
}

func TestHTTPPusherReportsATokenFailure(t *testing.T) {
	pusher := NewHTTPPusher(failingToken{}, time.Second)

	err := pusher.Push(Subscriber{Name: "messaging", URL: "http://127.0.0.1:1"}, channelCreated("evt_1"))

	if err == nil || err.Error() != "no token" {
		t.Fatalf("error %v", err)
	}
}

func TestHTTPPusherReportsANetworkFailureAsTransient(t *testing.T) {
	server := httptest.NewServer(http.NotFoundHandler())
	url := server.URL
	server.Close()
	pusher := NewHTTPPusher(StaticToken{Value: "service-token"}, time.Second)

	err := pusher.Push(Subscriber{Name: "messaging", URL: url}, channelCreated("evt_1"))

	failure := DeliveryFailure{}
	if !errors.As(err, &failure) || failure.Permanent || failure.Status != 0 {
		t.Fatalf("error %v", err)
	}
}

func TestHTTPPusherReportsAnEnvelopeItCannotEncode(t *testing.T) {
	envelope := channelCreated("evt_1")
	envelope.Payload = json.RawMessage("{")

	err := NewHTTPPusher(StaticToken{}, time.Second).Push(Subscriber{Name: "messaging", URL: "http://127.0.0.1:1"}, envelope)

	if err == nil {
		t.Fatal("a broken payload should be reported")
	}
}

func TestHTTPPusherReportsABadURL(t *testing.T) {
	err := NewHTTPPusher(StaticToken{}, time.Second).Push(Subscriber{Name: "messaging", URL: "::"}, channelCreated("evt_1"))

	if err == nil {
		t.Fatal("a bad URL should be reported")
	}
}
