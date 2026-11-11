package realtime

import (
	"errors"
	"strconv"
	"testing"
	"time"
)

func TestSaveInsertsThenUpdatesTheSameMember(t *testing.T) {
	memory := newMemoryStorage()
	book := PresenceBook{Storage: memory.storage()}
	book.Save(PresenceRecord{DomainId: "example.com", MemberId: "ana", State: "active"})
	err := book.Save(PresenceRecord{DomainId: "example.com", MemberId: "ana", State: "idle"})
	record, found, _ := book.Find("example.com", "ana")
	all, _ := book.ForMembers("example.com", []string{"ana"})
	if err != nil || !found || record.State != "idle" || len(all) != 1 || memory.writes != 2 {
		t.Fatalf("got %+v %v %d %v", record, found, len(all), err)
	}
}

func TestFindAnswersNotFoundForAnUnknownMember(t *testing.T) {
	_, found, err := PresenceBook{Storage: newMemoryStorage().storage()}.Find("example.com", "nobody")
	if found || err != nil {
		t.Fatalf("got %v %v", found, err)
	}
}

func TestForMembersLeavesOutUnknownMembers(t *testing.T) {
	book := PresenceBook{Storage: newMemoryStorage().storage()}
	book.Save(PresenceRecord{DomainId: "example.com", MemberId: "ana", State: "active"})
	records, err := book.ForMembers("example.com", []string{"ana", "nobody"})
	if err != nil || len(records) != 1 || records[0].MemberId != "ana" {
		t.Fatalf("got %+v %v", records, err)
	}
}

func dueBook() PresenceBook {
	book := PresenceBook{Storage: newMemoryStorage().storage()}
	records := []PresenceRecord{
		{MemberId: "long-gone", State: "active", DisconnectedAt: presenceText(-61 * time.Second)},
		{MemberId: "just-gone", State: "idle", DisconnectedAt: presenceText(-30 * time.Second)},
		{MemberId: "connected", State: "active"},
		{MemberId: "offline", State: "offline", DisconnectedAt: presenceText(-time.Hour)},
		{MemberId: "dnd-ended", State: "dnd", DoNotDisturbUntil: presenceText(-time.Second)},
		{MemberId: "dnd-later", State: "dnd", DoNotDisturbUntil: presenceText(time.Hour)},
	}
	for _, record := range records {
		record.DomainId = "example.com"
		book.Save(record)
	}
	return book
}

func TestDueFindsLongDisconnectedMembersAndEndedDoNotDisturb(t *testing.T) {
	due, err := dueBook().Due("example.com", presenceText(0), 25)
	names := map[string]bool{}
	for _, record := range due {
		names[record.MemberId] = true
	}
	if err != nil || len(due) != 2 || !names["long-gone"] || !names["dnd-ended"] {
		t.Fatalf("got %+v %v", due, err)
	}
}

func TestDueStopsAtTheLimit(t *testing.T) {
	due, _ := dueBook().Due("example.com", presenceText(0), 1)
	none, _ := dueBook().Due("example.com", presenceText(0), 0)
	if len(due) != 1 || len(none) != 0 {
		t.Fatalf("got %d and %d", len(due), len(none))
	}
}

func TestDuePagesThroughMoreThanOnePage(t *testing.T) {
	book := PresenceBook{Storage: newMemoryStorage().storage()}
	for index := 0; index < 150; index++ {
		book.Save(PresenceRecord{DomainId: "example.com", MemberId: "m" + strconv.Itoa(index), State: "active", DisconnectedAt: presenceText(-2 * time.Minute)})
	}
	due, err := book.Due("example.com", presenceText(0), 120)
	if err != nil || len(due) != 120 {
		t.Fatalf("got %d %v", len(due), err)
	}
}

func TestDueRefusesATimeThatIsNotRfc3339(t *testing.T) {
	_, err := dueBook().Due("example.com", "yesterday", 25)
	if !errors.As(err, &Invalid{}) {
		t.Fatalf("got %v", err)
	}
}

func TestPresenceBookPassesAStorageFailureOn(t *testing.T) {
	memory := newMemoryStorage()
	memory.failAll = true
	book := PresenceBook{Storage: memory.storage()}
	_, _, findErr := book.Find("example.com", "ana")
	_, membersErr := book.ForMembers("example.com", []string{"ana"})
	_, dueErr := book.Due("example.com", presenceText(0), 25)
	if findErr == nil || membersErr == nil || dueErr == nil || book.Save(PresenceRecord{DomainId: "example.com", MemberId: "ana"}) == nil {
		t.Fatal("a failing storage was not reported")
	}
}

func TestPresenceDueKeepsOfflineMembersOut(t *testing.T) {
	rule := PresenceDue{Now: presenceNow()}
	offline := PresenceRecord{State: "offline", DisconnectedAt: presenceText(-time.Hour)}
	if rule.Due(offline) {
		t.Fatal("an offline member is due")
	}
}
