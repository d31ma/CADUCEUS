package realtime

import (
	"errors"
	"reflect"
	"testing"
	"time"
)

func presenceDomainSweep(memory *memoryStorage, gateway *recordingGateway) DomainSweep {
	keeper := presenceKeeper(memory, gateway)
	return DomainSweep{Book: DomainBook{Storage: memory.storage()}, Connections: keeper.Connections, Pushes: gateway, Presence: keeper}
}

func presenceSweeper(memory *memoryStorage, gateway *recordingGateway) Sweeper {
	tickets := TicketSweep{Book: ticketTestBook(memory, presenceNow())}
	return Sweeper{Gate: presenceGate(), Domains: presenceDomainSweep(memory, gateway), Tickets: tickets}
}

func TestSweepRefusesAMissingToken(t *testing.T) {
	_, err := presenceSweeper(newMemoryStorage(), newRecordingGateway()).Run("")
	if !errors.Is(err, Unauthenticated{}) {
		t.Fatalf("got %v", err)
	}
}

func TestSweepAnswersMissingForADomainServiceToken(t *testing.T) {
	_, err := presenceSweeper(newMemoryStorage(), newRecordingGateway()).Run("Bearer service")
	if !errors.As(err, &Missing{}) {
		t.Fatalf("got %v", err)
	}
}

func TestSweepRefusesAnOperatorsCallerWithoutTheServiceRole(t *testing.T) {
	_, err := presenceSweeper(newMemoryStorage(), newRecordingGateway()).Run("Bearer operator")
	if !errors.Is(err, Forbidden{}) {
		t.Fatalf("got %v", err)
	}
}

func TestSweepRunsForTheOperatorsService(t *testing.T) {
	report, err := presenceSweeper(newMemoryStorage(), newRecordingGateway()).Run("Bearer operators")
	if err != nil || report.More {
		t.Fatalf("got %+v %v", report, err)
	}
}

func TestClosingAnExpiredConnectionWarnsClosesAndMarksTheMember(t *testing.T) {
	memory := newMemoryStorage()
	gateway := newRecordingGateway()
	sweeper := presenceDomainSweep(memory, gateway)
	sweeper.Presence.Connected("example.com", "ana")
	report, err := sweeper.closeAll([]Connection{{ConnectionId: "c1", DomainId: "example.com", MemberId: "ana"}})
	record, _, _ := sweeper.Presence.Book.Find("example.com", "ana")
	if err != nil || !reflect.DeepEqual(gateway.eventsOf("c1"), []Event{{Type: "auth.expired", State: "4401"}}) || gateway.closes["c1"] != 4401 || record.DisconnectedAt != presenceText(0) || report != (SweepReport{ClosedConnections: 1, Writes: 2}) {
		t.Fatalf("got %v %v %+v %+v %v", gateway.eventsOf("c1"), gateway.closes, record, report, err)
	}
}

func TestClosingAConnectionThatIsGoneIsFine(t *testing.T) {
	gateway := newRecordingGateway()
	gateway.gone["c1"] = true
	err := presenceDomainSweep(newMemoryStorage(), gateway).closeOne(Connection{ConnectionId: "c1", DomainId: "example.com", MemberId: "ana"})
	if err != nil {
		t.Fatal(err)
	}
}

func TestClosingFailsWhileTheGatewayIsDown(t *testing.T) {
	gateway := newRecordingGateway()
	gateway.down = true
	_, err := presenceDomainSweep(newMemoryStorage(), gateway).closeAll([]Connection{{ConnectionId: "c1", DomainId: "example.com", MemberId: "ana"}})
	if !errors.As(err, &Unavailable{}) {
		t.Fatalf("got %v", err)
	}
}

func TestSweepingADomainSettlesItsPresence(t *testing.T) {
	memory := newMemoryStorage()
	sweeper := presenceDomainSweep(memory, newRecordingGateway())
	sweeper.Presence.Book.Save(PresenceRecord{DomainId: "example.com", MemberId: "ana", State: "idle", DisconnectedAt: presenceText(-2 * time.Minute)})
	report, err := sweeper.sweepDomains([]string{"example.com"}, SweepReport{}, presenceNow())
	record, _, _ := sweeper.Presence.Book.Find("example.com", "ana")
	if err != nil || record.State != "offline" || report.OfflineMembers != 1 || report.Writes != 1 {
		t.Fatalf("got %+v %+v %v", record, report, err)
	}
}

func TestSweepStopsWhenTheBudgetIsUsedUp(t *testing.T) {
	memory := newMemoryStorage()
	sweeper := presenceSweeper(memory, newRecordingGateway())
	domains := presenceDomainSweep(memory, newRecordingGateway())
	used := SweepReport{Writes: 25}
	report, err := domains.sweepDomains([]string{"example.com"}, used, presenceNow())
	expired, _ := domains.expired("example.com", presenceText(0), 0)
	if err != nil || report != used || !sweeper.more(used) || sweeper.more(SweepReport{Writes: 3}) || len(expired) != 0 || domains.morePages(0, "", used) {
		t.Fatalf("got %+v %v", report, err)
	}
}

func TestSweepPassesAStorageFailureOn(t *testing.T) {
	memory := newMemoryStorage()
	sweeper := presenceDomainSweep(memory, newRecordingGateway())
	memory.failAll = true
	_, err := sweeper.sweepDomains([]string{"example.com"}, SweepReport{}, presenceNow())
	if err == nil {
		t.Fatal("a failing storage was not reported")
	}
}

func TestSweepReportsAddUp(t *testing.T) {
	sum := SweepReports{}.Sum(SweepReport{ClosedConnections: 1, DeletedTickets: 1, Writes: 3}, SweepReport{OfflineMembers: 1, EndedDoNotDisturb: 1, DeletedTickets: 2, Writes: 4, More: true})
	if sum != (SweepReport{ClosedConnections: 1, OfflineMembers: 1, EndedDoNotDisturb: 1, DeletedTickets: 3, Writes: 7, More: true}) {
		t.Fatalf("got %+v", sum)
	}
}

func sweepTicket(book TicketBook, usedAt string, expiresAt string) {
	book.keeper().Ensure(book.collection())
	book.Storage.Documents.Insert(book.collection(), TicketRecord{TicketHash: expiresAt + usedAt, DomainId: "example.com", MemberId: "ana", ExpiresAt: expiresAt, TokenExpiresAt: expiresAt, UsedAt: usedAt})
}

func remainingTickets(book TicketBook) []TicketRecord {
	found, _ := book.all()
	return found
}

func TestSweepDeletesUsedAndExpiredTicketsOnly(t *testing.T) {
	memory := newMemoryStorage()
	sweeper := presenceSweeper(memory, newRecordingGateway())
	book := sweeper.Tickets.Book
	sweepTicket(book, presenceText(-time.Second), presenceText(20*time.Second))
	sweepTicket(book, "", presenceText(-time.Second))
	sweepTicket(book, "", presenceText(20*time.Second))
	report, err := sweeper.Run("Bearer operators")
	left := remainingTickets(book)
	if err != nil || report.DeletedTickets != 2 || report.Writes != 2 || report.More || len(left) != 1 || left[0].ExpiresAt != presenceText(20*time.Second) {
		t.Fatalf("got %+v %v %+v", report, err, left)
	}
}

func TestTicketCleaningKeepsToTheWritesThatAreLeft(t *testing.T) {
	memory := newMemoryStorage()
	sweep := presenceSweeper(memory, newRecordingGateway()).Tickets
	sweepTicket(sweep.Book, "", presenceText(-time.Second))
	sweepTicket(sweep.Book, "", presenceText(-2*time.Second))
	report, err := sweep.Clean(1)
	nothing, _ := sweep.Clean(0)
	if err != nil || report != (SweepReport{DeletedTickets: 1, Writes: 1}) || nothing != (SweepReport{}) || len(remainingTickets(sweep.Book)) != 1 {
		t.Fatalf("got %+v %+v %v", report, nothing, err)
	}
}

func TestTicketCleaningPassesAStorageFailureOn(t *testing.T) {
	memory := newMemoryStorage()
	sweep := presenceSweeper(memory, newRecordingGateway()).Tickets
	sweepTicket(sweep.Book, "", presenceText(-time.Second))
	memory.failAll = true
	_, err := sweep.Clean(5)
	removeErr := sweep.Book.Remove(TicketRecord{ID: "missing"})
	if err == nil || !errors.As(removeErr, &Unavailable{}) {
		t.Fatalf("got %v %v", err, removeErr)
	}
}

func TestTicketCleaningSkipsARecordWithoutAnId(t *testing.T) {
	book := presenceSweeper(newMemoryStorage(), newRecordingGateway()).Tickets.Book
	stale := book.staleBy([]TicketRecord{{ExpiresAt: presenceText(-time.Second)}}, presenceNow())
	if len(stale) != 0 {
		t.Fatalf("got %+v", stale)
	}
}
