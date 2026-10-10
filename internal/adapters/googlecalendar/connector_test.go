package googlecalendar

import (
	"context"
	"encoding/json"
	"fmt"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
	"time"

	"github.com/d31ma/caduceus/internal/accounts"
	"github.com/d31ma/caduceus/internal/accountsync"
	"github.com/d31ma/caduceus/internal/adapters/calendarapi"
	"github.com/d31ma/caduceus/internal/calendarsync"
)

func TestConnectorSynchronizesGoogleCalendarsAndIncrementalChanges(t *testing.T) {
	now := time.Date(2026, 8, 22, 12, 0, 0, 0, time.UTC)
	requests := 0
	server := httptest.NewServer(http.HandlerFunc(func(response http.ResponseWriter, request *http.Request) {
		if request.Header.Get("Authorization") != "Bearer access-token" {
			t.Fatalf("authorization=%q", request.Header.Get("Authorization"))
		}
		response.Header().Set("Content-Type", "application/json")
		switch request.URL.Path {
		case "/calendar/v3/users/me/calendarList":
			_, _ = fmt.Fprint(response, `{"items":[{"id":"work@example.com","summary":"Work","backgroundColor":"#336699","accessRole":"owner"}]}`)
		case "/calendar/v3/calendars/work@example.com/events":
			requests++
			if request.URL.Query().Get("syncToken") == "sync-1" {
				_, _ = fmt.Fprint(response, `{"nextSyncToken":"sync-2","items":[{"id":"one","etag":"\"2\"","iCalUID":"one@example.com","summary":"Updated","status":"confirmed","start":{"dateTime":"2026-08-24T13:00:00-04:00","timeZone":"America/Toronto"},"end":{"dateTime":"2026-08-24T14:00:00-04:00","timeZone":"America/Toronto"}}]}`)
				return
			}
			_, _ = fmt.Fprint(response, `{"nextSyncToken":"sync-1","items":[{"id":"one","etag":"\"1\"","iCalUID":"one@example.com","summary":"Planning","description":"Roadmap","location":"HQ","status":"confirmed","start":{"dateTime":"2026-08-24T13:00:00-04:00","timeZone":"America/Toronto"},"end":{"dateTime":"2026-08-24T14:00:00-04:00","timeZone":"America/Toronto"},"recurrence":["RRULE:FREQ=WEEKLY;BYDAY=MO"],"organizer":{"email":"owner@example.com"},"attendees":[{"email":"alice@example.com","displayName":"Alice","responseStatus":"accepted"}],"reminders":{"overrides":[{"method":"popup","minutes":10}]}}]}`)
		default:
			http.NotFound(response, request)
		}
	}))
	defer server.Close()
	connector, err := New(Options{BaseURL: server.URL + "/calendar/v3/", HTTPClient: server.Client(), Clock: func() time.Time { return now }})
	if err != nil {
		t.Fatal(err)
	}
	account := accounts.ExternalAccount{ID: "ext_AAAAAAAAAAAAAAAAAAAA", Provider: accounts.ProviderGmail, Capabilities: accounts.Capabilities{Calendar: true}}
	first, err := connector.Sync(context.Background(), account, accounts.Credentials{AccessToken: "access-token"}, calendarsync.State{})
	if err != nil || len(first.Calendars) != 1 || len(first.Resources) != 1 || !first.Calendars[0].Writable || first.Calendars[0].SyncToken != "sync-1" {
		t.Fatalf("first=%#v err=%v", first, err)
	}
	parsed, err := calendarapi.Parse(first.Resources[0].RawICalendar)
	if err != nil || parsed.Summary != "Planning" || parsed.Start.TimeZone != "America/Toronto" || len(parsed.Attendees) != 1 || parsed.ReminderMinutes == nil || *parsed.ReminderMinutes != 10 {
		t.Fatalf("parsed=%#v err=%v", parsed, err)
	}
	second, err := connector.Sync(context.Background(), account, accounts.Credentials{AccessToken: "access-token"}, calendarsync.State{Calendars: first.Calendars, Resources: first.Resources})
	if err != nil || len(second.Resources) != 1 || second.Calendars[0].SyncToken != "sync-2" || !strings.Contains(second.Resources[0].RawICalendar, "SUMMARY:Updated") || requests != 2 {
		t.Fatalf("second=%#v requests=%d err=%v", second, requests, err)
	}
}

func TestConnectorAppliesConditionalGoogleMutations(t *testing.T) {
	var createdID string
	server := httptest.NewServer(http.HandlerFunc(func(response http.ResponseWriter, request *http.Request) {
		response.Header().Set("Content-Type", "application/json")
		if request.Header.Get("Authorization") != "Bearer access-token" {
			t.Fatalf("authorization=%q", request.Header.Get("Authorization"))
		}
		switch {
		case request.Method == http.MethodPost && request.URL.Path == "/calendar/v3/calendars/work@example.com/events":
			var event googleEvent
			if json.NewDecoder(request.Body).Decode(&event) != nil || event.ID == "" || event.Summary != "Created" || event.ICalUID != "" || event.Organizer.Email != "" {
				t.Fatalf("created event=%#v", event)
			}
			createdID = event.ID
			response.WriteHeader(http.StatusCreated)
			_, _ = fmt.Fprintf(response, `{"id":%q}`, event.ID)
		case request.Method == http.MethodPut && request.URL.Path == "/calendar/v3/calendars/work@example.com/events/one":
			if request.Header.Get("If-Match") != `"7"` {
				t.Fatalf("if-match=%q", request.Header.Get("If-Match"))
			}
			_, _ = fmt.Fprint(response, `{"id":"one"}`)
		case request.Method == http.MethodDelete && request.URL.Path == "/calendar/v3/calendars/work@example.com/events/one":
			if request.Header.Get("If-Match") != `"7"` {
				t.Fatalf("if-match=%q", request.Header.Get("If-Match"))
			}
			response.WriteHeader(http.StatusNoContent)
		default:
			http.NotFound(response, request)
		}
	}))
	defer server.Close()
	connector, _ := New(Options{BaseURL: server.URL + "/calendar/v3/", HTTPClient: server.Client()})
	account := accounts.ExternalAccount{ID: "ext_AAAAAAAAAAAAAAAAAAAA", Provider: accounts.ProviderGmail, Capabilities: accounts.Capabilities{Calendar: true}}
	scope := accountsync.Scope{AccountID: account.ID}
	calendarURL := server.URL + "/calendar/v3/calendars/work@example.com/events"
	start, _ := calendarapi.NewInstant("2026-08-24T13:00:00Z", "UTC")
	end, _ := calendarapi.NewInstant("2026-08-24T14:00:00Z", "UTC")
	raw, _ := calendarapi.Render(calendarapi.Event{UID: "created@example.com", Summary: "Created", Start: start, End: end}, time.Now())
	createCurrent := calendarsync.Resource{ID: calendarsync.RemoteResourceID(account.ID, calendarURL+"/pending.ics"), AccountID: account.ID, CalendarID: "calendar_work", Href: calendarURL + "/pending.ics", RawICalendar: raw}
	create := calendarsync.Mutation{MutationRequest: calendarsync.MutationRequest{ID: "calmut_abcdefghijklmnopqrstuvwx", IdempotencyKey: "create-google-1", Kind: calendarsync.MutationCreate, ResourceID: createCurrent.ID, Desired: createCurrent}, Scope: scope, State: calendarsync.MutationApplying}
	if outcome, err := connector.ApplyMutation(context.Background(), account, accounts.Credentials{AccessToken: "access-token"}, create, createCurrent); err != nil || outcome != calendarsync.OutcomeApplied || createdID == "" {
		t.Fatalf("create outcome=%s id=%q err=%v", outcome, createdID, err)
	}
	current := calendarsync.Resource{ID: calendarsync.RemoteResourceID(account.ID, calendarURL+"/one"), AccountID: account.ID, CalendarID: "calendar_work", Href: calendarURL + "/one", ETag: `"7"`, RawICalendar: raw}
	for _, kind := range []calendarsync.MutationKind{calendarsync.MutationUpdate, calendarsync.MutationDelete} {
		mutation := calendarsync.Mutation{MutationRequest: calendarsync.MutationRequest{ID: "calmut_abcdefghijklmnopqrstuvwx", IdempotencyKey: "google-" + string(kind), Kind: kind, ResourceID: current.ID, ExpectedETag: current.ETag, Desired: calendarsync.Resource{RawICalendar: raw}}, Scope: scope, State: calendarsync.MutationApplying}
		if kind == calendarsync.MutationDelete {
			mutation.Desired = calendarsync.Resource{}
		}
		if outcome, err := connector.ApplyMutation(context.Background(), account, accounts.Credentials{AccessToken: "access-token"}, mutation, current); err != nil || outcome != calendarsync.OutcomeApplied {
			t.Fatalf("%s outcome=%s err=%v", kind, outcome, err)
		}
	}
}

func TestConnectorProjectsSparseCancelledRecurringInstance(t *testing.T) {
	now := time.Date(2026, 8, 22, 12, 0, 0, 0, time.UTC)
	server := httptest.NewServer(http.HandlerFunc(func(response http.ResponseWriter, request *http.Request) {
		response.Header().Set("Content-Type", "application/json")
		switch request.URL.Path {
		case "/calendar/v3/users/me/calendarList":
			_, _ = fmt.Fprint(response, `{"items":[{"id":"work@example.com","summary":"Work","accessRole":"owner"}]}`)
		case "/calendar/v3/calendars/work@example.com/events":
			if request.URL.Query().Get("syncToken") == "sync-1" {
				_, _ = fmt.Fprint(response, `{"nextSyncToken":"sync-2","items":[{"id":"exception-1","status":"cancelled","recurringEventId":"series-1","originalStartTime":{"dateTime":"2026-08-31T13:00:00Z","timeZone":"UTC"}}]}`)
				return
			}
			_, _ = fmt.Fprint(response, `{"nextSyncToken":"sync-1","items":[{"id":"series-1","etag":"\"1\"","iCalUID":"series@example.com","summary":"Weekly","status":"confirmed","start":{"dateTime":"2026-08-24T13:00:00Z","timeZone":"UTC"},"end":{"dateTime":"2026-08-24T14:00:00Z","timeZone":"UTC"},"recurrence":["RRULE:FREQ=WEEKLY;BYDAY=MO"]}]}`)
		default:
			http.NotFound(response, request)
		}
	}))
	defer server.Close()
	connector, _ := New(Options{BaseURL: server.URL + "/calendar/v3/", HTTPClient: server.Client(), Clock: func() time.Time { return now }})
	account := accounts.ExternalAccount{ID: "ext_AAAAAAAAAAAAAAAAAAAA", Provider: accounts.ProviderGmail, Capabilities: accounts.Capabilities{Calendar: true}}
	first, err := connector.Sync(context.Background(), account, accounts.Credentials{AccessToken: "access-token"}, calendarsync.State{})
	if err != nil {
		t.Fatal(err)
	}
	second, err := connector.Sync(context.Background(), account, accounts.Credentials{AccessToken: "access-token"}, calendarsync.State{Calendars: first.Calendars, Resources: first.Resources})
	if err != nil || len(second.Resources) != 2 {
		t.Fatalf("snapshot=%#v err=%v", second, err)
	}
	found := false
	for _, resource := range second.Resources {
		event, parseErr := calendarapi.Parse(resource.RawICalendar)
		if parseErr == nil && event.RecurrenceID != nil {
			found = event.UID == "series@example.com" && event.Status == "cancelled" && event.RecurrenceID.Instant.Equal(time.Date(2026, 8, 31, 13, 0, 0, 0, time.UTC))
		}
	}
	if !found {
		t.Fatalf("cancelled recurrence not projected: %#v", second.Resources)
	}
}

func TestConnectorFallsBackToFullSyncWhenGoogleTokenExpires(t *testing.T) {
	requests := 0
	server := httptest.NewServer(http.HandlerFunc(func(response http.ResponseWriter, request *http.Request) {
		response.Header().Set("Content-Type", "application/json")
		if request.URL.Path == "/calendar/v3/users/me/calendarList" {
			_, _ = fmt.Fprint(response, `{"items":[{"id":"work@example.com","summary":"Work","accessRole":"reader"}]}`)
			return
		}
		requests++
		if request.URL.Query().Get("syncToken") == "expired" {
			response.WriteHeader(http.StatusGone)
			return
		}
		_, _ = fmt.Fprint(response, `{"nextSyncToken":"fresh","items":[{"id":"one","etag":"\"1\"","summary":"Recovered","status":"confirmed","start":{"date":"2026-08-24"},"end":{"date":"2026-08-25"}}]}`)
	}))
	defer server.Close()
	connector, _ := New(Options{BaseURL: server.URL + "/calendar/v3/", HTTPClient: server.Client()})
	account := accounts.ExternalAccount{ID: "ext_AAAAAAAAAAAAAAAAAAAA", Provider: accounts.ProviderGmail, Capabilities: accounts.Capabilities{Calendar: true}}
	calendarURL := server.URL + "/calendar/v3/calendars/work@example.com/events"
	state := calendarsync.State{Calendars: []calendarsync.Calendar{{StableID: stableID(calendarURL), URL: calendarURL, SyncToken: "expired"}}}
	snapshot, err := connector.Sync(context.Background(), account, accounts.Credentials{AccessToken: "access-token"}, state)
	if err != nil || requests != 2 || len(snapshot.Resources) != 1 || snapshot.Calendars[0].SyncToken != "fresh" || snapshot.Calendars[0].Writable {
		t.Fatalf("snapshot=%#v requests=%d err=%v", snapshot, requests, err)
	}
}
