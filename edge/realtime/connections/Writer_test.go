package connections_test

import (
	"errors"
	"reflect"
	"testing"
	"time"

	"mesail.dev/edge/realtime/connections"
)

const helloEvent = `{"type":"hello","conversationId":"","messageId":"","state":""}`
const expiredEvent = `{"type":"auth.expired","conversationId":"","messageId":"","state":"4401"}`
const messageEvent = `{"type":"message.created"}`
const neverTicks = time.Hour

func newSession(authWindow time.Duration) *connections.Session {
	settings := connections.DefaultSessionSettings()
	settings.AuthWindow = authWindow
	return connections.NewSession("session-1", settings)
}

func runWriter(session *connections.Session, transport *FakeTransport, interval time.Duration) {
	connections.NewWriter(session, transport, interval).Run(make(chan struct{}))
}

func TestTheWriterSendsQueuedEventsUntilTheTransportBreaks(t *testing.T) {
	session := newSession(neverTicks)
	session.Push([]byte(messageEvent))
	transport := &FakeTransport{successfulSends: 1}

	runWriter(session, transport, neverTicks)

	expected := []string{helloEvent, messageEvent}
	if !reflect.DeepEqual(transport.Sends(), expected) {
		t.Fatalf("sends = %v, want %v", transport.Sends(), expected)
	}
	if transport.CloseCode() != (connections.CloseCodes{}).Broken() {
		t.Fatalf("close code = %d, want %d", transport.CloseCode(), connections.CloseCodes{}.Broken())
	}
}

func TestTheWriterSendsNothingForASessionThatIsAlreadyClosed(t *testing.T) {
	session := newSession(neverTicks)
	session.Close(connections.CloseCodes{}.Normal())
	transport := &FakeTransport{successfulSends: 10}

	runWriter(session, transport, neverTicks)

	if len(transport.Sends()) != 0 {
		t.Fatalf("sends = %v, want none", transport.Sends())
	}
	if transport.CloseCode() != (connections.CloseCodes{}).Normal() {
		t.Fatalf("close code = %d, want %d", transport.CloseCode(), connections.CloseCodes{}.Normal())
	}
}

func TestTheWriterSaysFarewellWhenTheSignInExpires(t *testing.T) {
	session := newSession(10 * time.Millisecond)
	transport := &FakeTransport{successfulSends: 10}

	runWriter(session, transport, neverTicks)

	if transport.LastSend() != expiredEvent {
		t.Fatalf("last send = %s, want %s", transport.LastSend(), expiredEvent)
	}
	if transport.CloseCode() != (connections.CloseCodes{}).Unauthorized() {
		t.Fatalf("close code = %d, want %d", transport.CloseCode(), connections.CloseCodes{}.Unauthorized())
	}
}

func TestTheWriterStopsWhenAHeartbeatFails(t *testing.T) {
	session := newSession(neverTicks)
	transport := &FakeTransport{successfulSends: 10, heartbeatError: errors.New("no answer")}

	runWriter(session, transport, time.Millisecond)

	if transport.Heartbeats() != 1 {
		t.Fatalf("heartbeats = %d, want 1", transport.Heartbeats())
	}
	if transport.CloseCode() != (connections.CloseCodes{}).Broken() {
		t.Fatalf("close code = %d, want %d", transport.CloseCode(), connections.CloseCodes{}.Broken())
	}
}
