package realtime

import (
	"errors"
	"testing"
)

func refusedWith4401(t *testing.T, fixture gatewayFixture, connectionId string, reply GatewayReply, err error) {
	t.Helper()
	events := fixture.pushes.eventsOf(connectionId)
	if err != nil || reply != (GatewayReply{}) || fixture.pushes.closes[connectionId] != 4401 || len(events) != 1 || events[0] != (Event{Type: "auth.expired", State: "4401"}) {
		t.Fatalf("got %+v, %v, events %+v, closes %v", reply, err, events, fixture.pushes.closes)
	}
}

func TestConnectAcceptsASocketOrAStream(t *testing.T) {
	gateway := newGatewayFixture().gateway
	if err := gateway.Connect(GatewayConnect{ConnectionId: "c1", Kind: "socket"}); err != nil {
		t.Fatalf("socket: got %v", err)
	}
	if err := gateway.Connect(GatewayConnect{ConnectionId: "c2", Kind: "stream"}); err != nil {
		t.Fatalf("stream: got %v", err)
	}
}

func TestConnectRefusesAMissingIdOrAnUnknownKind(t *testing.T) {
	gateway := newGatewayFixture().gateway
	failure := Invalid{}
	if err := gateway.Connect(GatewayConnect{Kind: "socket"}); !errors.As(err, &failure) || failure.Field != "connectionId" {
		t.Fatalf("missing id: got %v", err)
	}
	if err := gateway.Connect(GatewayConnect{ConnectionId: "c1", Kind: "pigeon"}); !errors.As(err, &failure) || failure.Field != "kind" {
		t.Fatalf("kind: got %v", err)
	}
}

func TestMessageAuthAcceptsAndTellsTheClientWhenItEnds(t *testing.T) {
	fixture := newGatewayFixture()
	reply := fixture.authenticate("c1", "ana-token")
	events := fixture.pushes.eventsOf("c1")
	if reply != (GatewayReply{DomainId: "example.com", ExpiresAt: "2026-11-10T12:01:00Z"}) || len(events) == 0 || events[len(events)-1] != (Event{Type: "auth.accepted", State: "2026-11-10T12:01:00Z"}) {
		t.Fatalf("got %+v, %+v", reply, events)
	}
}

func TestMessageAuthOnAGoneConnectionStillAnswers(t *testing.T) {
	fixture := newGatewayFixture()
	fixture.pushes.gone["c1"] = true
	reply, err := fixture.gateway.Message(GatewayMessage{ConnectionId: "c1", Body: `{"type":"auth","token":"ana-token"}`})
	if err != nil || reply.DomainId != "example.com" {
		t.Fatalf("got %+v, %v", reply, err)
	}
}

func TestMessageRefusedAuthSendsAuthExpiredAndCloses(t *testing.T) {
	fixture := newGatewayFixture()
	reply, err := fixture.gateway.Message(GatewayMessage{ConnectionId: "c1", Body: `{"type":"auth","token":"forged"}`})
	refusedWith4401(t, fixture, "c1", reply, err)
}

func TestMessageRefusedTicketOnAGoneConnectionIsNotAnError(t *testing.T) {
	fixture := newGatewayFixture()
	fixture.pushes.gone["s1"] = true
	reply, err := fixture.gateway.Message(GatewayMessage{ConnectionId: "s1", Body: `{"type":"ticket","ticket":"unknown"}`})
	if err != nil || reply != (GatewayReply{}) {
		t.Fatalf("got %+v, %v", reply, err)
	}
}

func TestMessageRefusedAuthReportsAGatewayThatIsDown(t *testing.T) {
	fixture := newGatewayFixture()
	fixture.pushes.down = true
	if _, err := fixture.gateway.Message(GatewayMessage{ConnectionId: "c1", Body: `{"type":"auth","token":"forged"}`}); !errors.As(err, &Unavailable{}) {
		t.Fatalf("got %v", err)
	}
}

func TestMessageAuthReportsStorageFailuresWithoutClosing(t *testing.T) {
	fixture := newGatewayFixture()
	fixture.memory.failAll = true
	_, err := fixture.gateway.Message(GatewayMessage{ConnectionId: "c1", Body: `{"type":"auth","token":"ana-token"}`})
	if !errors.As(err, &Unavailable{}) || len(fixture.pushes.closes) != 0 {
		t.Fatalf("got %v, closes %v", err, fixture.pushes.closes)
	}
}

func TestMessagePingDoesNothingEvenBeforeAuth(t *testing.T) {
	fixture := newGatewayFixture()
	reply, err := fixture.gateway.Message(GatewayMessage{ConnectionId: "c1", Body: `{"type":"ping"}`})
	if err != nil || reply != (GatewayReply{}) || len(fixture.pushes.pushes) != 0 || len(fixture.pushes.closes) != 0 {
		t.Fatalf("got %+v, %v", reply, err)
	}
}

func TestMessageRefusesAnUnknownTypeOrBadJson(t *testing.T) {
	gateway := newGatewayFixture().gateway
	if _, err := gateway.Message(GatewayMessage{ConnectionId: "c1", Body: `{"type":"dance"}`}); !errors.As(err, &Invalid{}) {
		t.Fatalf("unknown: got %v", err)
	}
	if _, err := gateway.Message(GatewayMessage{ConnectionId: "c1", Body: `{"type":"ping",`}); !errors.As(err, &Invalid{}) {
		t.Fatalf("bad json: got %v", err)
	}
}

func TestMessageTypingBeforeAuthCloses(t *testing.T) {
	fixture := newGatewayFixture()
	reply, err := fixture.gateway.Message(GatewayMessage{ConnectionId: "c1", Body: `{"type":"typing","conversationId":"conversation-1"}`})
	refusedWith4401(t, fixture, "c1", reply, err)
}

func TestMessageTypingForAnUnknownConnectionCloses(t *testing.T) {
	fixture := newGatewayFixture()
	reply, err := fixture.gateway.Message(GatewayMessage{ConnectionId: "c1", DomainId: "example.com", Body: `{"type":"typing","conversationId":"conversation-1"}`})
	refusedWith4401(t, fixture, "c1", reply, err)
}

func TestMessageTypingOnAnEndedConnectionCloses(t *testing.T) {
	fixture := newGatewayFixture()
	connection := testConnection("example.com", "ana", "c1")
	connection.ExpiresAt = "2026-11-10T12:00:00Z"
	fixture.gateway.Connections.Add(connection)
	reply, err := fixture.gateway.Message(GatewayMessage{ConnectionId: "c1", DomainId: "example.com", Body: `{"type":"typing","conversationId":"conversation-1"}`})
	refusedWith4401(t, fixture, "c1", reply, err)
}

func TestMessageTypingFromANonMemberIsNotFound(t *testing.T) {
	fixture := newGatewayFixture()
	fixture.authenticate("c1", "ana-token")
	_, err := fixture.gateway.Message(GatewayMessage{ConnectionId: "c1", DomainId: "example.com", Body: `{"type":"typing","conversationId":"conversation-1"}`})
	if !errors.As(err, &Missing{}) || len(fixture.pushes.closes) != 0 {
		t.Fatalf("got %v, closes %v", err, fixture.pushes.closes)
	}
}

func TestMessagePresenceBeforeAuthCloses(t *testing.T) {
	fixture := newGatewayFixture()
	reply, err := fixture.gateway.Message(GatewayMessage{ConnectionId: "c1", Body: `{"type":"presence","state":"idle"}`})
	refusedWith4401(t, fixture, "c1", reply, err)
}

func TestMessagePresenceAfterAuthIsAccepted(t *testing.T) {
	fixture := newGatewayFixture()
	fixture.authenticate("c1", "ana-token")
	reply, err := fixture.gateway.Message(GatewayMessage{ConnectionId: "c1", DomainId: "example.com", Body: `{"type":"presence","state":"idle"}`})
	if err != nil || reply != (GatewayReply{}) || len(fixture.pushes.closes) != 0 {
		t.Fatalf("got %+v, %v", reply, err)
	}
}

func TestMessageTypingReportsStorageFailuresWithoutClosing(t *testing.T) {
	fixture := newGatewayFixture()
	fixture.authenticate("c1", "ana-token")
	fixture.memory.failAll = true
	_, err := fixture.gateway.Message(GatewayMessage{ConnectionId: "c1", DomainId: "example.com", Body: `{"type":"typing","conversationId":"conversation-1"}`})
	if !errors.As(err, &Unavailable{}) || len(fixture.pushes.closes) != 0 {
		t.Fatalf("got %v", err)
	}
}

func TestDisconnectBeforeAuthDoesNothing(t *testing.T) {
	fixture := newGatewayFixture()
	fixture.authenticate("c1", "ana-token")
	err := fixture.gateway.Disconnect(GatewayDisconnect{ConnectionId: "c1"})
	_, found := fixture.gateway.Connections.Find("example.com", "c1")
	if err != nil || found != nil {
		t.Fatalf("got %v, %v", err, found)
	}
}

func TestDisconnectRemovesTheConnection(t *testing.T) {
	fixture := newGatewayFixture()
	fixture.authenticate("c1", "ana-token")
	err := fixture.gateway.Disconnect(GatewayDisconnect{ConnectionId: "c1", DomainId: "example.com"})
	_, found := fixture.gateway.Connections.Find("example.com", "c1")
	again := fixture.gateway.Disconnect(GatewayDisconnect{ConnectionId: "c1", DomainId: "example.com"})
	if err != nil || again != nil || !errors.As(found, &Missing{}) {
		t.Fatalf("got %v, %v, %v", err, again, found)
	}
}

func TestDisconnectReportsStorageFailures(t *testing.T) {
	fixture := newGatewayFixture()
	fixture.authenticate("c1", "ana-token")
	fixture.memory.failAll = true
	if err := fixture.gateway.Disconnect(GatewayDisconnect{ConnectionId: "c1", DomainId: "example.com"}); !errors.As(err, &Unavailable{}) {
		t.Fatalf("got %v", err)
	}
}
