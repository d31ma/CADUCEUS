package textindex

import (
	"errors"
	"testing"
)

func TestCursorSurvivesEncoding(t *testing.T) {
	cursor := Cursor{CreatedAt: "2026-11-10T10:00:00Z", DocumentId: "abc"}
	decoded, err := Cursor{}.Decode(cursor.Encode())
	if err != nil || decoded != cursor {
		t.Fatalf("got %v %v", decoded, err)
	}
}

func TestBadBase64IsRefused(t *testing.T) {
	if _, err := (Cursor{}).Decode("%%%"); !errors.Is(err, BadCursor{}) {
		t.Fatalf("got %v", err)
	}
}

func TestBadJsonIsRefused(t *testing.T) {
	if _, err := (Cursor{}).Decode("bm90LWpzb24"); !errors.Is(err, BadCursor{}) {
		t.Fatalf("got %v", err)
	}
}

func TestBadCursorSaysWhatIsWrong(t *testing.T) {
	if (BadCursor{}).Error() == "" {
		t.Fatal("expected a message")
	}
}

func TestStartCursorAdmitsEverything(t *testing.T) {
	if !(Cursor{}).Admits("2026-11-10T10:00:00Z", "x") {
		t.Fatal("expected the start cursor to admit the item")
	}
}

func TestCursorAdmitsOnlyOlderItems(t *testing.T) {
	cursor := Cursor{CreatedAt: "2026-11-10T10:00:00Z", DocumentId: "m"}
	if !cursor.Admits("2026-11-10T09:00:00Z", "z") {
		t.Fatal("expected an older item to be admitted")
	}
	if cursor.Admits("2026-11-10T11:00:00Z", "a") {
		t.Fatal("expected a newer item to be refused")
	}
}

func TestCursorBreaksTiesById(t *testing.T) {
	cursor := Cursor{CreatedAt: "2026-11-10T10:00:00Z", DocumentId: "m"}
	if !cursor.Admits("2026-11-10T10:00:00Z", "a") || cursor.Admits("2026-11-10T10:00:00Z", "m") {
		t.Fatal("expected ties to be ordered by id")
	}
}
