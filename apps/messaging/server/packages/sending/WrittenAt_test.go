package sending

import (
	"testing"
	"time"

	"mesail.dev/packages/core"
)

func TestASentMessageStartsAtVersionOne(t *testing.T) {
	s := newScene(t)
	outcome := s.mustSend(t, core.SendRequest{ClientMessageId: "c1", Body: "hi", Recipients: []core.RecipientChoice{to("bo@example.com", "chat", "member")}})
	if s.stored(t, outcome).Version != 1 {
		t.Fatalf("got version %d", s.stored(t, outcome).Version)
	}
}

func TestAWrittenAtBeforeTheServerTimeIsKept(t *testing.T) {
	s := newScene(t)
	written := s.world.Clock.Now().Add(-time.Hour).Format(time.RFC3339)
	outcome := s.mustSend(t, core.SendRequest{ClientMessageId: "c1", Body: "hi", WrittenAt: written, Recipients: []core.RecipientChoice{to("bo@example.com", "chat", "member")}})
	if s.stored(t, outcome).WrittenAt != written {
		t.Fatalf("got %q", s.stored(t, outcome).WrittenAt)
	}
}

func TestOnlyAWrittenAtOfTheLastThirtyDaysIsKept(t *testing.T) {
	now := time.Date(2026, time.November, 10, 12, 0, 0, 0, time.UTC)
	written := WrittenAt{}
	cases := map[string]string{
		"":                          "",
		"not a time":                "",
		"2026-11-10T13:00:00Z":      "",
		"2026-11-10T12:00:00Z":      "",
		"2026-10-11T11:59:59Z":      "",
		"2026-10-11T12:00:00Z":      "2026-10-11T12:00:00Z",
		"2026-11-10T13:00:00+02:00": "2026-11-10T11:00:00Z",
	}
	for text, want := range cases {
		if got := written.kept(text, now); got != want {
			t.Fatalf("%q: got %q, want %q", text, got, want)
		}
	}
}
