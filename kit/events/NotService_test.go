package events

import "testing"

func TestNotServiceMessage(t *testing.T) {
	message := NotService{}.Error()
	if message != "only a caller with the Service role may deliver events" {
		t.Fatalf("unexpected message %q", message)
	}
}
