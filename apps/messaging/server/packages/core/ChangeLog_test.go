package core

import (
	"testing"
	"time"
)

func TestAChangeCursorReadsBackFromItsText(t *testing.T) {
	at := time.Date(2026, time.November, 10, 12, 0, 0, 123456789, time.UTC)
	cursor := ChangeCursor{At: at, RowId: "000000000007"}
	parsed, valid := ChangeCursors{}.Parse(cursor.Text())
	if !valid || !parsed.At.Equal(at) || parsed.RowId != "000000000007" {
		t.Fatalf("got %+v %v from %s", parsed, valid, cursor.Text())
	}
}

func TestTextsTheServerDidNotMakeAreNotCursors(t *testing.T) {
	for _, text := range []string{"", "abc", "123.", "x.y", "123"} {
		if _, valid := (ChangeCursors{}).Parse(text); valid {
			t.Fatalf("%q was read as a cursor", text)
		}
	}
}

func TestChangeCursorsOrderByTimeThenRow(t *testing.T) {
	at := time.Date(2026, time.November, 10, 12, 0, 0, 0, time.UTC)
	first := ChangeCursor{At: at, RowId: "2"}
	second := ChangeCursor{At: at, RowId: "3"}
	third := ChangeCursor{At: at.Add(time.Nanosecond), RowId: "1"}
	if !second.After(first) || !third.After(second) || first.After(first) || first.Compare(third) >= 0 {
		t.Fatal("cursors are out of order")
	}
}

func TestACursorOfARowUsesItsLogTime(t *testing.T) {
	at := time.Date(2026, time.November, 10, 12, 0, 0, 5, time.UTC)
	cursor := ChangeCursors{}.Of(Change{Id: "9", LoggedAt: ChangeCursors{}.Stamp(at)})
	if !cursor.At.Equal(at) || cursor.RowId != "9" {
		t.Fatalf("got %+v", cursor)
	}
}

func TestVersionsStartAtOneAndCountUp(t *testing.T) {
	versions := Versions{}
	got := []int{versions.First(), versions.Current(Message{}), versions.Next(Message{}), versions.Current(Message{Version: 3}), versions.Next(Message{Version: 3})}
	if got[0] != 1 || got[1] != 1 || got[2] != 2 || got[3] != 3 || got[4] != 4 {
		t.Fatalf("got %v", got)
	}
}

func TestTheLogKeepsTheCurrentAndTwoPreviousMonths(t *testing.T) {
	retention := ChangeRetention{}
	january := time.Date(2027, time.January, 20, 8, 0, 0, 0, time.UTC)
	if retention.FirstKeptMonth(january) != "202611" || !retention.KeptFrom(january).Equal(time.Date(2026, time.November, 1, 0, 0, 0, 0, time.UTC)) {
		t.Fatalf("got %s", retention.FirstKeptMonth(january))
	}
}

func TestMemoryChangesFindRowsByEventAndConversation(t *testing.T) {
	store := NewMemoryChanges()
	store.Append(Change{DomainId: "example.com", ConversationId: "cnv_1", EventId: "e1", Month: "202611"})
	store.Append(Change{DomainId: "example.com", ConversationId: "cnv_2", EventId: "e2", Month: "202611"})
	store.Append(Change{DomainId: "example.com", ConversationId: "cnv_1", EventId: "e3", Month: "202610"})
	byEvent, _ := store.OfEvent("example.com", "202611", "e1")
	byConversation, _ := store.OfConversation("example.com", "202611", "cnv_1")
	if len(byEvent) != 1 || len(byConversation) != 1 || len(store.All()) != 3 || byEvent[0].Id == "" {
		t.Fatalf("got %v %v", byEvent, byConversation)
	}
}

func TestMemoryChangeHeadsAreFoundAndSaved(t *testing.T) {
	store := NewMemoryChangeHeads()
	id, _ := store.Insert(ChangeHead{DomainId: "example.com", ConversationId: "cnv_1", LastChangeId: "a"})
	store.Insert(ChangeHead{DomainId: "example.com", ConversationId: "cnv_2", LastChangeId: "x"})
	store.Save(ChangeHead{Id: id, DomainId: "example.com", ConversationId: "cnv_1", LastChangeId: "b"})
	head, found, _ := store.Find("example.com", "cnv_1")
	_, missing, _ := store.Find("example.org", "cnv_1")
	if !found || missing || head.LastChangeId != "b" || len(store.All()) != 2 {
		t.Fatalf("got %+v %v %v", head, found, missing)
	}
}
