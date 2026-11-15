package backups

import (
	"encoding/json"
	"errors"
	"testing"
)

func TestFindReadsPastAnEmptyFirstPage(t *testing.T) {
	fixture := newFixture(t)
	fixture.backedUp(t, ana, 0, "cipher")
	fixture.documents.emptyFirstPage = true
	mine, err := fixture.desk.Mine(ana)
	if err != nil || mine.Version != 1 {
		t.Fatalf("mine %+v %v", mine, err)
	}
}

func TestPagesTreatEmptyContentAsNoRows(t *testing.T) {
	pages := Pages{Scroll: func(string, map[string]any, string, int) (Window, error) { return Window{}, nil }}
	found, err := pages.First("c", map[string]any{}, &Record{})
	if found || err != nil {
		t.Fatalf("found %v err %v", found, err)
	}
}

func TestPagesReportBadContentAndFailures(t *testing.T) {
	bad := Pages{Scroll: func(string, map[string]any, string, int) (Window, error) {
		return Window{Content: json.RawMessage(`{"not":"a list"}`)}, nil
	}}
	failing := Pages{Scroll: func(string, map[string]any, string, int) (Window, error) { return Window{}, errors.New("down") }}
	_, badErr := bad.First("c", map[string]any{}, &Record{})
	_, failErr := failing.First("c", map[string]any{}, &Record{})
	if badErr == nil || failErr == nil {
		t.Fatalf("errors hidden: %v, %v", badErr, failErr)
	}
}

func TestPagesStopAfterTheLastPage(t *testing.T) {
	calls := 0
	endless := Pages{Scroll: func(string, map[string]any, string, int) (Window, error) {
		calls++
		return Window{Content: json.RawMessage(`[]`), NextCursor: "more"}, nil
	}}
	found, err := endless.First("c", map[string]any{}, &Record{})
	if found || err != nil || calls != 20 {
		t.Fatalf("found %v err %v calls %d", found, err, calls)
	}
}
