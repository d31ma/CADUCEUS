package storage

import (
	"errors"
	"testing"
	"time"

	"mesail.dev/packages/core"
)

func TestChangeRowsRegisterTheirMonthOnce(t *testing.T) {
	source := newMemorySource()
	storage := newStorage(source)
	changes := storage.Stores().Changes
	for _, eventId := range []string{"e1", "e2"} {
		if _, err := changes.Append(core.Change{DomainId: "example.com", ConversationId: "cnv_1", MessageId: "msg_1", EventId: eventId, Month: "202611"}); err != nil {
			t.Fatal(err)
		}
	}
	records := []ChangeLogRecord{}
	storage.documents().All(CollectionNames{}.ChangeLogs(), map[string]any{}, &records)
	byEvent, _ := changes.OfEvent("example.com", "202611", "e2")
	byConversation, _ := changes.OfConversation("example.com", "202611", "cnv_1")
	if len(records) != 1 || records[0].Month != "202611" || len(byEvent) != 1 || len(byConversation) != 2 {
		t.Fatalf("got %+v %v %v", records, byEvent, byConversation)
	}
}

func TestARegisteredMonthIsNotRegisteredAgain(t *testing.T) {
	source := newMemorySource()
	logs := newStorage(source).ChangeLogs()
	if err := logs.recordUnless(false, ChangeLogRecord{Name: "changes-x-202611", Month: "202611"}); err != nil {
		t.Fatal(err)
	}
	if err := logs.recordUnless(false, ChangeLogRecord{Name: "changes-x-202611", Month: "202611"}); err != nil {
		t.Fatal(err)
	}
	records, _ := logs.records()
	if len(records) != 1 {
		t.Fatalf("got %d records", len(records))
	}
}

func TestChangeHeadsAreFoundInsertedAndSaved(t *testing.T) {
	heads := newStorage(newMemorySource()).Stores().ChangeHeads
	id, err := heads.Insert(core.ChangeHead{DomainId: "example.com", ConversationId: "cnv_1", LastChangeId: "a"})
	if err != nil {
		t.Fatal(err)
	}
	heads.Save(core.ChangeHead{Id: id, DomainId: "example.com", ConversationId: "cnv_1", LastChangeId: "b"})
	head, found, err := heads.Find("example.com", "cnv_1")
	_, foreign, _ := heads.Find("example.org", "cnv_1")
	if err != nil || !found || foreign || head.LastChangeId != "b" {
		t.Fatalf("got %+v %v %v %v", head, found, foreign, err)
	}
}

func TestTheCleanupDropsMonthsBeforeTheOldestKeptOne(t *testing.T) {
	source := newMemorySource()
	storage := newStorage(source)
	changes := storage.Stores().Changes
	for _, month := range []string{"202608", "202609", "202610", "202611"} {
		changes.Append(core.Change{DomainId: "example.com", ConversationId: "cnv_1", EventId: month, Month: month})
	}
	report, err := storage.ChangeLogs().Cleanup(time.Date(2026, time.November, 14, 0, 0, 0, 0, time.UTC))
	records, _ := storage.ChangeLogs().records()
	_, kept := source.collections[CollectionNames{}.Changes("example.com", "202609")]
	_, dropped := source.collections[CollectionNames{}.Changes("example.com", "202608")]
	if err != nil || report.Dropped != 1 || report.More || len(records) != 3 || !kept || dropped {
		t.Fatalf("got %+v %v, %d records, kept %v dropped %v", report, err, len(records), kept, dropped)
	}
}

func TestTheCleanupDropsAtMostTheGivenNumberAndSaysMore(t *testing.T) {
	source := newMemorySource()
	storage := newStorage(source)
	for _, month := range []string{"202601", "202602", "202603"} {
		storage.Stores().Changes.Append(core.Change{DomainId: "example.com", EventId: month, Month: month})
	}
	report, err := storage.ChangeLogs().DropBefore("202611", 2)
	if err != nil || report.Dropped != 2 || !report.More {
		t.Fatalf("got %+v %v", report, err)
	}
}

func TestCleanupFailuresAreUnavailable(t *testing.T) {
	source := newMemorySource()
	storage := newStorage(source)
	storage.Stores().Changes.Append(core.Change{DomainId: "example.com", EventId: "e", Month: "202601"})
	source.failure = errors.New("fylo is down")
	_, err := storage.ChangeLogs().Cleanup(time.Date(2026, time.November, 14, 0, 0, 0, 0, time.UTC))
	var unavailable *core.Unavailable
	if !errors.As(err, &unavailable) {
		t.Fatalf("want Unavailable, got %v", err)
	}
}

func TestAFailedDropKeepsItsRecord(t *testing.T) {
	source := newMemorySource()
	storage := newStorage(source)
	logs := storage.ChangeLogs()
	logs.Source = droppingFails{source}
	storage.Stores().Changes.Append(core.Change{DomainId: "example.com", EventId: "e", Month: "202601"})
	report, err := logs.DropBefore("202611", 12)
	records, _ := logs.records()
	if err == nil || report.Dropped != 0 || len(records) != 1 {
		t.Fatalf("got %+v %v %d", report, err, len(records))
	}
}

func TestAnAppendFailsWhenTheRegistryCannotBeRead(t *testing.T) {
	source := newMemorySource()
	source.failure = errors.New("fylo is down")
	if _, err := newStorage(source).Stores().Changes.Append(core.Change{DomainId: "example.com", Month: "202611"}); err == nil {
		t.Fatal("want a failure")
	}
}

type droppingFails struct {
	*memorySource
}

func (droppingFails) DropCollection(name string) (bool, error) {
	return false, errors.New("drop failed")
}
