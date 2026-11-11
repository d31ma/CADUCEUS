package realtime

import (
	"errors"
	"strconv"
	"testing"
)

func TestAddStoresTheConnectionOnceAndStampsTheTime(t *testing.T) {
	memory := newMemoryStorage()
	book := connectionTestBook(memory)
	first := book.Add(testConnection("example.com", "ana", "c1"))
	second := book.Add(testConnection("example.com", "ana", "c1"))
	found, err := book.Find("example.com", "c1")
	all, _ := book.InDomain("example.com")
	if first != nil || second != nil || err != nil || len(all) != 1 || found.MemberId != "ana" || found.ConnectedAt != "2026-11-10T12:00:00Z" {
		t.Fatalf("got %+v %v %v %v %d", found, first, second, err, len(all))
	}
}

func TestFindReportsAMissingConnection(t *testing.T) {
	book := connectionTestBook(newMemoryStorage())
	if _, err := book.Find("example.com", "c1"); !errors.As(err, &Missing{}) {
		t.Fatalf("no collection: got %v", err)
	}
	book.Add(testConnection("example.com", "ana", "c1"))
	if _, err := book.Find("example.com", "c2"); !errors.As(err, &Missing{}) {
		t.Fatalf("other id: got %v", err)
	}
}

func TestFindDoesNotLookInAnotherDomain(t *testing.T) {
	book := connectionTestBook(newMemoryStorage())
	book.Add(testConnection("example.com", "ana", "c1"))
	if _, err := book.Find("example.org", "c1"); !errors.As(err, &Missing{}) {
		t.Fatalf("got %v", err)
	}
}

func TestRenewMovesTheExpiry(t *testing.T) {
	book := connectionTestBook(newMemoryStorage())
	book.Add(testConnection("example.com", "ana", "c1"))
	err := book.Renew("example.com", "c1", "2026-11-10T12:05:00Z")
	found, _ := book.Find("example.com", "c1")
	if err != nil || found.ExpiresAt != "2026-11-10T12:05:00Z" {
		t.Fatalf("got %+v, %v", found, err)
	}
	if err := book.Renew("example.com", "c9", "2026-11-10T12:05:00Z"); !errors.As(err, &Missing{}) {
		t.Fatalf("unknown connection: got %v", err)
	}
}

func TestRemoveDeletesAndReturnsTheRecord(t *testing.T) {
	book := connectionTestBook(newMemoryStorage())
	book.Add(testConnection("example.com", "ana", "c1"))
	removed, err := book.Remove("example.com", "c1")
	_, missing := book.Find("example.com", "c1")
	if err != nil || removed.MemberId != "ana" || !errors.As(missing, &Missing{}) {
		t.Fatalf("got %+v, %v, %v", removed, err, missing)
	}
	again, err := book.Remove("example.com", "c1")
	if err != nil || again != (Connection{}) {
		t.Fatalf("second remove: got %+v, %v", again, err)
	}
}

func TestRemoveWithoutACollectionIsNotAnError(t *testing.T) {
	removed, err := connectionTestBook(newMemoryStorage()).Remove("example.com", "c1")
	if err != nil || removed != (Connection{}) {
		t.Fatalf("got %+v, %v", removed, err)
	}
}

func TestCountForMemberCountsOnlyThatMember(t *testing.T) {
	book := connectionTestBook(newMemoryStorage())
	book.Add(testConnection("example.com", "ana", "c1"))
	book.Add(testConnection("example.com", "ana", "c2"))
	book.Add(testConnection("example.com", "ben", "c3"))
	count, err := book.CountForMember("example.com", "ana")
	if err != nil || count != 2 {
		t.Fatalf("got %d, %v", count, err)
	}
}

func TestForMembersReturnsOnlyTheGivenMembersOnce(t *testing.T) {
	book := connectionTestBook(newMemoryStorage())
	book.Add(testConnection("example.com", "ana", "c1"))
	book.Add(testConnection("example.com", "ben", "c2"))
	book.Add(testConnection("example.com", "cid", "c3"))
	book.Add(testConnection("example.org", "ana", "c4"))
	found, err := book.ForMembers("example.com", []string{"ana", "ben", "ana"})
	if err != nil || len(found) != 2 || found[0].ConnectionId != "c1" || found[1].ConnectionId != "c2" {
		t.Fatalf("got %+v, %v", found, err)
	}
}

func TestInDomainReadsEveryPage(t *testing.T) {
	book := connectionTestBook(newMemoryStorage())
	for index := range 150 {
		book.Add(testConnection("example.com", "ana", "c"+strconv.Itoa(index)))
	}
	found, err := book.InDomain("example.com")
	if err != nil || len(found) != 150 {
		t.Fatalf("got %d, %v", len(found), err)
	}
}

func TestExpiredReturnsAtMostLimitPastConnections(t *testing.T) {
	book := connectionTestBook(newMemoryStorage())
	for index, expiresAt := range []string{"2026-11-10T11:59:00Z", "2026-11-10T12:00:00Z", "2026-11-10T11:58:00Z", "2026-11-10T12:00:01Z"} {
		connection := testConnection("example.com", "ana", "c"+strconv.Itoa(index))
		connection.ExpiresAt = expiresAt
		book.Add(connection)
	}
	all, err := book.Expired("example.com", "2026-11-10T12:00:00Z", 10)
	limited, _ := book.Expired("example.com", "2026-11-10T12:00:00Z", 2)
	if err != nil || len(all) != 3 || len(limited) != 2 {
		t.Fatalf("got %+v, %+v, %v", all, limited, err)
	}
}

func TestConnectionBookReportsStorageFailures(t *testing.T) {
	memory := newMemoryStorage()
	book := connectionTestBook(memory)
	book.Add(testConnection("example.com", "ana", "c1"))
	memory.failAll = true
	if err := book.Add(testConnection("example.com", "ana", "c2")); !errors.As(err, &Unavailable{}) {
		t.Fatalf("add: got %v", err)
	}
	if _, err := book.InDomain("example.com"); !errors.As(err, &Unavailable{}) {
		t.Fatalf("in domain: got %v", err)
	}
	if _, err := book.Expired("example.com", "2026-11-10T12:00:00Z", 5); !errors.As(err, &Unavailable{}) {
		t.Fatalf("expired: got %v", err)
	}
	if _, err := book.ForMembers("example.com", []string{"ana"}); !errors.As(err, &Unavailable{}) {
		t.Fatalf("for members: got %v", err)
	}
}

func TestConnectionBookReportsAFailedScroll(t *testing.T) {
	memory := newMemoryStorage()
	book := connectionTestBook(memory)
	book.Add(testConnection("example.com", "ana", "c1"))
	book.Storage.Scroll = func(collection string, where map[string]any, cursor string, size int) (Window, error) {
		return Window{}, errors.New("down")
	}
	if _, err := book.Remove("example.com", "c1"); !errors.As(err, &Unavailable{}) {
		t.Fatalf("got %v", err)
	}
	if err := book.Add(testConnection("example.com", "ana", "c1")); !errors.As(err, &Unavailable{}) {
		t.Fatalf("add: got %v", err)
	}
}

func TestConnectionBookReportsFailedWrites(t *testing.T) {
	memory := newMemoryStorage()
	book := connectionTestBook(memory)
	book.Add(testConnection("example.com", "ana", "c1"))
	book.Storage.Documents = failingWrites{memory}
	if err := book.Renew("example.com", "c1", "2026-11-10T12:05:00Z"); !errors.As(err, &Unavailable{}) {
		t.Fatalf("renew: got %v", err)
	}
	if _, err := book.Remove("example.com", "c1"); !errors.As(err, &Unavailable{}) {
		t.Fatalf("remove: got %v", err)
	}
	if err := book.Add(testConnection("example.com", "ana", "c2")); !errors.As(err, &Unavailable{}) {
		t.Fatalf("insert: got %v", err)
	}
}

func TestAddAgainReplacesTheOwnerAndTheEndTime(t *testing.T) {
	memory := newMemoryStorage()
	book := connectionTestBook(memory)
	stale := testConnection("example.com", "ana", "c1")
	stale.ExpiresAt = "2026-11-10T11:00:00Z"
	book.Add(stale)
	fresh := testConnection("example.com", "ben", "c1")
	fresh.ExpiresAt = "2026-11-10T12:01:00Z"
	err := book.Add(fresh)
	found, _ := book.Find("example.com", "c1")
	count, _ := book.CountForMember("example.com", "ana")
	if err != nil || found.MemberId != "ben" || found.ExpiresAt != "2026-11-10T12:01:00Z" || count != 0 {
		t.Fatalf("got %+v %v, ana still has %d", found, err, count)
	}
}
