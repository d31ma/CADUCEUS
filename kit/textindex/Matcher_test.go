package textindex

import "testing"

func TestMatchesWhenEveryQueryTermIsFound(t *testing.T) {
	if !(Matcher{}).Matches([]string{"budget", "report", "q3"}, []string{"report", "q3"}) {
		t.Fatal("expected a match")
	}
}

func TestMatchesAQueryTermThatIsAPrefix(t *testing.T) {
	if !(Matcher{}).Matches([]string{"searchable"}, []string{"search"}) {
		t.Fatal("expected a prefix match")
	}
}

func TestDoesNotMatchWhenOneQueryTermIsMissing(t *testing.T) {
	if (Matcher{}).Matches([]string{"budget", "report"}, []string{"report", "q4"}) {
		t.Fatal("expected no match")
	}
}

func TestDoesNotMatchATermInsideAnotherWord(t *testing.T) {
	if (Matcher{}).Matches([]string{"searchable"}, []string{"arch"}) {
		t.Fatal("expected no match")
	}
}

func TestEmptyQueryMatchesEveryDocument(t *testing.T) {
	if !(Matcher{}).Matches([]string{"anything"}, nil) {
		t.Fatal("expected a match")
	}
}
