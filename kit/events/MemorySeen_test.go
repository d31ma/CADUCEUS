package events

import "testing"

func TestMemorySeenRemembersByDomainAndEvent(t *testing.T) {
	seen := NewMemorySeen()
	err := seen.Remember("example.com", "evt_1")
	if err != nil {
		t.Fatalf("expected no error, got %v", err)
	}
	same, _ := seen.Seen("example.com", "evt_1")
	other, _ := seen.Seen("example.org", "evt_1")
	if !same || other || seen.Count() != 1 {
		t.Fatalf("unexpected state: same %v, other %v, count %d", same, other, seen.Count())
	}
}
