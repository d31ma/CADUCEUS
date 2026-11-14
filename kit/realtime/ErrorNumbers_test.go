package realtime

import "testing"

type numbered interface {
	error
	Number() int
}

func TestEveryErrorCaseHasItsOwnNumber(t *testing.T) {
	cases := []struct {
		err    numbered
		number int
		text   string
	}{
		{err: Invalid{}.ConnectionIdMissing(), number: 5001, text: "invalid connectionId: a connection id is required"},
		{err: Invalid{}.UnknownKind(), number: 5002, text: "invalid kind: the kind is socket or stream"},
		{err: Invalid{}.UnknownMessageType(), number: 5003, text: "invalid type: unknown message type"},
		{err: Invalid{}.ConversationIdMissing(), number: 5004, text: "invalid conversationId: a conversation id is required"},
		{err: Invalid{}.WrongMemberCount(), number: 5005, text: "invalid memberIds: must name 1 to 100 members"},
		{err: Invalid{}.NowNotATime(), number: 5006, text: "invalid now: must be an RFC 3339 time"},
		{err: Invalid{}.UnknownState(), number: 5007, text: "invalid state: must be active, idle or dnd"},
		{err: Invalid{}.EndNotInFuture(), number: 5008, text: "invalid doNotDisturbUntil: must be an RFC 3339 time in the future"},
		{err: Unauthenticated{}.TokenRefused(), number: 5011, text: "the access token is missing, expired or not valid"},
		{err: Unauthenticated{}.MemberTokenRefused(), number: 5012, text: "the access token is missing, expired or not valid"},
		{err: Unauthenticated{}.TicketRefused(), number: 5013, text: "the access token is missing, expired or not valid"},
		{err: Unauthenticated{}.NotSignedIn(), number: 5014, text: "the access token is missing, expired or not valid"},
		{err: Unauthenticated{}.SignInEnded(), number: 5015, text: "the access token is missing, expired or not valid"},
		{err: Unauthenticated{}.OtherMember(), number: 5016, text: "the access token is missing, expired or not valid"},
		{err: Forbidden{}.NotAService(), number: 5017, text: "this caller may not use this route"},
		{err: Missing{}.EventDomain(), number: 5019, text: "not found: domain"},
		{err: Missing{}.SweepDomain(), number: 5020, text: "not found: domain"},
		{err: Missing{}.Presence(), number: 5021, text: "not found: presence"},
		{err: Missing{}.Conversation(), number: 5022, text: "not found: conversation"},
		{err: Missing{}.Connection(), number: 5023, text: "not found: connection"},
	}
	for _, each := range cases {
		if each.err.Number() != each.number || each.err.Error() != each.text {
			t.Fatalf("got %d %q, want %d %q", each.err.Number(), each.err.Error(), each.number, each.text)
		}
	}
}

func TestInvalidCasesNameTheirField(t *testing.T) {
	fields := map[string]Invalid{
		"connectionId":      Invalid{}.ConnectionIdMissing(),
		"kind":              Invalid{}.UnknownKind(),
		"type":              Invalid{}.UnknownMessageType(),
		"conversationId":    Invalid{}.ConversationIdMissing(),
		"memberIds":         Invalid{}.WrongMemberCount(),
		"now":               Invalid{}.NowNotATime(),
		"state":             Invalid{}.UnknownState(),
		"doNotDisturbUntil": Invalid{}.EndNotInFuture(),
	}
	for field, failure := range fields {
		if failure.Field != field {
			t.Fatalf("got %q, want %q", failure.Field, field)
		}
	}
}
