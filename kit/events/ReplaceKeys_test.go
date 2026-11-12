package events

import (
	"encoding/json"
	"testing"
)

func keyedEnvelope(eventType string, payload string) Envelope {
	return Envelope{EventID: "evt_1", Type: eventType, DomainId: "example.com", Producer: "messaging", Payload: json.RawMessage(payload)}
}

func TestReplaceKeyOfAnEditIsItsMessageId(t *testing.T) {
	key, err := ReplaceKeys{}.Of(keyedEnvelope(Types{}.MessageEdited(), `{"messageId":"msg_1","body":"hi"}`))
	if err != nil || key != (EventKey{Type: "message.edited", Key: "msg_1"}) {
		t.Fatalf("got %+v, %v", key, err)
	}
}

func TestReplaceKeyOfAReactionIsItsMessageId(t *testing.T) {
	key, err := ReplaceKeys{}.Of(keyedEnvelope(Types{}.ReactionChanged(), `{"messageId":"msg_2","memberId":"mbr_1"}`))
	if err != nil || key != (EventKey{Type: "reaction.changed", Key: "msg_2"}) {
		t.Fatalf("got %+v, %v", key, err)
	}
}

func TestReplaceKeyOfPresenceIsItsMemberId(t *testing.T) {
	key, err := ReplaceKeys{}.Of(keyedEnvelope(Types{}.PresenceChanged(), `{"memberId":"mbr_3","state":"online"}`))
	if err != nil || key != (EventKey{Type: "presence.changed", Key: "mbr_3"}) {
		t.Fatalf("got %+v, %v", key, err)
	}
}

func TestACreatedMessageHasNoReplaceKey(t *testing.T) {
	key, err := ReplaceKeys{}.Of(keyedEnvelope(Types{}.MessageCreated(), `{"messageId":"msg_1"}`))
	if err != nil || key != (EventKey{}) {
		t.Fatalf("got %+v, %v", key, err)
	}
}

func TestAnEditWithoutAMessageIdHasNoReplaceKey(t *testing.T) {
	key, err := ReplaceKeys{}.Of(keyedEnvelope(Types{}.MessageEdited(), `{"body":"hi"}`))
	if err != nil || key != (EventKey{}) {
		t.Fatalf("got %+v, %v", key, err)
	}
}

func TestAKeyedEventWithABrokenPayloadFails(t *testing.T) {
	_, err := ReplaceKeys{}.Of(keyedEnvelope(Types{}.MessageEdited(), `[1,2`))
	if err == nil {
		t.Fatal("expected an error")
	}
}

func TestADeleteRemovesTheEditsOfItsMessage(t *testing.T) {
	key, err := ReplaceKeys{}.Removes(keyedEnvelope(Types{}.MessageDeleted(), `{"messageId":"msg_4"}`))
	if err != nil || key != (EventKey{Type: "message.edited", Key: "msg_4"}) {
		t.Fatalf("got %+v, %v", key, err)
	}
}

func TestAnEditRemovesNothing(t *testing.T) {
	key, err := ReplaceKeys{}.Removes(keyedEnvelope(Types{}.MessageEdited(), `{"messageId":"msg_4"}`))
	if err != nil || key != (EventKey{}) {
		t.Fatalf("got %+v, %v", key, err)
	}
}
