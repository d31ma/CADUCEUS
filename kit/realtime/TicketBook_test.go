package realtime

import (
	"crypto/rand"
	"crypto/sha256"
	"encoding/hex"
	"errors"
	"strings"
	"testing"
	"time"
)

func ticketTestBook(memory *memoryStorage, at time.Time) TicketBook {
	return TicketBook{Storage: memory.storage(), Now: fixedClock(at), Random: rand.Reader}
}

func ticketTestGrant() Grant {
	return Grant{DomainId: "example.com", MemberId: "ana", ExpiresAt: "2026-11-10T12:01:00Z"}
}

func TestIssueStoresOnlyTheHashOfTheTicket(t *testing.T) {
	memory := newMemoryStorage()
	ticket, err := ticketTestBook(memory, connectionTestTime()).Issue(ticketTestGrant())
	stored := []TicketRecord{}
	memory.storage().Page(Query{Collection: "realtime-tickets", Size: 10}, &stored)
	digest := sha256.Sum256([]byte(ticket.Ticket))
	if err != nil || len(ticket.Ticket) != 43 || ticket.ExpiresAt != "2026-11-10T12:00:30Z" || len(stored) != 1 {
		t.Fatalf("got %+v, %v, %d records", ticket, err, len(stored))
	}
	record := stored[0]
	if record.TicketHash != hex.EncodeToString(digest[:]) || strings.Contains(record.TicketHash, ticket.Ticket) || record.TokenExpiresAt != "2026-11-10T12:01:00Z" || record.UsedAt != "" {
		t.Fatalf("got %+v", record)
	}
}

func TestRedeemGivesTheGrantOnlyOnce(t *testing.T) {
	book := ticketTestBook(newMemoryStorage(), connectionTestTime())
	ticket, _ := book.Issue(ticketTestGrant())
	grant, err := book.Redeem(ticket.Ticket)
	if err != nil || grant != ticketTestGrant() {
		t.Fatalf("got %+v, %v", grant, err)
	}
	if _, err := book.Redeem(ticket.Ticket); !errors.Is(err, Unauthenticated{}) {
		t.Fatalf("second redeem: got %v", err)
	}
}

func TestRedeemRefusesAnExpiredTicket(t *testing.T) {
	memory := newMemoryStorage()
	ticket, _ := ticketTestBook(memory, connectionTestTime()).Issue(ticketTestGrant())
	later := ticketTestBook(memory, connectionTestTime().Add(30*time.Second))
	if _, err := later.Redeem(ticket.Ticket); !errors.Is(err, Unauthenticated{}) {
		t.Fatalf("got %v", err)
	}
}

func TestRedeemRefusesATicketWhoseTokenEnded(t *testing.T) {
	book := ticketTestBook(newMemoryStorage(), connectionTestTime())
	grant := ticketTestGrant()
	grant.ExpiresAt = "2026-11-10T12:00:00Z"
	ticket, _ := book.Issue(grant)
	if _, err := book.Redeem(ticket.Ticket); !errors.Is(err, Unauthenticated{}) {
		t.Fatalf("got %v", err)
	}
}

func TestRedeemRefusesAnUnknownTicket(t *testing.T) {
	book := ticketTestBook(newMemoryStorage(), connectionTestTime())
	if _, err := book.Redeem("unknown"); !errors.Is(err, Unauthenticated{}) {
		t.Fatalf("no collection: got %v", err)
	}
	book.Issue(ticketTestGrant())
	if _, err := book.Redeem("unknown"); !errors.Is(err, Unauthenticated{}) {
		t.Fatalf("got %v", err)
	}
}

func TestIssueReportsAFailedRandomSource(t *testing.T) {
	book := ticketTestBook(newMemoryStorage(), connectionTestTime())
	book.Random = strings.NewReader("short")
	if _, err := book.Issue(ticketTestGrant()); !errors.As(err, &Unavailable{}) {
		t.Fatalf("got %v", err)
	}
}

func TestTicketBookReportsStorageFailures(t *testing.T) {
	memory := newMemoryStorage()
	book := ticketTestBook(memory, connectionTestTime())
	ticket, _ := book.Issue(ticketTestGrant())
	memory.failAll = true
	if _, err := book.Issue(ticketTestGrant()); !errors.As(err, &Unavailable{}) {
		t.Fatalf("issue: got %v", err)
	}
	if _, err := book.Redeem(ticket.Ticket); !errors.As(err, &Unavailable{}) {
		t.Fatalf("redeem: got %v", err)
	}
}

func TestTicketBookReportsFailedWritesAndScrolls(t *testing.T) {
	memory := newMemoryStorage()
	book := ticketTestBook(memory, connectionTestTime())
	ticket, _ := book.Issue(ticketTestGrant())
	book.Storage.Documents = failingWrites{memory}
	if _, err := book.Issue(ticketTestGrant()); !errors.As(err, &Unavailable{}) {
		t.Fatalf("insert: got %v", err)
	}
	if _, err := book.Redeem(ticket.Ticket); !errors.As(err, &Unavailable{}) {
		t.Fatalf("use: got %v", err)
	}
	book.Storage.Scroll = func(collection string, where map[string]any, cursor string, size int) (Window, error) {
		return Window{}, errors.New("down")
	}
	if _, err := book.Redeem(ticket.Ticket); !errors.As(err, &Unavailable{}) {
		t.Fatalf("find: got %v", err)
	}
}
