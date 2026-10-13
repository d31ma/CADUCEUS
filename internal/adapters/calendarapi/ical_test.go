package calendarapi

import (
	"strings"
	"testing"
	"time"
)

func TestRenderParseRoundTripTimedRecurringEvent(t *testing.T) {
	start, _ := NewInstant("2026-08-24T13:00:00-04:00", "America/Toronto")
	end, _ := NewInstant("2026-08-24T14:30:00-04:00", "America/Toronto")
	minutes := 15
	event := Event{UID: "provider-1@example.com", Summary: "Roadmap, phase 2", Description: "Line one\nLine two", Location: "Toronto; HQ", Organizer: "owner@example.com", Start: start, End: end,
		Recurrence: []string{"RRULE:FREQ=WEEKLY;INTERVAL=1;BYDAY=MO"}, Attendees: []Attendee{{Email: "alice@example.com", Name: "Alice Example", Status: "accepted"}}, ReminderMinutes: &minutes}
	raw, err := Render(event, time.Date(2026, 8, 22, 12, 0, 0, 0, time.UTC))
	if err != nil {
		t.Fatal(err)
	}
	for _, required := range []string{"DTSTART;TZID=America/Toronto:20260824T130000", "SUMMARY:Roadmap\\, phase 2", "RRULE:FREQ=WEEKLY", "PARTSTAT=ACCEPTED", "TRIGGER:-PT15M"} {
		if !strings.Contains(raw, required) {
			t.Fatalf("calendar missing %q:\n%s", required, raw)
		}
	}
	parsed, err := Parse(raw)
	if err != nil {
		t.Fatal(err)
	}
	if parsed.UID != event.UID || parsed.Summary != event.Summary || parsed.Description != event.Description || parsed.Location != event.Location ||
		DateString(parsed.Start) != DateString(event.Start) || DateString(parsed.End) != DateString(event.End) || len(parsed.Attendees) != 1 || parsed.Attendees[0].Status != "accepted" || parsed.ReminderMinutes == nil || *parsed.ReminderMinutes != 15 {
		t.Fatalf("round trip=%#v", parsed)
	}
}

func TestRenderFoldsUTF8AndParsesQuotedAttendeeParameters(t *testing.T) {
	start, _ := NewInstant("2026-08-24T13:00:00Z", "UTC")
	end, _ := NewInstant("2026-08-24T14:00:00Z", "UTC")
	event := Event{UID: "provider-1@example.com", Summary: strings.Repeat("Résumé ", 30), Start: start, End: end, Attendees: []Attendee{{Email: "alice@example.com", Name: "Last; First: Team", Status: "accepted"}}}
	raw, err := Render(event, time.Now())
	if err != nil {
		t.Fatal(err)
	}
	for _, line := range strings.Split(strings.TrimSuffix(raw, "\r\n"), "\r\n") {
		if len(line) > 75 {
			t.Fatalf("line has %d octets: %q", len(line), line)
		}
	}
	parsed, err := Parse(raw)
	if err != nil || parsed.Summary != event.Summary || len(parsed.Attendees) != 1 || parsed.Attendees[0].Name != "Last; First: Team" {
		t.Fatalf("parsed=%#v err=%v", parsed, err)
	}
}

func TestRenderParseRoundTripAllDayCancellation(t *testing.T) {
	start, _ := NewDate("2026-08-24")
	end, _ := NewDate("2026-08-26")
	recurrenceID, _ := NewDate("2026-08-24")
	raw, err := Render(Event{UID: "series-1", Summary: "Away", Start: start, End: end, RecurrenceID: &recurrenceID, Status: "cancelled"}, time.Now())
	if err != nil {
		t.Fatal(err)
	}
	parsed, err := Parse(raw)
	if err != nil || !parsed.Start.AllDay || parsed.Start.Date != "2026-08-24" || parsed.End.Date != "2026-08-26" || parsed.RecurrenceID == nil || parsed.Status != "cancelled" {
		t.Fatalf("parsed=%#v err=%v", parsed, err)
	}
}

func TestParseRejectsUnsafeOrInvalidCalendar(t *testing.T) {
	for _, raw := range []string{"", "BEGIN:VCALENDAR\r\nEND:VCALENDAR\r\n", "BEGIN:VCALENDAR\r\nBEGIN:VEVENT\r\nUID:x\r\nDTSTART:bad\r\nEND:VEVENT\r\nEND:VCALENDAR\r\n"} {
		if _, err := Parse(raw); err == nil {
			t.Fatalf("accepted %q", raw)
		}
	}
}

func TestRenderRejectsInjectedRecurrenceAndInvalidStatus(t *testing.T) {
	start, _ := NewInstant("2026-08-24T13:00:00Z", "UTC")
	end, _ := NewInstant("2026-08-24T14:00:00Z", "UTC")
	for _, event := range []Event{
		{UID: "one", Start: start, End: end, Recurrence: []string{"RRULE:FREQ=DAILY\r\nATTENDEE:mailto:attacker@example.com"}},
		{UID: "one", Start: start, End: end, Status: "CONFIRMED\r\nX-INJECTED:true"},
	} {
		if _, err := Render(event, time.Now()); err == nil {
			t.Fatalf("rendered invalid event %#v", event)
		}
	}
}
