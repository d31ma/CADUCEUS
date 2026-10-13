package microsoftcalendar

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

func TestConnectorSynchronizesOwnedMicrosoftCalendars(t *testing.T) {
	now := time.Date(2026, 8, 22, 12, 0, 0, 0, time.UTC)
	server := httptest.NewServer(http.HandlerFunc(func(response http.ResponseWriter, request *http.Request) {
		if request.Header.Get("Authorization") != "Bearer access-token" || request.Header.Get("Prefer") != `outlook.timezone="UTC"` {
			t.Fatalf("headers=%#v", request.Header)
		}
		response.Header().Set("Content-Type", "application/json")
		switch request.URL.Path {
		case "/v1.0/users/alice@example.com/calendars":
			_, _ = fmt.Fprint(response, `{"value":[{"id":"work-id","name":"Work","color":"lightBlue","canEdit":true}]}`)
		case "/v1.0/users/alice@example.com/calendars/work-id/events":
			_, _ = fmt.Fprint(response, `{"value":[{"id":"event-1","@odata.etag":"W/\"7\"","iCalUId":"event-1@example.com","subject":"Planning","body":{"contentType":"text","content":"Roadmap"},"location":{"displayName":"HQ"},"start":{"dateTime":"2026-08-24T13:00:00.0000000","timeZone":"UTC"},"end":{"dateTime":"2026-08-24T14:00:00.0000000","timeZone":"UTC"},"organizer":{"emailAddress":{"address":"owner@example.com","name":"Owner"}},"attendees":[{"emailAddress":{"address":"alice@example.com","name":"Alice"},"status":{"response":"accepted"}}],"recurrence":{"pattern":{"type":"weekly","interval":1,"daysOfWeek":["monday"]},"range":{"type":"numbered","startDate":"2026-08-24","numberOfOccurrences":4}},"isReminderOn":true,"reminderMinutesBeforeStart":15}]}`)
		default:
			http.NotFound(response, request)
		}
	}))
	defer server.Close()
	connector, err := New(Options{BaseURL: server.URL + "/v1.0/", HTTPClient: server.Client(), Clock: func() time.Time { return now }})
	if err != nil {
		t.Fatal(err)
	}
	account := accounts.ExternalAccount{ID: "ext_AAAAAAAAAAAAAAAAAAAA", Provider: accounts.ProviderMicrosoft, MailboxAddress: "Alice@Example.com", Capabilities: accounts.Capabilities{Calendar: true}}
	snapshot, err := connector.Sync(context.Background(), account, accounts.Credentials{AccessToken: "access-token"}, calendarsync.State{})
	if err != nil || len(snapshot.Calendars) != 1 || len(snapshot.Resources) != 1 || !snapshot.Calendars[0].Writable || snapshot.Calendars[0].Color != "#4FC3F7" {
		t.Fatalf("snapshot=%#v err=%v", snapshot, err)
	}
	parsed, err := calendarapi.Parse(snapshot.Resources[0].RawICalendar)
	if err != nil || parsed.Summary != "Planning" || parsed.Description != "Roadmap" || len(parsed.Attendees) != 1 || len(parsed.Recurrence) != 1 || !strings.Contains(parsed.Recurrence[0], "COUNT=4") || parsed.ReminderMinutes == nil || *parsed.ReminderMinutes != 15 {
		t.Fatalf("parsed=%#v err=%v", parsed, err)
	}
}

func TestConnectorAppliesConditionalMicrosoftMutations(t *testing.T) {
	var transaction string
	server := httptest.NewServer(http.HandlerFunc(func(response http.ResponseWriter, request *http.Request) {
		response.Header().Set("Content-Type", "application/json")
		switch {
		case request.Method == http.MethodPost && request.URL.Path == "/v1.0/users/alice@example.com/calendars/work-id/events":
			var event graphEvent
			if json.NewDecoder(request.Body).Decode(&event) != nil || event.Subject != "Created" || event.TransactionID == "" || event.Recurrence == nil {
				t.Fatalf("created=%#v", event)
			}
			transaction = event.TransactionID
			response.WriteHeader(http.StatusCreated)
			_, _ = fmt.Fprint(response, `{"id":"created"}`)
		case request.Method == http.MethodPatch && request.URL.Path == "/v1.0/users/alice@example.com/calendars/work-id/events/one":
			if request.Header.Get("If-Match") != `W/"7"` {
				t.Fatalf("if-match=%q", request.Header.Get("If-Match"))
			}
			_, _ = fmt.Fprint(response, `{"id":"one"}`)
		case request.Method == http.MethodDelete && request.URL.Path == "/v1.0/users/alice@example.com/calendars/work-id/events/one":
			response.WriteHeader(http.StatusNoContent)
		default:
			http.NotFound(response, request)
		}
	}))
	defer server.Close()
	connector, _ := New(Options{BaseURL: server.URL + "/v1.0/", HTTPClient: server.Client()})
	account := accounts.ExternalAccount{ID: "ext_AAAAAAAAAAAAAAAAAAAA", Provider: accounts.ProviderMicrosoft, MailboxAddress: "alice@example.com", Capabilities: accounts.Capabilities{Calendar: true}}
	scope := accountsync.Scope{AccountID: account.ID}
	calendarURL := server.URL + "/v1.0/users/alice@example.com/calendars/work-id/events"
	start, _ := calendarapi.NewInstant("2026-08-24T13:00:00Z", "UTC")
	end, _ := calendarapi.NewInstant("2026-08-24T14:00:00Z", "UTC")
	raw, _ := calendarapi.Render(calendarapi.Event{UID: "created@example.com", Summary: "Created", Start: start, End: end, Recurrence: []string{"RRULE:FREQ=WEEKLY;BYDAY=MO;COUNT=4"}}, time.Now())
	createCurrent := calendarsync.Resource{ID: calendarsync.RemoteResourceID(account.ID, calendarURL+"/pending.ics"), AccountID: account.ID, CalendarID: "calendar_work", Href: calendarURL + "/pending.ics", RawICalendar: raw}
	create := calendarsync.Mutation{MutationRequest: calendarsync.MutationRequest{ID: "calmut_abcdefghijklmnopqrstuvwx", IdempotencyKey: "create-microsoft-1", Kind: calendarsync.MutationCreate, ResourceID: createCurrent.ID, Desired: createCurrent}, Scope: scope, State: calendarsync.MutationApplying}
	if outcome, err := connector.ApplyMutation(context.Background(), account, accounts.Credentials{AccessToken: "access-token"}, create, createCurrent); err != nil || outcome != calendarsync.OutcomeApplied || transaction == "" {
		t.Fatalf("create outcome=%s transaction=%q err=%v", outcome, transaction, err)
	}
	current := calendarsync.Resource{ID: calendarsync.RemoteResourceID(account.ID, calendarURL+"/one"), AccountID: account.ID, CalendarID: "calendar_work", Href: calendarURL + "/one", ETag: `W/"7"`, RawICalendar: raw}
	for _, kind := range []calendarsync.MutationKind{calendarsync.MutationUpdate, calendarsync.MutationDelete} {
		mutation := calendarsync.Mutation{MutationRequest: calendarsync.MutationRequest{ID: "calmut_abcdefghijklmnopqrstuvwx", IdempotencyKey: "microsoft-" + string(kind), Kind: kind, ResourceID: current.ID, ExpectedETag: current.ETag, Desired: calendarsync.Resource{RawICalendar: raw}}, Scope: scope, State: calendarsync.MutationApplying}
		if kind == calendarsync.MutationDelete {
			mutation.Desired = calendarsync.Resource{}
		}
		if outcome, err := connector.ApplyMutation(context.Background(), account, accounts.Credentials{AccessToken: "access-token"}, mutation, current); err != nil || outcome != calendarsync.OutcomeApplied {
			t.Fatalf("%s outcome=%s err=%v", kind, outcome, err)
		}
	}
}

func TestConnectorProjectsAllDayMicrosoftException(t *testing.T) {
	remote := graphEvent{
		ID: "exception-1", ICalUID: "series@example.com", Subject: "Away", IsAllDay: true, SeriesMasterID: "series-1", OriginalStart: "2026-08-24T00:00:00.0000000",
		Start: graphDate{DateTime: "2026-08-25T00:00:00.0000000", TimeZone: "UTC"}, End: graphDate{DateTime: "2026-08-26T00:00:00.0000000", TimeZone: "UTC"},
	}
	event, err := toEvent(remote)
	if err != nil || event.RecurrenceID == nil || !event.RecurrenceID.AllDay || event.RecurrenceID.Date != "2026-08-24" || event.Start.Date != "2026-08-25" {
		t.Fatalf("event=%#v err=%v", event, err)
	}
}
