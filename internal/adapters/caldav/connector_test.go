package caldav

import (
	"context"
	"strings"
	"testing"
	"time"

	"github.com/d31ma/caduceus/internal/accounts"
	"github.com/d31ma/caduceus/internal/accountsync"
	"github.com/d31ma/caduceus/internal/calendarsync"
)

type fakeProtocolClient struct {
	discovery  Discovery
	initial    []RemoteResource
	delta      Delta
	deltaError error
	initials   int
	deltas     int
	putTarget  string
	putBody    string
	putETag    string
	putCreate  bool
	putErr     error
	deleteETag string
	fetched    RemoteResource
	fetchErr   error
	fetches    int
}

func (client *fakeProtocolClient) Put(_ context.Context, target, body, etag string, create bool, _ Auth) (string, error) {
	client.putTarget, client.putBody, client.putETag, client.putCreate = target, body, etag, create
	return `"next"`, client.putErr
}
func (client *fakeProtocolClient) Delete(_ context.Context, _ string, etag string, _ Auth) error {
	client.deleteETag = etag
	return client.putErr
}
func (client *fakeProtocolClient) Fetch(context.Context, string, Auth) (RemoteResource, error) {
	client.fetches++
	return client.fetched, client.fetchErr
}

func (client *fakeProtocolClient) Discover(context.Context, *accounts.CalDAVEndpoint, accounts.Credentials) (Discovery, error) {
	return client.discovery, nil
}
func (client *fakeProtocolClient) InitialSync(context.Context, Calendar, Auth) ([]RemoteResource, error) {
	client.initials++
	return client.initial, nil
}
func (client *fakeProtocolClient) IncrementalSync(context.Context, Calendar, string, Auth) (Delta, error) {
	client.deltas++
	return client.delta, client.deltaError
}

func TestConnectorAppliesIncrementalChangesAndDeletes(t *testing.T) {
	now := time.Date(2026, 8, 22, 12, 0, 0, 0, time.UTC)
	account := accounts.ExternalAccount{ID: "ext_AAAAAAAAAAAAAAAAAAAA", Connection: accounts.ConnectionSettings{CalDAV: &accounts.CalDAVEndpoint{URL: "https://dav.example/"}}}
	calendarURL := "https://dav.example/calendars/work/"
	calendarID := stableID("calendar", calendarURL)
	oldHref := calendarURL + "old.ics"
	keepHref := calendarURL + "keep.ics"
	client := &fakeProtocolClient{discovery: Discovery{PrincipalURL: "https://dav.example/p", HomeURL: "https://dav.example/c", Calendars: []Calendar{{URL: calendarURL, Name: "Work", SyncToken: "sync-2"}}},
		delta: Delta{SyncToken: "sync-3", Deleted: []string{oldHref}, Changed: []RemoteResource{{Href: keepHref, ETag: `"2"`, ICalendar: calendarPayload("keep")}}}}
	state := calendarsync.State{Calendars: []calendarsync.Calendar{{StableID: calendarID, URL: calendarURL, Name: "Work", SyncToken: "sync-1"}}, Resources: []calendarsync.Resource{
		{ID: calendarsync.RemoteResourceID(account.ID, oldHref), AccountID: account.ID, CalendarID: calendarID, Href: oldHref, RawICalendar: calendarPayload("old"), RemoteUpdatedAt: now.Add(-time.Hour)},
		{ID: calendarsync.RemoteResourceID(account.ID, keepHref), AccountID: account.ID, CalendarID: calendarID, Href: keepHref, RawICalendar: calendarPayload("before"), RemoteUpdatedAt: now.Add(-time.Hour)},
	}}
	snapshot, err := NewConnector(client, func() time.Time { return now }).Sync(context.Background(), account, accounts.Credentials{}, state)
	if err != nil || client.deltas != 1 || client.initials != 0 || len(snapshot.Resources) != 1 || snapshot.Resources[0].ETag != `"2"` || snapshot.Calendars[0].SyncToken != "sync-3" {
		t.Fatalf("snapshot=%#v delta=%d initial=%d err=%v", snapshot, client.deltas, client.initials, err)
	}
}

func TestConnectorFallsBackWhenSyncTokenExpires(t *testing.T) {
	now := time.Now().UTC()
	account := accounts.ExternalAccount{ID: "ext_AAAAAAAAAAAAAAAAAAAA", Connection: accounts.ConnectionSettings{CalDAV: &accounts.CalDAVEndpoint{URL: "https://dav.example/"}}}
	calendarURL := "https://dav.example/calendars/work/"
	client := &fakeProtocolClient{discovery: Discovery{PrincipalURL: "https://dav.example/p", HomeURL: "https://dav.example/c", Calendars: []Calendar{{URL: calendarURL, Name: "Work", SyncToken: "fresh"}}}, deltaError: ErrSyncTokenInvalid,
		initial: []RemoteResource{{Href: calendarURL + "one.ics", ICalendar: calendarPayload("one")}}}
	state := calendarsync.State{Calendars: []calendarsync.Calendar{{StableID: stableID("calendar", calendarURL), URL: calendarURL, Name: "Work", SyncToken: "expired"}}}
	snapshot, err := NewConnector(client, func() time.Time { return now }).Sync(context.Background(), account, accounts.Credentials{}, state)
	if err != nil || client.deltas != 1 || client.initials != 1 || len(snapshot.Resources) != 1 {
		t.Fatalf("snapshot=%#v delta=%d initial=%d err=%v", snapshot, client.deltas, client.initials, err)
	}
}

func TestConnectorCreatesConditionallyAndRecognizesCommittedCrashRetry(t *testing.T) {
	account := accounts.ExternalAccount{ID: "ext_AAAAAAAAAAAAAAAAAAAA", Connection: accounts.ConnectionSettings{CalDAV: &accounts.CalDAVEndpoint{URL: "https://dav.example/", Username: "alice"}}}
	resource := calendarsync.Resource{AccountID: account.ID, CalendarID: "calendar_abcdefghijklmnopqrstuvwx", Href: "https://dav.example/calendars/work/new.ics", RawICalendar: calendarPayload("new")}
	resource.ID = calendarsync.RemoteResourceID(account.ID, resource.Href)
	mutation := calendarsync.Mutation{MutationRequest: calendarsync.MutationRequest{ID: "calmut_abcdefghijklmnopqrstuvwx", IdempotencyKey: "create-1", Kind: calendarsync.MutationCreate, ResourceID: resource.ID, Desired: resource}, Scope: accountsync.Scope{AccountID: account.ID}, State: calendarsync.MutationQueued}
	client := &fakeProtocolClient{}
	outcome, err := NewConnector(client).ApplyMutation(context.Background(), account, accounts.Credentials{Password: "secret"}, mutation, resource)
	if err != nil || outcome != calendarsync.OutcomeApplied || !client.putCreate || client.putTarget != resource.Href || client.putETag != "" || client.putBody != resource.RawICalendar {
		t.Fatalf("outcome=%q err=%v client=%#v", outcome, err, client)
	}
	client.putErr = ErrPrecondition
	client.fetched = RemoteResource{Href: resource.Href, ETag: `"2"`, ICalendar: strings.ReplaceAll(resource.RawICalendar, "\r\n", "\n")}
	mutation.State = calendarsync.MutationApplying
	outcome, err = NewConnector(client).ApplyMutation(context.Background(), account, accounts.Credentials{Password: "secret"}, mutation, resource)
	if err != nil || outcome != calendarsync.OutcomeApplied || client.fetches != 1 {
		t.Fatalf("retry outcome=%q err=%v fetches=%d", outcome, err, client.fetches)
	}
}

func calendarPayload(uid string) string {
	return "BEGIN:VCALENDAR\r\nBEGIN:VEVENT\r\nUID:" + uid + "\r\nSUMMARY:Event\r\nEND:VEVENT\r\nEND:VCALENDAR\r\n"
}
