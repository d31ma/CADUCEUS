package realtime

import (
	"errors"
	"testing"
)

func TestTypingNeedsAConversation(t *testing.T) {
	fixture := newGatewayFixture()
	err := fixture.gateway.Typing.Typing(testConnection("example.com", "ana", "c1"), "")
	if !errors.As(err, &Invalid{}) || len(fixture.pushes.pushes) != 0 {
		t.Fatalf("got %v", err)
	}
}

func TestTypingFromANonMemberIsRefusedAndNotDelivered(t *testing.T) {
	fixture := newGatewayFixture()
	fixture.authenticate("c1", "ana-token")
	fixture.authenticate("c2", "ben-token")
	err := fixture.gateway.Typing.Typing(testConnection("example.com", "ana", "c1"), "conversation-1")
	if !errors.As(err, &Missing{}) || len(fixture.pushes.eventsOf("c2")) != 1 {
		t.Fatalf("got %v %+v", err, fixture.pushes.pushes)
	}
}
