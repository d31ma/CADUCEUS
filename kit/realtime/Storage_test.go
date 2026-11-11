package realtime

import (
	"encoding/json"
	"errors"
	"testing"
)

func TestPageDecodesTheWindowAndReturnsTheCursor(t *testing.T) {
	storage := Storage{Scroll: func(collection string, where map[string]any, cursor string, size int) (Window, error) {
		return Window{Content: json.RawMessage(`[{"domainId":"example.com"}]`), NextCursor: "next"}, nil
	}}
	found := []DomainRecord{}
	cursor, err := storage.Page(Query{Collection: "realtime-domains", Size: 10}, &found)
	if err != nil || cursor != "next" || len(found) != 1 {
		t.Fatalf("got %q %v %v", cursor, found, err)
	}
}

func TestPagePassesTheScrollFailureOn(t *testing.T) {
	failure := errors.New("down")
	storage := Storage{Scroll: func(collection string, where map[string]any, cursor string, size int) (Window, error) {
		return Window{}, failure
	}}
	_, err := storage.Page(Query{Collection: "realtime-domains", Size: 10}, &[]DomainRecord{})
	if !errors.Is(err, failure) {
		t.Fatalf("got %v", err)
	}
}
