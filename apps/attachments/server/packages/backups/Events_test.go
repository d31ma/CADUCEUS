package backups

import (
	"encoding/json"
	"errors"
	"testing"

	"mesail.dev/packages/attachments"
	"mesail.dev/packages/collections"
	"mesail.dev/packages/events"
)

func memberEvent(t *testing.T, eventId string, domainId string, memberId string, state string) events.Envelope {
	payload, _ := json.Marshal(events.MemberUpserted{MemberId: memberId, DisplayName: "Ana", PrimaryAddress: memberId + "@" + domainId, State: state})
	return events.Envelope{EventID: eventId, Type: events.Types{}.MemberUpserted(), DomainId: domainId, Producer: "directory", OccurredAt: "2026-11-14T12:00:00Z", Payload: payload}
}

func eventDesk(t *testing.T, fixture *fixture) EventDesk {
	seen := SeenEvents{Documents: fixture.documents, Scroll: fixture.documents.Scroll, Known: collections.KnownCollections{Folder: t.TempDir()}}
	return EventDesk{Gate: tokenGate{}, Seen: seen, Removal: Removal{Desk: fixture.desk}}
}

func TestRemovalDeletesTheBackupOfARemovedMember(t *testing.T) {
	fixture := newFixture(t)
	fixture.backedUp(t, ana, 0, "cipher")
	fixture.backedUp(t, "Bearer bo@example.com", 0, "bo")
	err := Removal{Desk: fixture.desk}.Handle(memberEvent(t, "e1", "example.com", "ana", "removed"))
	_, anaErr := fixture.desk.Mine(ana)
	_, boErr := fixture.desk.Mine("Bearer bo@example.com")
	if err != nil || !errors.Is(anaErr, Missing{}) || boErr != nil || len(fixture.store.blobs) != 1 {
		t.Fatalf("removal: %v, ana %v, bo %v, %d blobs", err, anaErr, boErr, len(fixture.store.blobs))
	}
}

func TestRemovalKeepsTheBackupOfAnActiveOrDisabledMember(t *testing.T) {
	fixture := newFixture(t)
	fixture.backedUp(t, ana, 0, "cipher")
	for _, state := range []string{"active", "disabled"} {
		if err := (Removal{Desk: fixture.desk}).Handle(memberEvent(t, "e-"+state, "example.com", "ana", state)); err != nil {
			t.Fatal(err)
		}
	}
	if _, err := fixture.desk.Mine(ana); err != nil {
		t.Fatalf("backup gone: %v", err)
	}
}

func TestRemovalOfAPersonWithoutABackupDoesNothing(t *testing.T) {
	if err := (Removal{Desk: newFixture(t).desk}).Handle(memberEvent(t, "e1", "example.com", "ana", "removed")); err != nil {
		t.Fatal(err)
	}
}

func TestRemovalRefusesABadPayload(t *testing.T) {
	envelope := memberEvent(t, "e1", "example.com", "ana", "removed")
	envelope.Payload = json.RawMessage(`{"memberId":"ana","extra":1}`)
	if err := (Removal{Desk: newFixture(t).desk}).Handle(envelope); err == nil {
		t.Fatal("bad payload accepted")
	}
}

func TestEventDeskRefusesACallerWithoutAToken(t *testing.T) {
	fixture := newFixture(t)
	_, err := eventDesk(t, fixture).Receive("Bearer nobody", memberEvent(t, "e1", "example.com", "ana", "removed"))
	if !errors.Is(err, attachments.Unauthenticated{}) {
		t.Fatalf("want unauthenticated, got %v", err)
	}
}

func TestEventDeskRefusesAnotherDomain(t *testing.T) {
	fixture := newFixture(t)
	_, err := eventDesk(t, fixture).Receive("Bearer directory@example.org", memberEvent(t, "e1", "example.com", "ana", "removed"))
	if !errors.Is(err, OtherDomain{}) {
		t.Fatalf("want other domain, got %v", err)
	}
}

func TestEventDeskRefusesAMember(t *testing.T) {
	fixture := newFixture(t)
	_, err := eventDesk(t, fixture).Receive(ana, memberEvent(t, "e1", "example.com", "ana", "removed"))
	if !errors.As(err, &events.NotService{}) {
		t.Fatalf("want not service, got %v", err)
	}
}

func TestEventDeskRemovesTheBackupOnceForAServiceEvent(t *testing.T) {
	fixture := newFixture(t)
	fixture.backedUp(t, ana, 0, "cipher")
	desk := eventDesk(t, fixture)
	first, firstErr := desk.Receive("Bearer directory@*", memberEvent(t, "e1", "example.com", "ana", "removed"))
	again, againErr := desk.Receive("Bearer directory@example.com", memberEvent(t, "e1", "example.com", "ana", "removed"))
	_, mineErr := fixture.desk.Mine(ana)
	if firstErr != nil || againErr != nil || first.Status != "applied" || again.Status != "duplicate" {
		t.Fatalf("receipts %+v %v, %+v %v", first, firstErr, again, againErr)
	}
	if !errors.Is(mineErr, Missing{}) {
		t.Fatalf("backup kept: %v", mineErr)
	}
}

func TestSeenEventsRememberAnEvent(t *testing.T) {
	fixture := newFixture(t)
	seen := SeenEvents{Documents: fixture.documents, Scroll: fixture.documents.Scroll, Known: collections.KnownCollections{Folder: t.TempDir()}}
	before, _ := seen.Seen("example.com", "e1")
	if err := seen.Remember("example.com", "e1"); err != nil {
		t.Fatal(err)
	}
	after, _ := seen.Seen("example.com", "e1")
	other, _ := seen.Seen("example.com", "e2")
	if before || !after || other {
		t.Fatalf("seen before %v after %v other %v", before, after, other)
	}
}

func TestSeenEventsReportAStorageFailure(t *testing.T) {
	fixture := newFixture(t)
	seen := SeenEvents{Documents: fixture.documents, Scroll: fixture.documents.Scroll, Known: collections.KnownCollections{Folder: t.TempDir()}}
	seen.Remember("example.com", "e1")
	fixture.documents.failOn = "scroll"
	if _, err := seen.Seen("example.com", "e1"); err == nil {
		t.Fatal("failure hidden")
	}
}
