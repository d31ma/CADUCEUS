package ids

import (
	"errors"
	"testing"
)

func TestAddPutsThePrefixFirst(t *testing.T) {
	if got := (WireId{}).Add("msg", "01HX"); got != "msg_01HX" {
		t.Fatalf("got %q", got)
	}
}

func TestStripReturnsTheStoredId(t *testing.T) {
	got, err := WireId{}.Strip("msg", "msg_01HX")
	if err != nil || got != "01HX" {
		t.Fatalf("got %q, %v", got, err)
	}
}

func TestStripRefusesAnotherKindOfId(t *testing.T) {
	_, err := WireId{}.Strip("msg", "cnv_01HX")
	var wrong WrongIdPrefix
	if !errors.As(err, &wrong) || wrong.Expected != "msg" {
		t.Fatalf("got %v", err)
	}
}

func TestStripRefusesAnEmptyStoredId(t *testing.T) {
	if _, err := (WireId{}).Strip("msg", "msg_"); err == nil {
		t.Fatal("expected a refusal")
	}
}

func TestWrongIdPrefixNamesThePrefix(t *testing.T) {
	if got := (WrongIdPrefix{Expected: "msg"}).Error(); got != `the id does not start with "msg"` {
		t.Fatalf("got %q", got)
	}
}
