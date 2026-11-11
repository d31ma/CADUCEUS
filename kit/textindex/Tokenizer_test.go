package textindex

import (
	"slices"
	"testing"
)

func TestTermsAreLowercaseWords(t *testing.T) {
	got := Tokenizer{}.Terms("Hello, World!")
	if !slices.Equal(got, []string{"hello", "world"}) {
		t.Fatalf("got %q", got)
	}
}

func TestTermsAreUnique(t *testing.T) {
	got := Tokenizer{}.Terms("ping PING ping")
	if !slices.Equal(got, []string{"ping"}) {
		t.Fatalf("got %q", got)
	}
}

func TestTermsKeepDigitsAndSplitOnPunctuation(t *testing.T) {
	got := Tokenizer{}.Terms("searchable-17a3 report_2026.pdf")
	if !slices.Equal(got, []string{"17a3", "2026", "pdf", "report", "searchable"}) {
		t.Fatalf("got %q", got)
	}
}

func TestTermsKeepLettersOfOtherScripts(t *testing.T) {
	got := Tokenizer{}.Terms("Grüße 東京")
	if !slices.Equal(got, []string{"grüße", "東京"}) {
		t.Fatalf("got %q", got)
	}
}

func TestTermsOfEmptyTextAreEmpty(t *testing.T) {
	if got := (Tokenizer{}).Terms("  ,. "); len(got) != 0 {
		t.Fatalf("got %q", got)
	}
}
