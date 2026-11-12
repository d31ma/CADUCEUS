package core

import (
	"errors"
	"strings"
	"testing"
)

func TestMessageBudgetCountsTheBodyTheBlocksAndTheEscapedEventBody(t *testing.T) {
	size := MessageBudget{}.ContentBytes(`a"b`, `[]`)
	if size != len(`"a\"b"`)+len(`"[]"`)+len(`"\"a\\\"b\""`) {
		t.Fatalf("got %d", size)
	}
}

func TestMessageBudgetAcceptsContentAtTheCap(t *testing.T) {
	budget := MessageBudget{}
	body := strings.Repeat("a", budget.MaxContentBytes()/2-5)
	if budget.ContentBytes(body, "") != budget.MaxContentBytes() || budget.Check(body, "") != nil {
		t.Fatalf("content at the cap was refused")
	}
}

func TestMessageBudgetRefusesContentOverTheCap(t *testing.T) {
	body := strings.Repeat("a", MessageBudget{}.MaxContentBytes()/2-4)
	var tooLarge *TooLarge
	if !errors.As(MessageBudget{}.Check(body, ""), &tooLarge) {
		t.Fatal("content over the cap was accepted")
	}
}

func TestMessageBudgetEncodesBlocks(t *testing.T) {
	budget := MessageBudget{}
	encoded := budget.BlocksJson([]Block{{Kind: "paragraph", Spans: []Span{}, Cells: []string{}}})
	if encoded != `[{"kind":"paragraph","level":0,"ordered":false,"spans":[],"cells":[]}]` || budget.BlocksJson(nil) != "[]" {
		t.Fatalf("got %s", encoded)
	}
}
