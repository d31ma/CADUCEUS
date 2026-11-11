package transport_test

import (
	"encoding/json"
	"net/http"
	"testing"
	"time"

	"mesail.dev/edge/realtime/connections"
)

func TestStreamWithoutTicketAnswers400(t *testing.T) {
	harness := newHarness(t, defaults)
	response, err := http.Get(harness.public.URL + "/api/v1/realtime/stream")
	if err != nil {
		t.Fatal(err)
	}
	_ = response.Body.Close()
	if response.StatusCode != http.StatusBadRequest {
		t.Fatalf("status %d", response.StatusCode)
	}
}

func TestStreamSendsTheTicketAsTheFirstMessage(t *testing.T) {
	harness := newHarness(t, defaults)
	stream := harness.openStream(`?ticket=ab%22cd`)
	if stream.response.Header.Get("Content-Type") != "text/event-stream" {
		t.Fatalf("content type %q", stream.response.Header.Get("Content-Type"))
	}
	harness.eventually("message call", func() bool { return len(harness.backend.messagesSnapshot()) == 1 })
	connect := harness.backend.connectsSnapshot()[0]
	message := harness.backend.messagesSnapshot()[0]
	body := map[string]string{}
	_ = json.Unmarshal([]byte(message.Body), &body)
	if connect.Kind != "stream" || message.ConnectionId != connect.ConnectionId || body["type"] != "ticket" || body["ticket"] != `ab"cd` {
		t.Fatalf("connect %+v message %+v", connect, message)
	}
}

func TestStreamDeliversEventsAsDataLines(t *testing.T) {
	harness := newHarness(t, defaults)
	harness.backend.acceptFor(time.Minute)
	stream := harness.openStream("?ticket=t1")
	harness.eventually("message call", func() bool { return len(harness.backend.messagesSnapshot()) == 1 })
	connectionId := harness.backend.connectsSnapshot()[0].ConnectionId
	time.Sleep(20 * time.Millisecond)
	if status := harness.push(connectionId, "{\n\"type\": \"message.created\"\n}"); status != http.StatusOK {
		t.Fatalf("push status %d", status)
	}
	if line := stream.next(); line != `data: {"type":"message.created"}` {
		t.Fatalf("line %q", line)
	}
}

func TestStreamSendsKeepAliveComments(t *testing.T) {
	harness := newHarness(t, func(settings *connections.SessionSettings) { settings.AuthWindow = time.Minute })
	stream := harness.openStream("?ticket=t1")
	stream.waitFor(": keep-alive")
}

func TestStreamClosesAtTheDeadline(t *testing.T) {
	harness := newHarness(t, defaults)
	harness.backend.acceptFor(400 * time.Millisecond)
	stream := harness.openStream("?ticket=t1")
	if line := stream.next(); line != "data: "+authExpiredFrame {
		t.Fatalf("line %q", line)
	}
	if line := stream.next(); line != "event: close" {
		t.Fatalf("line %q", line)
	}
	if line := stream.next(); line != `data: {"code":4401}` {
		t.Fatalf("line %q", line)
	}
	connectionId := harness.backend.connectsSnapshot()[0].ConnectionId
	if disconnect := harness.disconnectFor(connectionId); disconnect.DomainId != "domain-1" {
		t.Fatalf("disconnect %+v", disconnect)
	}
}

func TestStreamClosesOnDeleteWithoutExpiredEvent(t *testing.T) {
	harness := newHarness(t, func(settings *connections.SessionSettings) { settings.AuthWindow = time.Minute })
	stream := harness.openStream("?ticket=t1")
	harness.eventually("connect call", func() bool { return len(harness.backend.connectsSnapshot()) == 1 })
	connectionId := harness.backend.connectsSnapshot()[0].ConnectionId
	if status := harness.remove(connectionId, "?code=4401"); status != http.StatusNoContent {
		t.Fatalf("delete status %d", status)
	}
	if line := stream.next(); line != "event: close" {
		t.Fatalf("line %q", line)
	}
	if line := stream.next(); line != `data: {"code":4401}` {
		t.Fatalf("line %q", line)
	}
}

func TestStreamConnectFailureAnswers503(t *testing.T) {
	harness := newHarness(t, defaults)
	harness.backend.setConnectStatus(http.StatusInternalServerError)
	response, err := http.Get(harness.public.URL + "/api/v1/realtime/stream?ticket=t1")
	if err != nil {
		t.Fatal(err)
	}
	_ = response.Body.Close()
	if response.StatusCode != http.StatusServiceUnavailable {
		t.Fatalf("status %d", response.StatusCode)
	}
	harness.disconnectFor(harness.backend.connectsSnapshot()[0].ConnectionId)
}

func TestStreamClientLeavingCallsDisconnect(t *testing.T) {
	harness := newHarness(t, func(settings *connections.SessionSettings) { settings.AuthWindow = time.Minute })
	stream := harness.openStream("?ticket=t1")
	harness.eventually("connect call", func() bool { return len(harness.backend.connectsSnapshot()) == 1 })
	_ = stream.response.Body.Close()
	harness.disconnectFor(harness.backend.connectsSnapshot()[0].ConnectionId)
}
