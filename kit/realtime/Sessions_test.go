package realtime

import (
	"errors"
	"testing"
)

func TestAuthenticateOpensASocketConnection(t *testing.T) {
	fixture := newGatewayFixture()
	sessions := fixture.gateway.Sessions
	reply, err := sessions.Authenticate(GatewayMessage{ConnectionId: "c1"}, ClientMessage{Type: "auth", Token: "ana-token"})
	stored, found := sessions.Connections.Find("example.com", "c1")
	domainIds, _, _ := sessions.Domains.List("", 10)
	if err != nil || reply != (GatewayReply{DomainId: "example.com", ExpiresAt: "2026-11-10T12:01:00Z"}) {
		t.Fatalf("got %+v, %v", reply, err)
	}
	if found != nil || stored.MemberId != "ana" || stored.Kind != "socket" || stored.ExpiresAt != "2026-11-10T12:01:00Z" || len(domainIds) != 1 {
		t.Fatalf("got %+v, %v, %v", stored, found, domainIds)
	}
}

func TestAuthenticateOpensAStreamWithATicket(t *testing.T) {
	fixture := newGatewayFixture()
	sessions := fixture.gateway.Sessions
	ticket, _ := sessions.Tickets.Issue(testGrants()["ben-token"])
	reply, err := sessions.Authenticate(GatewayMessage{ConnectionId: "s1"}, ClientMessage{Type: "ticket", Ticket: ticket.Ticket})
	stored, _ := sessions.Connections.Find("example.com", "s1")
	if err != nil || reply.DomainId != "example.com" || stored.MemberId != "ben" || stored.Kind != "stream" {
		t.Fatalf("got %+v, %+v, %v", reply, stored, err)
	}
	if _, err := sessions.Authenticate(GatewayMessage{ConnectionId: "s2"}, ClientMessage{Type: "ticket", Ticket: ticket.Ticket}); !errors.Is(err, Unauthenticated{}) {
		t.Fatalf("reused ticket: got %v", err)
	}
}

func TestAuthenticateRefusesABadTokenAndOtherTypes(t *testing.T) {
	sessions := newGatewayFixture().gateway.Sessions
	if _, err := sessions.Authenticate(GatewayMessage{ConnectionId: "c1"}, ClientMessage{Type: "auth", Token: "forged"}); !errors.Is(err, Unauthenticated{}) {
		t.Fatalf("forged: got %v", err)
	}
	if _, err := sessions.Authenticate(GatewayMessage{ConnectionId: "c1"}, ClientMessage{Type: "typing", Token: "ana-token"}); !errors.Is(err, Unauthenticated{}) {
		t.Fatalf("typing: got %v", err)
	}
	if _, err := sessions.Connections.Find("example.com", "c1"); !errors.As(err, &Missing{}) {
		t.Fatalf("stored a refused connection: %v", err)
	}
}

func TestAuthenticateRenewsTheSameMember(t *testing.T) {
	sessions := newGatewayFixture().gateway.Sessions
	sessions.Authenticate(GatewayMessage{ConnectionId: "c1"}, ClientMessage{Type: "auth", Token: "ana-token"})
	reply, err := sessions.Authenticate(GatewayMessage{ConnectionId: "c1", DomainId: "example.com"}, ClientMessage{Type: "auth", Token: "ana-later"})
	stored, _ := sessions.Connections.Find("example.com", "c1")
	if err != nil || reply.ExpiresAt != "2026-11-10T12:02:00Z" || stored.ExpiresAt != "2026-11-10T12:02:00Z" {
		t.Fatalf("got %+v, %+v, %v", reply, stored, err)
	}
}

func TestAuthenticateRefusesARenewalWithAnotherMembersToken(t *testing.T) {
	sessions := newGatewayFixture().gateway.Sessions
	sessions.Authenticate(GatewayMessage{ConnectionId: "c1"}, ClientMessage{Type: "auth", Token: "ana-token"})
	_, err := sessions.Authenticate(GatewayMessage{ConnectionId: "c1", DomainId: "example.com"}, ClientMessage{Type: "auth", Token: "ben-token"})
	stored, _ := sessions.Connections.Find("example.com", "c1")
	if !errors.Is(err, Unauthenticated{}) || stored.MemberId != "ana" {
		t.Fatalf("got %+v, %v", stored, err)
	}
}

func TestAuthenticateRefusesARenewalFromAnotherDomain(t *testing.T) {
	sessions := newGatewayFixture().gateway.Sessions
	sessions.Authenticate(GatewayMessage{ConnectionId: "c1"}, ClientMessage{Type: "auth", Token: "ana-token"})
	if _, err := sessions.Authenticate(GatewayMessage{ConnectionId: "c1", DomainId: "example.com"}, ClientMessage{Type: "auth", Token: "other-token"}); !errors.Is(err, Unauthenticated{}) {
		t.Fatalf("got %v", err)
	}
}

func TestAuthenticateRefusesARenewalOfAnUnknownConnection(t *testing.T) {
	sessions := newGatewayFixture().gateway.Sessions
	if _, err := sessions.Authenticate(GatewayMessage{ConnectionId: "c9", DomainId: "example.com"}, ClientMessage{Type: "auth", Token: "ana-token"}); !errors.Is(err, Unauthenticated{}) {
		t.Fatalf("got %v", err)
	}
}

func TestAuthenticateReportsStorageFailures(t *testing.T) {
	fixture := newGatewayFixture()
	sessions := fixture.gateway.Sessions
	sessions.Authenticate(GatewayMessage{ConnectionId: "c1"}, ClientMessage{Type: "auth", Token: "ana-token"})
	fixture.memory.failAll = true
	if _, err := sessions.Authenticate(GatewayMessage{ConnectionId: "c2"}, ClientMessage{Type: "auth", Token: "ana-token"}); !errors.As(err, &Unavailable{}) {
		t.Fatalf("open: got %v", err)
	}
	if _, err := sessions.Authenticate(GatewayMessage{ConnectionId: "c1", DomainId: "example.com"}, ClientMessage{Type: "auth", Token: "ana-token"}); !errors.As(err, &Unavailable{}) {
		t.Fatalf("renew: got %v", err)
	}
}

func TestAuthenticateReportsAFailedConnectionWrite(t *testing.T) {
	fixture := newGatewayFixture()
	sessions := fixture.gateway.Sessions
	sessions.Domains.Remember("example.com")
	sessions.Connections.Storage.Documents = failingWrites{fixture.memory}
	if _, err := sessions.Authenticate(GatewayMessage{ConnectionId: "c1"}, ClientMessage{Type: "auth", Token: "ana-token"}); !errors.As(err, &Unavailable{}) {
		t.Fatalf("got %v", err)
	}
}
