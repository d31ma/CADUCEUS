package events

import (
	"encoding/json"
	"testing"
)

func TestEnvelopeUsesLowerCamelJSON(t *testing.T) {
	raw, err := json.Marshal(Envelope{EventID: "e1", Type: Types{}.MessageCreated(), Payload: json.RawMessage(`{}`)})
	if err != nil {
		t.Fatal(err)
	}
	want := `{"eventId":"e1","type":"message.created","domainId":"","producer":"","occurredAt":"","payload":{}}`
	if string(raw) != want {
		t.Fatalf("got %s", raw)
	}
}

func TestEveryEventTypeIsDistinct(t *testing.T) {
	types := Types{}
	names := []string{
		types.MemberUpserted(), types.ChannelCreated(), types.ChannelMemberAdded(),
		types.ChannelMemberRemoved(), types.ChannelArchived(), types.MessageCreated(),
		types.MessageEdited(), types.MessageDeleted(), types.ReactionChanged(),
		types.ReadPositionMoved(), types.EmailSendRequested(), types.EmailDeliveryChanged(),
		types.EmailReceived(), types.PresenceChanged(),
	}
	seen := map[string]bool{}
	for _, name := range names {
		seen[name] = true
	}
	if len(seen) != len(names) {
		t.Fatalf("duplicate event type in %v", names)
	}
}
