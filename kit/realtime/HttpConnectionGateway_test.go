package realtime

import (
	"encoding/json"
	"errors"
	"io"
	"net/http"
	"net/http/httptest"
	"testing"
	"time"
)

type gatewayCall struct {
	method string
	uri    string
	secret string
	body   string
}

func gatewayServer(t *testing.T, status int, calls chan gatewayCall) HttpConnectionGateway {
	t.Helper()
	server := httptest.NewServer(http.HandlerFunc(func(writer http.ResponseWriter, request *http.Request) {
		body, _ := io.ReadAll(request.Body)
		calls <- gatewayCall{method: request.Method, uri: request.RequestURI, secret: request.Header.Get("X-Realtime-Gateway"), body: string(body)}
		writer.WriteHeader(status)
	}))
	t.Cleanup(server.Close)
	return HttpConnectionGateway{BaseUrl: server.URL, Secret: "the-secret", Client: &http.Client{Timeout: 5 * time.Second}}
}

func TestPushPostsTheEventToTheEscapedConnection(t *testing.T) {
	calls := make(chan gatewayCall, 1)
	gateway := gatewayServer(t, http.StatusOK, calls)
	err := gateway.Push("a/b c", Event{Type: "typing", ConversationId: "c1", MemberId: "ana"})
	call := <-calls
	event := Event{}
	json.Unmarshal([]byte(call.body), &event)
	if err != nil || call.method != http.MethodPost || call.uri != "/@connections/a%2Fb%20c" || call.secret != "the-secret" || event.Type != "typing" || event.MemberId != "ana" {
		t.Fatalf("got %+v, %v", call, err)
	}
}

func TestCloseDeletesTheConnectionWithTheCode(t *testing.T) {
	calls := make(chan gatewayCall, 1)
	gateway := gatewayServer(t, http.StatusNoContent, calls)
	err := gateway.Close("c1", 4401)
	call := <-calls
	if err != nil || call.method != http.MethodDelete || call.uri != "/@connections/c1?code=4401" || call.secret != "the-secret" {
		t.Fatalf("got %+v, %v", call, err)
	}
}

func TestPushReportsAGoneConnection(t *testing.T) {
	gateway := gatewayServer(t, http.StatusGone, make(chan gatewayCall, 1))
	if err := gateway.Push("c1", Event{Type: "typing"}); !errors.Is(err, Gone{ConnectionId: "c1"}) {
		t.Fatalf("got %v", err)
	}
}

func TestCloseReportsAGatewayError(t *testing.T) {
	gateway := gatewayServer(t, http.StatusInternalServerError, make(chan gatewayCall, 1))
	if err := gateway.Close("c1", 4401); !errors.As(err, &Unavailable{}) {
		t.Fatalf("got %v", err)
	}
}

func TestPushReportsAGatewayThatDoesNotAnswer(t *testing.T) {
	gateway := gatewayServer(t, http.StatusOK, make(chan gatewayCall, 1))
	gateway.BaseUrl = "http://127.0.0.1:1"
	if err := gateway.Push("c1", Event{Type: "typing"}); !errors.As(err, &Unavailable{}) {
		t.Fatalf("got %v", err)
	}
}

func TestPushReportsABadGatewayAddress(t *testing.T) {
	gateway := HttpConnectionGateway{BaseUrl: "http://[::1", Client: http.DefaultClient}
	if err := gateway.Push("c1", Event{Type: "typing"}); !errors.As(err, &Unavailable{}) {
		t.Fatalf("got %v", err)
	}
}

func TestAnEmptyConnectionIdIsGoneWithoutARequest(t *testing.T) {
	gateway := HttpConnectionGateway{BaseUrl: "http://127.0.0.1:1", Secret: "s", Client: http.DefaultClient}
	if err := gateway.Push("", Event{Type: "hello"}); !errors.As(err, &Gone{}) {
		t.Fatalf("got %v", err)
	}
}
