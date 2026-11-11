package realtime

import (
	"testing"
	"time"
)

func TestTypingReachesTheOtherMembersOnly(t *testing.T) {
	fixture := newGatewayFixture()
	fixture.authenticate("c1", "ana-token")
	fixture.authenticate("c2", "ben-token")
	fixture.authenticate("c3", "other-token")
	MemberBook{Storage: fixture.memory.storage()}.Replace("example.com", "conversation-1", []string{"ana", "ben"})
	_, err := fixture.gateway.Message(GatewayMessage{ConnectionId: "c1", DomainId: "example.com", Body: `{"type":"typing","conversationId":"conversation-1"}`})
	typed := fixture.pushes.eventsOfType("c2", "typing")
	if err != nil || len(typed) != 1 || typed[0].MemberId != "ana" || typed[0].ConversationId != "conversation-1" {
		t.Fatalf("got %v %+v", err, typed)
	}
	if len(fixture.pushes.eventsOfType("c1", "typing")) != 0 || len(fixture.pushes.eventsOfType("c3", "typing")) != 0 {
		t.Fatalf("typing reached the typist or another domain: %+v", fixture.pushes.pushes)
	}
}

func TestPresenceReachesOnlyTheSameDomain(t *testing.T) {
	fixture := newGatewayFixture()
	fixture.authenticate("c1", "ana-token")
	fixture.authenticate("c2", "ben-token")
	fixture.authenticate("c3", "other-token")
	_, err := fixture.gateway.Message(GatewayMessage{ConnectionId: "c1", DomainId: "example.com", Body: `{"type":"presence","state":"idle"}`})
	idle := fixture.pushes.eventsOfType("c2", "presence.changed")
	if err != nil || len(idle) == 0 || idle[len(idle)-1] != (Event{Type: "presence.changed", MemberId: "ana", State: "idle"}) {
		t.Fatalf("got %v %+v", err, idle)
	}
	for _, event := range fixture.pushes.eventsOf("c3") {
		if event.Type == "presence.changed" && event.State == "idle" {
			t.Fatalf("idle reached another domain")
		}
	}
}

func TestLastDisconnectMarksTheMemberAndASecondConnectionKeepsThemActive(t *testing.T) {
	fixture := newGatewayFixture()
	fixture.authenticate("c1", "ana-token")
	fixture.authenticate("c2", "ana-later")
	book := PresenceBook{Storage: fixture.memory.storage()}
	fixture.gateway.Disconnect(GatewayDisconnect{ConnectionId: "c1", DomainId: "example.com"})
	stillOpen, _, _ := book.Find("example.com", "ana")
	fixture.gateway.Disconnect(GatewayDisconnect{ConnectionId: "c2", DomainId: "example.com"})
	closed, _, _ := book.Find("example.com", "ana")
	if stillOpen.DisconnectedAt != "" || closed.DisconnectedAt == "" || closed.State != "active" {
		t.Fatalf("got %+v then %+v", stillOpen, closed)
	}
}

func TestSettleTurnsAMemberOfflineSixtySecondsAfterTheLastConnection(t *testing.T) {
	fixture := newGatewayFixture()
	fixture.authenticate("c1", "ana-token")
	fixture.authenticate("c2", "ben-token")
	fixture.gateway.Disconnect(GatewayDisconnect{ConnectionId: "c1", DomainId: "example.com"})
	keeper := fixture.gateway.Presence
	keeper.Now = fixedClock(connectionTestTime().Add(61 * time.Second))
	report, err := keeper.Settle("example.com", keeper.Now().UTC().Format(time.RFC3339), 25)
	record, _, _ := PresenceBook{Storage: fixture.memory.storage()}.Find("example.com", "ana")
	offline := fixture.pushes.eventsOfType("c2", "presence.changed")
	if err != nil || report.OfflineMembers != 1 || record.State != "offline" || offline[len(offline)-1].State != "offline" {
		t.Fatalf("got %v %+v %+v %+v", err, report, record, offline)
	}
}

func TestEventFanOutPushesToTheListedMembersInTheEnvelopeDomain(t *testing.T) {
	fixture := newGatewayFixture()
	fixture.authenticate("c1", "ana-token")
	fixture.authenticate("c2", "ben-token")
	fixture.authenticate("c3", "other-token")
	desk := EventDesk{Seen: SeenBook{Storage: fixture.memory.storage()}, Members: MemberBook{Storage: fixture.memory.storage()}, FanOut: fixture.gateway.Typing.FanOut}
	envelope := presenceEnvelope("evt_1", "message.created", "example.com", map[string]any{"messageId": "msg_1", "conversationId": "cnv_1", "authorMemberId": "ana", "memberIds": []string{"ana", "ben"}})
	err := desk.Handle(envelope)
	created := fixture.pushes.eventsOfType("c2", "message.created")
	if err != nil || len(created) != 1 || created[0].MessageId != "msg_1" || len(fixture.pushes.eventsOfType("c3", "message.created")) != 0 {
		t.Fatalf("got %v %+v", err, fixture.pushes.pushes)
	}
}

func TestSweepClosesAnExpiredConnectionWithAWarning(t *testing.T) {
	fixture := newGatewayFixture()
	fixture.authenticate("c1", "ana-token")
	fixture.authenticate("c2", "ben-later")
	connections := fixture.gateway.Connections
	connections.Now = fixedClock(connectionTestTime().Add(90 * time.Second))
	keeper := fixture.gateway.Presence
	keeper.Connections = connections
	keeper.Now = connections.Now
	sweeper := Sweeper{Gate: presenceGate(), Domains: DomainBook{Storage: fixture.memory.storage()}, Connections: connections, Pushes: fixture.pushes, Presence: keeper}
	report, err := sweeper.Run("Bearer operators")
	_, missing := connections.Find("example.com", "c1")
	if err != nil || report.ClosedConnections != 1 || fixture.pushes.closes["c1"] != 4401 || len(fixture.pushes.eventsOfType("c1", "auth.expired")) != 1 || missing == nil {
		t.Fatalf("got %v %+v closes=%v", err, report, fixture.pushes.closes)
	}
	if _, kept := fixture.pushes.closes["c2"]; kept {
		t.Fatalf("closed a connection that has not expired")
	}
}

func TestFanOutSkipsARecordThatIsStillBeingWritten(t *testing.T) {
	fixture := newGatewayFixture()
	fixture.authenticate("c1", "ana-token")
	collection := fixture.gateway.Connections.collection("example.com")
	fixture.memory.Insert(collection, Connection{DomainId: "example.com"})
	err := fixture.gateway.Typing.FanOut.ToDomain("example.com", Event{Type: "presence.changed", MemberId: "ben", State: "idle"})
	if err != nil || len(fixture.pushes.eventsOfType("c1", "presence.changed")) == 0 {
		t.Fatalf("got %v %+v", err, fixture.pushes.pushes)
	}
	if _, pushed := fixture.pushes.pushes[""]; pushed {
		t.Fatalf("pushed to an empty connection id")
	}
}

func TestExpiredLeavesOutARecordWithoutAnId(t *testing.T) {
	memory := newMemoryStorage()
	book := connectionTestBook(memory)
	book.Add(testConnection("example.com", "ana", "c1"))
	memory.Insert(book.collection("example.com"), Connection{DomainId: "example.com"})
	expired, err := book.Expired("example.com", "2030-01-01T00:00:00Z", 25)
	if err != nil || len(expired) != 1 || expired[0].ConnectionId != "c1" {
		t.Fatalf("got %+v %v", expired, err)
	}
}
