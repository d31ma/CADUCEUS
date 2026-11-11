package search

import (
	"errors"
	"testing"
)

func TestDocumentStoreInsertCreatesTheCollectionOnce(t *testing.T) {
	documents := newFakeDocuments()
	store := DocumentStore{Documents: documents}
	first, err := store.Insert("things", Month{DomainId: domainA, Month: "202611"})
	if err != nil {
		t.Fatal(err)
	}
	second, err := store.Insert("things", Month{DomainId: domainA, Month: "202610"})
	if err != nil || first == second || len(documents.rows["things"]) != 2 {
		t.Fatalf("got %q %q %v", first, second, err)
	}
}

func TestDocumentStoreFindAndReplace(t *testing.T) {
	documents := newFakeDocuments()
	store := DocumentStore{Documents: documents}
	id, _ := store.Insert("things", Month{DomainId: domainA, Month: "202611"})
	if err := store.Replace("things", id, Month{DomainId: domainA, Month: "202612"}); err != nil {
		t.Fatal(err)
	}
	found := Month{}
	err := store.Find("things", id, &found)
	if err != nil || found.Month != "202612" || found.DomainId != domainA {
		t.Fatalf("got %+v %v", found, err)
	}
}

func TestDocumentStoreAllOfAMissingCollectionIsEmpty(t *testing.T) {
	documents := newFakeDocuments()
	found := []Month{}
	err := DocumentStore{Documents: documents}.All("missing", map[string]any{}, &found)
	if err != nil || len(found) != 0 || len(documents.commands) != 0 {
		t.Fatalf("got %v %v %v", found, err, documents.commands)
	}
}

func TestDocumentStoreAllReadsEveryPage(t *testing.T) {
	documents := newFakeDocuments()
	for range 250 {
		documents.put("things", Month{DomainId: domainA, Month: "202611"})
	}
	documents.put("things", Month{DomainId: domainB, Month: "202611"})
	found := []Month{}
	err := DocumentStore{Documents: documents}.All("things", map[string]any{"domainId": domainA}, &found)
	if err != nil || len(found) != 250 || len(documents.commands) != 3 {
		t.Fatalf("got %d rows, %d commands, %v", len(found), len(documents.commands), err)
	}
	first := documents.commands[0]
	if _, present := first["cursor"]; present {
		t.Fatalf("the first scroll sends a cursor: %v", first)
	}
	if first["operation"] != "document" || first["command"] != "scroll" || first["collection"] != "things" || first["size"] != 100 {
		t.Fatalf("got %v", first)
	}
	if documents.commands[1]["cursor"] != "100" || documents.commands[2]["cursor"] != "200" {
		t.Fatalf("got %v %v", documents.commands[1], documents.commands[2])
	}
}

func TestDocumentStoreFailuresAreUnavailable(t *testing.T) {
	cases := []struct {
		name   string
		method string
		call   func(DocumentStore) error
	}{
		{"ensure exists check", "CollectionExists", func(store DocumentStore) error { return store.Ensure("fresh") }},
		{"ensure create", "CreateCollection", func(store DocumentStore) error { return store.Ensure("fresh") }},
		{"insert exists check", "CollectionExists", func(store DocumentStore) error {
			_, err := store.Insert("things", Month{})
			return err
		}},
		{"insert", "Insert", func(store DocumentStore) error {
			_, err := store.Insert("things", Month{})
			return err
		}},
		{"find", "FindById", func(store DocumentStore) error { return store.Find("things", "doc_1", &Month{}) }},
		{"replace", "Patch", func(store DocumentStore) error { return store.Replace("things", "doc_1", Month{}) }},
		{"all exists check", "CollectionExists", func(store DocumentStore) error { return store.All("things", map[string]any{}, &[]Month{}) }},
		{"all scroll", "Command", func(store DocumentStore) error { return store.All("things", map[string]any{}, &[]Month{}) }},
	}
	for _, test := range cases {
		t.Run(test.name, func(t *testing.T) {
			documents := newFakeDocuments()
			documents.put("things", Month{DomainId: domainA})
			documents.failNext(test.method)
			err := test.call(DocumentStore{Documents: documents})
			failure := Unavailable{}
			if !errors.As(err, &failure) || failure.Message != "the search index is not available" || !errors.Is(err, errFakeStore) {
				t.Fatalf("got %#v", err)
			}
		})
	}
}

func TestDocumentStoreAllFailsOnALaterPage(t *testing.T) {
	documents := newFakeDocuments()
	for range 150 {
		documents.put("things", Month{DomainId: domainA})
	}
	documents.failAt("Command", 1)
	err := DocumentStore{Documents: documents}.All("things", map[string]any{}, &[]Month{})
	if !errors.As(err, &Unavailable{}) || !errors.Is(err, errFakeStore) {
		t.Fatalf("got %#v", err)
	}
}

func TestDocumentStoreAllRefusesAnAnswerOfTheWrongShape(t *testing.T) {
	documents := newFakeDocuments()
	documents.put("things", Month{})
	documents.answer = "not a window"
	err := DocumentStore{Documents: documents}.All("things", map[string]any{}, &[]Month{})
	if !errors.As(err, &Unavailable{}) {
		t.Fatalf("got %#v", err)
	}
}

func TestDocumentStoreAllRefusesRowsThatDoNotFitTheTarget(t *testing.T) {
	documents := newFakeDocuments()
	documents.put("things", map[string]any{"month": 202611})
	err := DocumentStore{Documents: documents}.All("things", map[string]any{}, &[]Month{})
	if !errors.As(err, &Unavailable{}) {
		t.Fatalf("got %#v", err)
	}
}
