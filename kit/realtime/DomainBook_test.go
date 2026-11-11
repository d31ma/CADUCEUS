package realtime

import (
	"errors"
	"testing"
)

func TestRememberKeepsEachDomainOnce(t *testing.T) {
	book := DomainBook{Storage: newMemoryStorage().storage()}
	first := book.Remember("example.com")
	second := book.Remember("example.com")
	domainIds, cursor, err := book.List("", 10)
	if first != nil || second != nil || err != nil || cursor != "" || len(domainIds) != 1 || domainIds[0] != "example.com" {
		t.Fatalf("got %v %q %v %v %v", domainIds, cursor, err, first, second)
	}
}

func TestListPagesThroughTheDomains(t *testing.T) {
	book := DomainBook{Storage: newMemoryStorage().storage()}
	book.Remember("a.com")
	book.Remember("b.com")
	book.Remember("c.com")
	first, cursor, err := book.List("", 2)
	second, end, again := book.List(cursor, 2)
	if err != nil || again != nil || len(first) != 2 || cursor == "" || len(second) != 1 || second[0] != "c.com" || end != "" {
		t.Fatalf("got %v %q %v %q", first, cursor, second, end)
	}
}

func TestListWithoutDomainsIsEmpty(t *testing.T) {
	domainIds, cursor, err := DomainBook{Storage: newMemoryStorage().storage()}.List("", 10)
	if err != nil || cursor != "" || len(domainIds) != 0 {
		t.Fatalf("got %v %q %v", domainIds, cursor, err)
	}
}

func TestDomainBookReportsStorageFailures(t *testing.T) {
	memory := newMemoryStorage()
	book := DomainBook{Storage: memory.storage()}
	book.Remember("example.com")
	book.Storage.Documents = failingWrites{memory}
	if err := book.Remember("example.org"); !errors.As(err, &Unavailable{}) {
		t.Fatalf("insert: got %v", err)
	}
	memory.failAll = true
	if err := book.Remember("example.org"); !errors.As(err, &Unavailable{}) {
		t.Fatalf("remember: got %v", err)
	}
	if _, _, err := book.List("", 10); !errors.As(err, &Unavailable{}) {
		t.Fatalf("list: got %v", err)
	}
}

func TestDomainBookReportsAFailedScroll(t *testing.T) {
	book := DomainBook{Storage: newMemoryStorage().storage()}
	book.Remember("example.com")
	book.Storage.Scroll = func(collection string, where map[string]any, cursor string, size int) (Window, error) {
		return Window{}, errors.New("cursor expired")
	}
	if _, _, err := book.List("old", 10); !errors.As(err, &Unavailable{}) {
		t.Fatalf("list: got %v", err)
	}
	if err := book.Remember("example.org"); !errors.As(err, &Unavailable{}) {
		t.Fatalf("remember: got %v", err)
	}
}

func TestEnsureAcceptsACollectionMadeByAnotherRequest(t *testing.T) {
	memory := newMemoryStorage()
	memory.CreateCollection("realtime-domains")
	keeper := CollectionKeeper{Documents: failingWrites{memory}}
	if err := keeper.settled("realtime-domains", errors.New("already exists")); err != nil {
		t.Fatalf("got %v", err)
	}
	if err := keeper.Ensure("other-collection"); !errors.As(err, &Unavailable{}) {
		t.Fatalf("got %v", err)
	}
}
