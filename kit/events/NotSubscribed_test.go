package events

import "testing"

func TestNotSubscribedMessage(t *testing.T) {
	message := NotSubscribed{Service: "notify", Type: "email.received"}.Error()
	if message != "the service notify does not consume events of type email.received" {
		t.Fatalf("unexpected message %q", message)
	}
}

func TestNotSubscribedNumber(t *testing.T) {
	if (NotSubscribed{}).Number() != 13 {
		t.Fatalf("got %d", NotSubscribed{}.Number())
	}
}
