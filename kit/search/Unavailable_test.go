package search

import "testing"

func TestUnavailableWithoutACauseIsItsMessage(t *testing.T) {
	failure := Unavailable{Message: "the search index is not available"}
	if failure.Error() != "the search index is not available" {
		t.Fatalf("unexpected text %q", failure.Error())
	}
}
