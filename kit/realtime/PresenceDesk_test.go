package realtime

import (
	"errors"
	"testing"
)

func presenceDesk(memory *memoryStorage) PresenceDesk {
	return PresenceDesk{Gate: presenceGate(), Presence: presenceKeeper(memory, newRecordingGateway())}
}

func TestPresenceDeskRefusesAMissingToken(t *testing.T) {
	desk := presenceDesk(newMemoryStorage())
	_, err := desk.Query("", "ana")
	if !errors.Is(err, Unauthenticated{}) || !errors.Is(desk.Set("Bearer nobody", PresenceRequest{State: "idle"}), Unauthenticated{}) {
		t.Fatalf("got %v", err)
	}
}

func TestPresenceDeskSetsTheCallersOwnPresence(t *testing.T) {
	desk := presenceDesk(newMemoryStorage())
	err := desk.Set("Bearer ana", PresenceRequest{State: "idle"})
	found, again := desk.Query("Bearer ana", " ana , nobody,,")
	if err != nil || again != nil || len(found) != 1 || found[0].MemberId != "ana" || found[0].State != "idle" {
		t.Fatalf("got %+v %v %v", found, err, again)
	}
}

func TestMemberIdListDropsBlanksAndRepeats(t *testing.T) {
	memberIds := MemberIdList{}.Parse(" bo,ana,,bo ")
	if len(memberIds) != 2 || memberIds[0] != "ana" || memberIds[1] != "bo" {
		t.Fatalf("got %v", memberIds)
	}
}
