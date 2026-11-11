package outbox

import (
	"encoding/json"
	"testing"
)

func TestCodecWritesAndReadsTheOutboxState(t *testing.T) {
	state := OutboxState{
		Pending: []PendingEvent{{Envelope: channelCreated("evt_1"), Waiting: []string{"messaging"}, Attempts: 2}},
		Dead:    []DeadLetter{{Envelope: channelCreated("evt_2"), Subscriber: "realtime", Attempts: 3, LastError: "gone"}},
	}

	fields, err := Codec{}.Write(state)
	if err != nil || fields.OutboxPending != 1 || fields.OutboxDead != 1 {
		t.Fatalf("fields %+v, error %v", fields, err)
	}
	read, err := Codec{}.Read(fields)
	if err != nil || read.Pending[0].Attempts != 2 || read.Dead[0].LastError != "gone" {
		t.Fatalf("state %+v, error %v", read, err)
	}
}

func TestCodecWritesEmptyListsForAnEmptyState(t *testing.T) {
	fields, err := Codec{}.Write(OutboxState{})
	if err != nil || fields.PendingEvents == nil || fields.DeadEvents == nil {
		t.Fatalf("fields %+v, error %v", fields, err)
	}
}

func TestCodecRefusesUnknownFields(t *testing.T) {
	_, err := Codec{}.DecodePending([]string{`{"envelope":{},"surprise":1}`})
	if err == nil {
		t.Fatal("an unknown field should be refused")
	}
}

func TestCodecReportsABrokenDeadLetter(t *testing.T) {
	_, err := Codec{}.Read(Fields{DeadEvents: []string{"not json"}})
	if err == nil {
		t.Fatal("a broken dead letter should be reported")
	}
}

func TestCodecReportsAPayloadItCannotEncode(t *testing.T) {
	envelope := channelCreated("evt_1")
	envelope.Payload = json.RawMessage("{")

	_, pendingErr := Codec{}.Write(OutboxState{Pending: []PendingEvent{{Envelope: envelope}}})
	_, deadErr := Codec{}.Write(OutboxState{Dead: []DeadLetter{{Envelope: envelope}}})
	if pendingErr == nil || deadErr == nil {
		t.Fatalf("errors %v and %v", pendingErr, deadErr)
	}
}
