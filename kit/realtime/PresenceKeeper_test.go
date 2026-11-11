package realtime

import (
	"errors"
	"strconv"
	"testing"
	"time"
)

func storedPresence(keeper PresenceKeeper, memberId string) PresenceRecord {
	record, _, _ := keeper.Book.Find("example.com", memberId)
	return record
}

func TestSetRefusesABadStateWithoutWriting(t *testing.T) {
	memory := newMemoryStorage()
	err := presenceKeeper(memory, newRecordingGateway()).Set("example.com", "ana", PresenceRequest{State: "away"})
	if !errors.As(err, &Invalid{}) || memory.writes != 0 {
		t.Fatalf("got %v, %d writes", err, memory.writes)
	}
}

func TestSetStoresDoNotDisturbWithItsEndAndTheLastSeenTime(t *testing.T) {
	keeper := presenceKeeper(newMemoryStorage(), newRecordingGateway())
	err := keeper.Set("example.com", "ana", PresenceRequest{State: "dnd", DoNotDisturbUntil: presenceText(time.Hour)})
	record := storedPresence(keeper, "ana")
	if err != nil || record.State != "dnd" || record.DoNotDisturbUntil != presenceText(time.Hour) || record.LastSeenAt != presenceText(0) || record.DomainId != "example.com" {
		t.Fatalf("got %+v %v", record, err)
	}
}

func TestSetClearsTheDoNotDisturbEndForOtherStates(t *testing.T) {
	keeper := presenceKeeper(newMemoryStorage(), newRecordingGateway())
	keeper.Set("example.com", "ana", PresenceRequest{State: "dnd", DoNotDisturbUntil: presenceText(time.Hour)})
	keeper.Set("example.com", "ana", PresenceRequest{State: "idle", DoNotDisturbUntil: presenceText(time.Hour)})
	if record := storedPresence(keeper, "ana"); record.State != "idle" || record.DoNotDisturbUntil != "" {
		t.Fatalf("got %+v", record)
	}
}

func TestPresenceIsAnnouncedOnlyWhenTheStateOrTheEndChanged(t *testing.T) {
	keeper := PresenceKeeper{}
	active := PresenceRecord{State: "active"}
	if keeper.announced(active, active) || !keeper.announced(active, PresenceRecord{State: "idle"}) || !keeper.announced(PresenceRecord{State: "dnd", DoNotDisturbUntil: "a"}, PresenceRecord{State: "dnd", DoNotDisturbUntil: "b"}) {
		t.Fatal("wrong announce decision")
	}
}

func TestConnectedMakesANewMemberActive(t *testing.T) {
	keeper := presenceKeeper(newMemoryStorage(), newRecordingGateway())
	err := keeper.Connected("example.com", "ana")
	if record := storedPresence(keeper, "ana"); err != nil || record.State != "active" || record.LastSeenAt != presenceText(0) {
		t.Fatalf("got %+v %v", record, err)
	}
}

func TestConnectedWakesAnOfflineMember(t *testing.T) {
	keeper := presenceKeeper(newMemoryStorage(), newRecordingGateway())
	keeper.Book.Save(PresenceRecord{DomainId: "example.com", MemberId: "ana", State: "offline", DisconnectedAt: presenceText(-time.Hour)})
	keeper.Connected("example.com", "ana")
	if record := storedPresence(keeper, "ana"); record.State != "active" || record.DisconnectedAt != "" {
		t.Fatalf("got %+v", record)
	}
}

func TestConnectedKeepsTheStateAndClearsTheDisconnectTime(t *testing.T) {
	keeper := presenceKeeper(newMemoryStorage(), newRecordingGateway())
	keeper.Book.Save(PresenceRecord{DomainId: "example.com", MemberId: "ana", State: "idle", DisconnectedAt: presenceText(-10 * time.Second)})
	keeper.Connected("example.com", "ana")
	if record := storedPresence(keeper, "ana"); record.State != "idle" || record.DisconnectedAt != "" {
		t.Fatalf("got %+v", record)
	}
}

func TestConnectedWritesNothingForAConnectedMember(t *testing.T) {
	memory := newMemoryStorage()
	keeper := presenceKeeper(memory, newRecordingGateway())
	keeper.Book.Save(PresenceRecord{DomainId: "example.com", MemberId: "ana", State: "idle"})
	keeper.Connected("example.com", "ana")
	if memory.writes != 1 {
		t.Fatalf("got %d writes", memory.writes)
	}
}

func TestDisconnectedMarksTheTimeOnce(t *testing.T) {
	memory := newMemoryStorage()
	keeper := presenceKeeper(memory, newRecordingGateway())
	keeper.Connected("example.com", "ana")
	err := keeper.Disconnected("example.com", "ana")
	keeper.Disconnected("example.com", "ana")
	if record := storedPresence(keeper, "ana"); err != nil || record.DisconnectedAt != presenceText(0) || record.State != "active" || memory.writes != 2 {
		t.Fatalf("got %+v %v, %d writes", record, err, memory.writes)
	}
}

func TestDisconnectedWritesNothingForAnUnknownOrOfflineMember(t *testing.T) {
	memory := newMemoryStorage()
	keeper := presenceKeeper(memory, newRecordingGateway())
	keeper.Book.Save(PresenceRecord{DomainId: "example.com", MemberId: "bo", State: "offline"})
	keeper.Disconnected("example.com", "ana")
	keeper.Disconnected("example.com", "bo")
	if memory.writes != 1 {
		t.Fatalf("got %d writes", memory.writes)
	}
}

func TestDisconnectedWaitsWhileAnotherConnectionIsOpen(t *testing.T) {
	keeper := PresenceKeeper{Now: presenceNow}
	if keeper.marksDisconnect(PresenceRecord{State: "active"}, true, 1) || !keeper.marksDisconnect(PresenceRecord{State: "active"}, true, 0) {
		t.Fatal("wrong disconnect decision")
	}
}

func TestQueryRefusesAnEmptyList(t *testing.T) {
	_, err := presenceKeeper(newMemoryStorage(), newRecordingGateway()).Query("example.com", []string{"", ""})
	if !errors.As(err, &Invalid{}) {
		t.Fatalf("got %v", err)
	}
}

func TestQueryRefusesMoreThanOneHundredIds(t *testing.T) {
	memberIds := []string{}
	for index := 0; index < 101; index++ {
		memberIds = append(memberIds, "m"+strconv.Itoa(index))
	}
	_, err := presenceKeeper(newMemoryStorage(), newRecordingGateway()).Query("example.com", memberIds)
	if !errors.As(err, &Invalid{}) {
		t.Fatalf("got %v", err)
	}
}

func TestQueryLeavesOutUnknownIdsAndRepeats(t *testing.T) {
	keeper := presenceKeeper(newMemoryStorage(), newRecordingGateway())
	keeper.Set("example.com", "ana", PresenceRequest{State: "idle"})
	found, err := keeper.Query("example.com", []string{"ana", "nobody", "ana"})
	if err != nil || len(found) != 1 || found[0].MemberId != "ana" || found[0].State != "idle" || found[0].LastSeenAt != presenceText(0) {
		t.Fatalf("got %+v %v", found, err)
	}
}

func TestQueryAnswersMissingForAMemberOfAnotherDomain(t *testing.T) {
	keeper := presenceKeeper(newMemoryStorage(), newRecordingGateway())
	keeper.Set("example.org", "ana", PresenceRequest{State: "idle"})
	_, err := keeper.Query("example.com", []string{"ana"})
	if !errors.As(err, &Missing{}) {
		t.Fatalf("got %v", err)
	}
}

func TestQueryShowsAnEndedDoNotDisturbAsOfflineWithoutConnections(t *testing.T) {
	keeper := presenceKeeper(newMemoryStorage(), newRecordingGateway())
	keeper.Book.Save(PresenceRecord{DomainId: "example.com", MemberId: "ana", State: "dnd", DoNotDisturbUntil: presenceText(-time.Minute)})
	found, _ := keeper.Query("example.com", []string{"ana"})
	if found[0].State != "offline" || found[0].DoNotDisturbUntil != "" {
		t.Fatalf("got %+v", found)
	}
}

func TestQueryShowsTheDoNotDisturbEndWhileItLasts(t *testing.T) {
	keeper := presenceKeeper(newMemoryStorage(), newRecordingGateway())
	keeper.Set("example.com", "ana", PresenceRequest{State: "dnd", DoNotDisturbUntil: presenceText(time.Hour)})
	found, _ := keeper.Query("example.com", []string{"ana"})
	if found[0].State != "dnd" || found[0].DoNotDisturbUntil != presenceText(time.Hour) {
		t.Fatalf("got %+v", found)
	}
}

func TestAnEndedDoNotDisturbShowsActiveWhileConnected(t *testing.T) {
	keeper := PresenceKeeper{}
	if keeper.shownState("dnd", true, 1) != "active" || keeper.shownState("dnd", true, 0) != "offline" || keeper.shownState("idle", false, 0) != "idle" {
		t.Fatal("wrong shown state")
	}
}

func TestSettleTurnsALongDisconnectedMemberOffline(t *testing.T) {
	keeper := presenceKeeper(newMemoryStorage(), newRecordingGateway())
	keeper.Book.Save(PresenceRecord{DomainId: "example.com", MemberId: "ana", State: "active", DisconnectedAt: presenceText(-61 * time.Second)})
	report, err := keeper.Settle("example.com", presenceText(0), 25)
	record := storedPresence(keeper, "ana")
	if err != nil || record.State != "offline" || record.DisconnectedAt != "" || report != (SweepReport{Writes: 1, OfflineMembers: 1}) {
		t.Fatalf("got %+v %+v %v", record, report, err)
	}
}

func TestSettleEndsAPastDoNotDisturb(t *testing.T) {
	keeper := presenceKeeper(newMemoryStorage(), newRecordingGateway())
	keeper.Book.Save(PresenceRecord{DomainId: "example.com", MemberId: "ana", State: "dnd", DoNotDisturbUntil: presenceText(-time.Second)})
	report, _ := keeper.Settle("example.com", presenceText(0), 25)
	record := storedPresence(keeper, "ana")
	if record.State != "offline" || record.DoNotDisturbUntil != "" || report != (SweepReport{Writes: 1, EndedDoNotDisturb: 1}) {
		t.Fatalf("got %+v %+v", record, report)
	}
}

func TestSettleLeavesMembersWhoJustLeftAlone(t *testing.T) {
	keeper := presenceKeeper(newMemoryStorage(), newRecordingGateway())
	keeper.Book.Save(PresenceRecord{DomainId: "example.com", MemberId: "ana", State: "active", DisconnectedAt: presenceText(-30 * time.Second)})
	report, _ := keeper.Settle("example.com", presenceText(0), 25)
	if record := storedPresence(keeper, "ana"); record.State != "active" || report.Writes != 0 || report.More {
		t.Fatalf("got %+v %+v", record, report)
	}
}

func TestSettleSaysMoreWhenTheBudgetIsUsedUp(t *testing.T) {
	keeper := presenceKeeper(newMemoryStorage(), newRecordingGateway())
	keeper.Book.Save(PresenceRecord{DomainId: "example.com", MemberId: "ana", State: "active", DisconnectedAt: presenceText(-time.Hour)})
	keeper.Book.Save(PresenceRecord{DomainId: "example.com", MemberId: "bo", State: "idle", DisconnectedAt: presenceText(-time.Hour)})
	report, _ := keeper.Settle("example.com", presenceText(0), 1)
	if report.Writes != 1 || !report.More {
		t.Fatalf("got %+v", report)
	}
}

func TestSettledKeepsAReconnectedMemberAndWakesAnEndedDoNotDisturb(t *testing.T) {
	keeper := PresenceKeeper{}
	reconnected := keeper.settled(PresenceRecord{State: "idle", DisconnectedAt: "x"}, false, 1)
	woken := keeper.settled(PresenceRecord{State: "dnd", DoNotDisturbUntil: "x"}, true, 1)
	if reconnected != (PresenceRecord{State: "idle"}) || woken != (PresenceRecord{State: "active"}) || keeper.tally(reconnected, false) != (SweepReport{Writes: 1}) {
		t.Fatalf("got %+v %+v", reconnected, woken)
	}
}

func TestPresenceKeeperPassesAStorageFailureOn(t *testing.T) {
	memory := newMemoryStorage()
	keeper := presenceKeeper(memory, newRecordingGateway())
	memory.failAll = true
	_, queryErr := keeper.Query("example.com", []string{"ana"})
	_, settleErr := keeper.Settle("example.com", presenceText(0), 25)
	if queryErr == nil || settleErr == nil || keeper.Set("example.com", "ana", PresenceRequest{State: "idle"}) == nil || keeper.Connected("example.com", "ana") == nil || keeper.Disconnected("example.com", "ana") == nil {
		t.Fatal("a failing storage was not reported")
	}
}
