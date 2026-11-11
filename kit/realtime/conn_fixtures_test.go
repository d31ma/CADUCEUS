package realtime

import (
	"crypto/rand"
	"time"
)

type gatewayFixture struct {
	memory  *memoryStorage
	pushes  *recordingGateway
	gateway Gateway
}

func connectionTestTime() time.Time {
	return time.Date(2026, 11, 10, 12, 0, 0, 0, time.UTC)
}

func connectionTestBook(memory *memoryStorage) ConnectionBook {
	return ConnectionBook{Storage: memory.storage(), Now: fixedClock(connectionTestTime())}
}

func testConnection(domainId string, memberId string, connectionId string) Connection {
	return Connection{ConnectionId: connectionId, DomainId: domainId, MemberId: memberId, Kind: "socket", ExpiresAt: "2026-11-10T12:01:00Z"}
}

func testGrants() map[string]Grant {
	return map[string]Grant{
		"ana-token":   {DomainId: "example.com", MemberId: "ana", ExpiresAt: "2026-11-10T12:01:00Z"},
		"ana-later":   {DomainId: "example.com", MemberId: "ana", ExpiresAt: "2026-11-10T12:02:00Z"},
		"ben-token":   {DomainId: "example.com", MemberId: "ben", ExpiresAt: "2026-11-10T12:01:00Z"},
		"other-token": {DomainId: "example.org", MemberId: "ana", ExpiresAt: "2026-11-10T12:01:00Z"},
	}
}

func newGatewayFixture() gatewayFixture {
	memory := newMemoryStorage()
	pushes := newRecordingGateway()
	storage := memory.storage()
	connections := connectionTestBook(memory)
	fanOut := FanOut{Connections: connections, Gateway: pushes}
	presence := PresenceKeeper{Book: PresenceBook{Storage: storage}, Connections: connections, FanOut: fanOut, Now: connections.Now}
	sessions := Sessions{
		Tokens:      fixedTokens{grants: testGrants()},
		Tickets:     TicketBook{Storage: storage, Now: connections.Now, Random: rand.Reader},
		Connections: connections,
		Domains:     DomainBook{Storage: storage},
		Presence:    presence,
	}
	typing := Typing{Members: MemberBook{Storage: storage}, FanOut: fanOut}
	gateway := Gateway{Sessions: sessions, Typing: typing, Presence: presence, Connections: connections, Pushes: pushes}
	return gatewayFixture{memory: memory, pushes: pushes, gateway: gateway}
}

func (fixture gatewayFixture) authenticate(connectionId string, token string) GatewayReply {
	reply, err := fixture.gateway.Message(GatewayMessage{ConnectionId: connectionId, Body: `{"type":"auth","token":"` + token + `"}`})
	if err != nil {
		panic(err)
	}
	return reply
}
