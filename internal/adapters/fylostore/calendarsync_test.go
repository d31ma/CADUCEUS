package fylostore

import (
	"context"
	"errors"
	"testing"
	"time"

	"github.com/d31ma/caduceus/internal/accounts"
	"github.com/d31ma/caduceus/internal/accountsync"
	"github.com/d31ma/caduceus/internal/calendarsync"
)

func TestCalendarSnapshotGenerationIsOwnerIsolatedAndReplacesStaleResources(t *testing.T) {
	machine := newMemoryCaller()
	store, _ := NewAccountSync(machine, true, t.TempDir())
	scope := syncScope()
	now := time.Now().UTC()
	lease, err := store.AcquireLease(context.Background(), scope, "calendar-worker", now, time.Minute)
	if err != nil {
		t.Fatal(err)
	}
	first := calendarSnapshot(scope.AccountID, now, "caldav_one", "one")
	if err := store.CommitCalendarSnapshot(context.Background(), lease, first); err != nil {
		t.Fatal(err)
	}
	listed, err := store.ListProviderCalendarResources(context.Background(), scope.Owner)
	if err != nil || len(listed) != 1 || listed[0].ID != "caldav_one" || listed[0].CalendarName != "Work" || listed[0].RawICalendar == "" {
		t.Fatalf("first list=%#v err=%v", listed, err)
	}
	second := calendarSnapshot(scope.AccountID, now.Add(time.Second), "caldav_two", "two")
	if err := store.CommitCalendarSnapshot(context.Background(), lease, second); err != nil {
		t.Fatal(err)
	}
	listed, err = store.ListProviderCalendarResources(context.Background(), scope.Owner)
	if err != nil || len(listed) != 1 || listed[0].ID != "caldav_two" {
		t.Fatalf("replacement list=%#v err=%v", listed, err)
	}
	if countCollection(machine, ProviderCalendarState) != 1 || countCollection(machine, ProviderCalendars) != 1 || countCollection(machine, ProviderCalendarResources) != 1 {
		t.Fatalf("state=%d calendars=%d resources=%d", countCollection(machine, ProviderCalendarState), countCollection(machine, ProviderCalendars), countCollection(machine, ProviderCalendarResources))
	}
	foreign := scope.Owner
	foreign.PrincipalID = "mallory"
	foreign.Address = "mallory@example.com"
	if resources, err := store.ListProviderCalendarResources(context.Background(), foreign); err != nil || len(resources) != 0 {
		t.Fatalf("foreign resources=%#v err=%v", resources, err)
	}
	state, err := store.CalendarState(context.Background(), scope)
	if err != nil || len(state.Calendars) != 1 || len(state.Resources) != 1 || state.LastSyncedAt == nil {
		t.Fatalf("state=%#v err=%v", state, err)
	}
	calendars, err := store.ListProviderCalendars(context.Background(), scope.Owner)
	if err != nil || len(calendars) != 1 || calendars[0].AccountID != scope.AccountID || calendars[0].StableID != "calendar_default" || calendars[0].URL == "" {
		t.Fatalf("calendars=%#v err=%v", calendars, err)
	}
}

func TestCalendarMutationJournalIsIdempotentOwnerScopedAndLeaseGuarded(t *testing.T) {
	machine := newMemoryCaller()
	store, _ := NewAccountSync(machine, true, t.TempDir())
	scope := syncScope()
	now := time.Now().UTC()
	lease, _ := store.AcquireLease(context.Background(), scope, "calendar-worker", now, time.Minute)
	if err := store.CommitCalendarSnapshot(context.Background(), lease, calendarSnapshot(scope.AccountID, now, "caldav_one", "one")); err != nil {
		t.Fatal(err)
	}
	raw := "BEGIN:VCALENDAR\r\nBEGIN:VEVENT\r\nUID:one\r\nSUMMARY:Updated\r\nEND:VEVENT\r\nEND:VCALENDAR\r\n"
	mutation := calendarsync.Mutation{MutationRequest: calendarsync.MutationRequest{ID: "calmut_abcdefghijklmnopqrstuvwx", IdempotencyKey: "calendar-edit-1", Kind: calendarsync.MutationUpdate, ResourceID: "caldav_one", ExpectedETag: `"1"`, Desired: calendarsync.Resource{RawICalendar: raw}}, State: calendarsync.MutationQueued, CreatedAt: now, UpdatedAt: now}
	queued, err := store.EnqueueCalendarMutation(context.Background(), scope, mutation)
	if err != nil {
		t.Fatal(err)
	}
	replayed, err := store.EnqueueCalendarMutation(context.Background(), scope, mutation)
	if err != nil || replayed.ID != queued.ID || countCollection(machine, ProviderCalendarMutations) != 1 {
		t.Fatalf("replay=%#v err=%v count=%d", replayed, err, countCollection(machine, ProviderCalendarMutations))
	}
	retry := mutation
	retry.ID = "calmut_zyxwvutsrqponmlkjihgfedc"
	replayed, err = store.EnqueueCalendarMutation(context.Background(), scope, retry)
	if err != nil || replayed.ID != queued.ID || countCollection(machine, ProviderCalendarMutations) != 1 {
		t.Fatalf("semantic replay=%#v err=%v count=%d", replayed, err, countCollection(machine, ProviderCalendarMutations))
	}
	pending, err := store.PendingCalendarMutations(context.Background(), lease, 10)
	if err != nil || len(pending) != 1 || pending[0].Scope != scope {
		t.Fatalf("pending=%#v err=%v", pending, err)
	}
	pending[0].State = calendarsync.MutationApplying
	if err := store.UpdateCalendarMutation(context.Background(), lease, pending[0]); err != nil {
		t.Fatal(err)
	}
	pending[0].State = calendarsync.MutationApplied
	if err := store.UpdateCalendarMutation(context.Background(), lease, pending[0]); err != nil {
		t.Fatal(err)
	}
	foreign := scope
	foreign.Owner.PrincipalID = "mallory"
	foreign.Owner.Address = "mallory@example.com"
	if _, err := store.ProviderCalendarResource(context.Background(), foreign, "caldav_one"); !errors.Is(err, accounts.ErrNotFound) {
		t.Fatalf("foreign resource error=%v", err)
	}
	if _, err := store.CalendarMutation(context.Background(), foreign, mutation.ID); !errors.Is(err, calendarsync.ErrResourceNotFound) {
		t.Fatalf("foreign mutation error=%v", err)
	}
	if _, err := store.PendingCalendarMutations(context.Background(), accountsync.Lease{Scope: foreign}, 10); err == nil {
		t.Fatal("foreign lease unexpectedly read pending mutations")
	}
}

func calendarSnapshot(accountID accounts.AccountID, now time.Time, id, uid string) calendarsync.Snapshot {
	return calendarsync.Snapshot{PrincipalURL: "https://dav.example/principals/alice/", HomeURL: "https://dav.example/calendars/alice/", SyncedAt: now,
		Calendars: []calendarsync.Calendar{{StableID: "calendar_default", URL: "https://dav.example/calendars/alice/work/", Name: "Work", Color: "#336699", SyncToken: "sync-1"}},
		Resources: []calendarsync.Resource{{ID: id, AccountID: accountID, CalendarID: "calendar_default", Href: "https://dav.example/calendars/alice/work/" + uid + ".ics", ETag: `"1"`, RawICalendar: "BEGIN:VCALENDAR\r\nBEGIN:VEVENT\r\nUID:" + uid + "\r\nSUMMARY:Event\r\nEND:VEVENT\r\nEND:VCALENDAR\r\n", RemoteUpdatedAt: now}}}
}
