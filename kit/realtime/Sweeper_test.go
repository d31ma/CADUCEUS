package realtime

import (
	"errors"
	"reflect"
	"testing"
	"time"
)

func presenceSweeper(memory *memoryStorage, gateway *recordingGateway) Sweeper {
	keeper := presenceKeeper(memory, gateway)
	return Sweeper{Gate: presenceGate(), Domains: DomainBook{Storage: memory.storage()}, Connections: keeper.Connections, Pushes: gateway, Presence: keeper}
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
	sweeper := presenceSweeper(memory, gateway)
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
	err := presenceSweeper(newMemoryStorage(), gateway).closeOne(Connection{ConnectionId: "c1", DomainId: "example.com", MemberId: "ana"})
	if err != nil {
		t.Fatal(err)
	}
}

func TestClosingFailsWhileTheGatewayIsDown(t *testing.T) {
	gateway := newRecordingGateway()
	gateway.down = true
	_, err := presenceSweeper(newMemoryStorage(), gateway).closeAll([]Connection{{ConnectionId: "c1", DomainId: "example.com", MemberId: "ana"}})
	if !errors.As(err, &Unavailable{}) {
		t.Fatalf("got %v", err)
	}
}

func TestSweepingADomainSettlesItsPresence(t *testing.T) {
	memory := newMemoryStorage()
	sweeper := presenceSweeper(memory, newRecordingGateway())
	sweeper.Presence.Book.Save(PresenceRecord{DomainId: "example.com", MemberId: "ana", State: "idle", DisconnectedAt: presenceText(-2 * time.Minute)})
	report, err := sweeper.sweepDomains([]string{"example.com"}, SweepReport{}, presenceNow())
	record, _, _ := sweeper.Presence.Book.Find("example.com", "ana")
	if err != nil || record.State != "offline" || report.OfflineMembers != 1 || report.Writes != 1 {
		t.Fatalf("got %+v %+v %v", record, report, err)
	}
}

func TestSweepStopsWhenTheBudgetIsUsedUp(t *testing.T) {
	sweeper := presenceSweeper(newMemoryStorage(), newRecordingGateway())
	used := SweepReport{Writes: 25}
	report, err := sweeper.sweepDomains([]string{"example.com"}, used, presenceNow())
	expired, _ := sweeper.expired("example.com", presenceText(0), 0)
	if err != nil || report != used || !sweeper.more(used) || sweeper.more(SweepReport{Writes: 3}) || len(expired) != 0 || sweeper.morePages(0, "", used) {
		t.Fatalf("got %+v %v", report, err)
	}
}

func TestSweepPassesAStorageFailureOn(t *testing.T) {
	memory := newMemoryStorage()
	sweeper := presenceSweeper(memory, newRecordingGateway())
	memory.failAll = true
	_, err := sweeper.sweepDomains([]string{"example.com"}, SweepReport{}, presenceNow())
	if err == nil {
		t.Fatal("a failing storage was not reported")
	}
}

func TestSweepReportsAddUp(t *testing.T) {
	sum := SweepReports{}.Sum(SweepReport{ClosedConnections: 1, Writes: 2}, SweepReport{OfflineMembers: 1, EndedDoNotDisturb: 1, Writes: 2, More: true})
	if sum != (SweepReport{ClosedConnections: 1, OfflineMembers: 1, EndedDoNotDisturb: 1, Writes: 4, More: true}) {
		t.Fatalf("got %+v", sum)
	}
}
