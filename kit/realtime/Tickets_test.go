package realtime

import (
	"crypto/rand"
	"errors"
	"testing"
)

func ticketsFixture() Tickets {
	memory := newMemoryStorage()
	now := fixedClock(connectionTestTime())
	return Tickets{Tokens: fixedTokens{grants: testGrants()}, Book: TicketBook{Storage: memory.storage(), Now: now, Random: rand.Reader}, Now: now}
}

func TestIssueGivesATicketForTheMember(t *testing.T) {
	tickets := ticketsFixture()
	ticket, err := tickets.Issue("Bearer ana-token")
	grant, redeemed := tickets.Book.Redeem(ticket.Ticket)
	if err != nil || redeemed != nil || ticket.ExpiresAt != "2026-11-10T12:00:30Z" || grant.MemberId != "ana" {
		t.Fatalf("got %+v %+v %v %v", ticket, grant, err, redeemed)
	}
}

func TestIssueRefusesAMissingOrBadToken(t *testing.T) {
	tickets := ticketsFixture()
	if _, err := tickets.Issue(""); !errors.Is(err, Unauthenticated{}) {
		t.Fatalf("missing: got %v", err)
	}
	if _, err := tickets.Issue("Bearer forged"); !errors.Is(err, Unauthenticated{}) {
		t.Fatalf("forged: got %v", err)
	}
}
