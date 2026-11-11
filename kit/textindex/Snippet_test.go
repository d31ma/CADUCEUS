package textindex

import "testing"

func TestShortTextIsReturnedWhole(t *testing.T) {
	if got := (Snippet{Width: 40}).Around("Lunch at noon?", []string{"noon"}); got != "Lunch at noon?" {
		t.Fatalf("got %q", got)
	}
}

func TestLongTextIsCutAroundTheFirstHit(t *testing.T) {
	text := "aaaa bbbb cccc dddd eeee target ffff gggg hhhh iiii"
	if got := (Snippet{Width: 16}).Around(text, []string{"target"}); got != "...eee target ffff..." {
		t.Fatalf("got %q", got)
	}
}

func TestHitIsFoundWithoutCase(t *testing.T) {
	text := "aaaa bbbb cccc dddd eeee TARGET ffff gggg hhhh iiii"
	if got := (Snippet{Width: 16}).Around(text, []string{"target"}); got != "...eee TARGET ffff..." {
		t.Fatalf("got %q", got)
	}
}

func TestTextWithoutAHitStartsAtTheBeginning(t *testing.T) {
	if got := (Snippet{Width: 9}).Around("aaaa bbbb cccc", []string{"zzz"}); got != "aaaa bbbb..." {
		t.Fatalf("got %q", got)
	}
}

func TestHitNearTheEndKeepsTheEnd(t *testing.T) {
	if got := (Snippet{Width: 9}).Around("aaaa bbbb cccc", []string{"cccc"}); got != "...bbbb cccc" {
		t.Fatalf("got %q", got)
	}
}

func TestSnippetCountsCharactersNotBytes(t *testing.T) {
	if got := (Snippet{Width: 4}).Around("東京東京東京 hit", []string{"hit"}); got != "...hit" {
		t.Fatalf("got %q", got)
	}
}
