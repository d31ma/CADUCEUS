package inbound

import (
	"errors"
	"testing"

	"mesail.dev/packages/core"
	"mesail.dev/packages/events"
)

func TestAnEventTypeMessagingDoesNotHandleIsAnError(t *testing.T) {
	scene := NewScene(t)
	err := scene.Send(events.Types{}.MessageCreated(), "example.com", events.MessageCreated{})
	if !errors.As(err, new(UnhandledEvent)) || err.Error() == "" {
		t.Fatalf("want UnhandledEvent, got %v", err)
	}
}

func TestAPayloadWithAnUnknownFieldIsRefused(t *testing.T) {
	scene := NewScene(t)
	err := scene.Send(events.Types{}.MemberUpserted(), "example.com", map[string]string{"memberId": "ana", "colour": "red"})
	if err == nil || errors.As(err, new(*core.Unavailable)) {
		t.Fatalf("want a payload error, got %v", err)
	}
}

func TestMemberUpsertedSavesTheMemberOnce(t *testing.T) {
	scene := NewScene(t)
	scene.Must(events.Types{}.MemberUpserted(), "example.com", events.MemberUpserted{MemberId: "ana", DisplayName: "Ana", PrimaryAddress: "Ana.Silva@Example.com", State: "active"})
	scene.Must(events.Types{}.MemberUpserted(), "example.com", events.MemberUpserted{MemberId: "ana", DisplayName: "Ana S.", PrimaryAddress: "Ana.Silva@Example.com", State: "disabled"})
	members := scene.Members()
	if len(members) != 1 {
		t.Fatalf("want one member, got %+v", members)
	}
	want := core.Member{Id: members[0].Id, DomainId: "example.com", MemberId: "ana", DisplayName: "Ana S.", PrimaryAddress: "Ana.Silva@Example.com", LocalPart: "ana.silva", State: "disabled"}
	if members[0] != want {
		t.Fatalf("got %+v", members[0])
	}
}

func TestMemberUpsertedReportsAStoreFailure(t *testing.T) {
	scene := NewScene(t)
	scene.FailAt(1)
	err := scene.Send(events.Types{}.MemberUpserted(), "example.com", events.MemberUpserted{MemberId: "ana"})
	if !errors.As(err, new(*core.Unavailable)) || err.Error() == "" || errors.Unwrap(err) == nil {
		t.Fatalf("want Unavailable, got %v", err)
	}
}
