package search

import (
	"errors"
	"testing"
)

func storageOver(documents *fakeDocuments) IndexStorage {
	return IndexStorage{Store: DocumentStore{Documents: documents}}
}

func TestIndexStorageFindRefusesAnEntryOfAnotherDomain(t *testing.T) {
	documents := newFakeDocuments()
	id := documents.put(Collections{}.Documents(domainA, "202611"), Document{DomainId: domainB, MessageId: "msg_1"})
	_, err := storageOver(documents).Find(Location{DomainId: domainA, Month: "202611", DocumentId: id})
	failure := Unavailable{}
	if !errors.As(err, &failure) || failure.Message != "the index entry belongs to another domain" {
		t.Fatalf("got %#v", err)
	}
}

func TestIndexStorageFindReturnsTheEntryOfTheSameDomain(t *testing.T) {
	documents := newFakeDocuments()
	id := documents.put(Collections{}.Documents(domainA, "202611"), Document{DomainId: domainA, MessageId: "msg_1"})
	document, err := storageOver(documents).Find(Location{DomainId: domainA, Month: "202611", DocumentId: id})
	if err != nil || document.MessageId != "msg_1" {
		t.Fatalf("got %+v %v", document, err)
	}
}

func TestIndexStorageAddRegistersEachMonthOnce(t *testing.T) {
	documents := newFakeDocuments()
	storage := storageOver(documents)
	for _, document := range []Document{{DomainId: domainA, MessageId: "msg_1"}, {DomainId: domainA, MessageId: "msg_2"}} {
		if err := storage.Add(document, "202611"); err != nil {
			t.Fatal(err)
		}
	}
	if err := storage.Add(Document{DomainId: domainA, MessageId: "msg_3"}, "202610"); err != nil {
		t.Fatal(err)
	}
	months, err := storage.Months(domainA)
	sameIds(t, months, "202611", "202610")
	if err != nil {
		t.Fatal(err)
	}
	location, found, err := storage.Location(domainA, "msg_2")
	if err != nil || !found || location.Month != "202611" || location.DocumentId == "" || !storage.Live(location) {
		t.Fatalf("got %+v %v %v", location, found, err)
	}
}

func TestIndexStorageAddFailures(t *testing.T) {
	cases := []struct {
		name   string
		method string
		skip   int
	}{
		{"month lookup", "CollectionExists", 0},
		{"month insert", "Insert", 0},
		{"document insert", "Insert", 1},
		{"location insert", "Insert", 2},
	}
	for _, test := range cases {
		t.Run(test.name, func(t *testing.T) {
			documents := newFakeDocuments()
			documents.failAt(test.method, test.skip)
			err := storageOver(documents).Add(Document{DomainId: domainA, MessageId: "msg_1"}, "202611")
			if !errors.As(err, &Unavailable{}) || !errors.Is(err, errFakeStore) {
				t.Fatalf("got %#v", err)
			}
		})
	}
}

func TestIndexStorageBuriedLocationIsNotLive(t *testing.T) {
	documents := newFakeDocuments()
	storage := storageOver(documents)
	storage.Add(Document{DomainId: domainA, MessageId: "msg_1"}, "202611")
	if err := storage.Bury(domainA, "msg_1"); err != nil {
		t.Fatal(err)
	}
	location, found, _ := storage.Location(domainA, "msg_1")
	if !found || !location.Deleted || location.DocumentId == "" || storage.Live(location) {
		t.Fatalf("got %+v", location)
	}
}

func TestIndexStorageUnknownLocationIsNotLive(t *testing.T) {
	storage := storageOver(newFakeDocuments())
	location, found, err := storage.Location(domainA, "msg_none")
	if err != nil || found || storage.Live(location) {
		t.Fatalf("got %+v %v %v", location, found, err)
	}
}

func TestIndexStorageDocumentsStayInTheirDomain(t *testing.T) {
	documents := newFakeDocuments()
	collection := Collections{}.Documents(domainA, "202611")
	documents.put(collection, Document{DomainId: domainA, MessageId: "msg_a"})
	documents.put(collection, Document{DomainId: domainB, MessageId: "msg_b"})
	scoped, err := storageOver(documents).Documents(domainA, "202611", map[string]any{})
	if err != nil || len(scoped) != 1 || scoped[0].MessageId != "msg_a" {
		t.Fatalf("got %+v %v", scoped, err)
	}
}
