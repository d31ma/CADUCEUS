package realtime

import (
	"testing"

	"mesail.dev/packages/events"
)

func TestSeenBookRemembersAnEventOnce(t *testing.T) {
	memory := newMemoryStorage()
	book := SeenBook{Storage: memory.storage()}
	before, _ := book.Seen("example.com", "evt_1")
	book.Remember("example.com", "evt_1")
	book.Remember("example.com", "evt_1")
	after, err := book.Seen("example.com", "evt_1")
	if before || !after || err != nil || memory.writes != 1 {
		t.Fatalf("before %v after %v writes %d err %v", before, after, memory.writes, err)
	}
}

func TestSeenBookKeepsDomainsApart(t *testing.T) {
	book := SeenBook{Storage: newMemoryStorage().storage()}
	book.Remember("example.com", "evt_1")
	seen, _ := book.Seen("example.org", "evt_1")
	if seen {
		t.Fatal("an event of another domain counted as seen")
	}
}

func TestSeenBookIsASeenStore(t *testing.T) {
	var store events.SeenStore = SeenBook{Storage: newMemoryStorage().storage()}
	if store == nil {
		t.Fatal("no store")
	}
}

func TestSeenBookPassesAStorageFailureOn(t *testing.T) {
	memory := newMemoryStorage()
	memory.failAll = true
	book := SeenBook{Storage: memory.storage()}
	_, err := book.Seen("example.com", "evt_1")
	if err == nil || book.Remember("example.com", "evt_1") == nil {
		t.Fatal("a failing storage was not reported")
	}
}
