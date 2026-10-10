package calendarsync

import (
	"context"
	"testing"
	"time"

	"github.com/d31ma/caduceus/internal/accounts"
	"github.com/d31ma/caduceus/internal/accountsync"
)

type jobStore struct {
	state     State
	committed Snapshot
	mutations []Mutation
	resource  Resource
	updated   []Mutation
}

func (store *jobStore) CalendarState(context.Context, accountsync.Scope) (State, error) {
	return store.state, nil
}
func (store *jobStore) CommitCalendarSnapshot(_ context.Context, _ accountsync.Lease, snapshot Snapshot) error {
	store.committed = snapshot
	return nil
}

func (store *jobStore) ProviderCalendarResource(context.Context, accountsync.Scope, string) (Resource, error) {
	return store.resource, nil
}
func (store *jobStore) EnqueueCalendarMutation(_ context.Context, _ accountsync.Scope, mutation Mutation) (Mutation, error) {
	return mutation, nil
}
func (store *jobStore) PendingCalendarMutations(context.Context, accountsync.Lease, int) ([]Mutation, error) {
	return store.mutations, nil
}
func (store *jobStore) UpdateCalendarMutation(_ context.Context, _ accountsync.Lease, mutation Mutation) error {
	store.updated = append(store.updated, mutation)
	return nil
}
func (store *jobStore) CalendarMutation(context.Context, accountsync.Scope, string) (Mutation, error) {
	return Mutation{}, nil
}

type jobConnector struct {
	called  bool
	applied int
}

func (connector *jobConnector) Sync(_ context.Context, account accounts.ExternalAccount, _ accounts.Credentials, _ State) (Snapshot, error) {
	connector.called = true
	now := time.Now().UTC()
	return Snapshot{PrincipalURL: "https://dav.example/p", HomeURL: "https://dav.example/c", SyncedAt: now,
		Calendars: []Calendar{{StableID: "calendar_abcdefghijklmnopqrstuvwx", URL: "https://dav.example/c/work/", Name: "Work"}},
		Resources: []Resource{{ID: "caldav_abcdefghijklmnopqrstuvwx", AccountID: account.ID, CalendarID: "calendar_abcdefghijklmnopqrstuvwx", Href: "https://dav.example/c/work/1.ics", RawICalendar: "BEGIN:VCALENDAR\r\nBEGIN:VEVENT\r\nUID:1\r\nEND:VEVENT\r\nEND:VCALENDAR\r\n", RemoteUpdatedAt: now}}}, nil
}

func (connector *jobConnector) ApplyMutation(context.Context, accounts.ExternalAccount, accounts.Credentials, Mutation, Resource) (MutationOutcome, error) {
	connector.applied++
	return OutcomeApplied, nil
}

func TestJobSkipsAccountsWithoutCalendarTransportAndCommitsOwnedSnapshot(t *testing.T) {
	store, connector := &jobStore{}, &jobConnector{}
	job := NewJob(store, connector)
	account := accounts.ExternalAccount{ID: "ext_AAAAAAAAAAAAAAAAAAAA"}
	lease := accountsync.Lease{Scope: accountsync.Scope{AccountID: account.ID}}
	if err := job.RunWithLease(context.Background(), lease, account, accounts.Credentials{}); err != nil || connector.called {
		t.Fatalf("CalDAV-disabled run err=%v called=%v", err, connector.called)
	}
	account.Connection.CalDAV = &accounts.CalDAVEndpoint{URL: "https://dav.example/"}
	if err := job.RunWithLease(context.Background(), lease, account, accounts.Credentials{}); err != nil {
		t.Fatalf("CalDAV run: %v", err)
	}
	if !connector.called || len(store.committed.Resources) != 1 {
		t.Fatalf("connector called=%v snapshot=%#v", connector.called, store.committed)
	}
}

func TestJobAppliesQueuedMutationBeforeRefreshingSnapshot(t *testing.T) {
	account := accounts.ExternalAccount{ID: "ext_AAAAAAAAAAAAAAAAAAAA", Connection: accounts.ConnectionSettings{CalDAV: &accounts.CalDAVEndpoint{URL: "https://dav.example/"}}}
	scope := accountsync.Scope{AccountID: account.ID}
	now := time.Now().UTC()
	resource := Resource{ID: "caldav_abcdefghijklmnopqrstuvwx", AccountID: account.ID, CalendarID: "calendar_abcdefghijklmnopqrstuvwx", Href: "https://dav.example/c/work/1.ics", ETag: `"1"`, RawICalendar: "BEGIN:VCALENDAR\r\nBEGIN:VEVENT\r\nUID:1\r\nEND:VEVENT\r\nEND:VCALENDAR\r\n", RemoteUpdatedAt: now}
	mutation := Mutation{MutationRequest: MutationRequest{ID: "calmut_abcdefghijklmnopqrstuvwx", IdempotencyKey: "update-1", Kind: MutationUpdate, ResourceID: resource.ID, ExpectedETag: resource.ETag, Desired: Resource{RawICalendar: resource.RawICalendar}}, Scope: scope, State: MutationQueued, CreatedAt: now, UpdatedAt: now}
	store := &jobStore{resource: resource, mutations: []Mutation{mutation}}
	connector := &jobConnector{}
	if err := NewJob(store, connector).RunWithLease(context.Background(), accountsync.Lease{Scope: scope}, account, accounts.Credentials{}); err != nil {
		t.Fatal(err)
	}
	if connector.applied != 1 || len(store.updated) != 2 || store.updated[0].State != MutationApplying || store.updated[1].State != MutationApplied {
		t.Fatalf("applied=%d updates=%#v", connector.applied, store.updated)
	}
}
