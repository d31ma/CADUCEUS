import (
	"encoding/base64"
	"fmt"
	"os"
	"sort"
	"strings"
	"unicode/utf8"
)

// CalendarImport scans a delivered message for iCalendar payloads and
// applies each VEVENT to the recipient's calendar, by the METHOD of its
// VCALENDAR: REQUEST (or none) stores or updates the event by its UID,
// CANCEL cancels it, REPLY records the replier's answer. Failures are logged
// and never block delivery.
type CalendarImport struct {
	Events *CalendarEventsHelper
}

// FromMessage applies every invitation in a message.
func (calendar CalendarImport) FromMessage(context ImportContext) ImportResult {
	result := ImportResult{}
	for _, text := range calendar.candidates(context.TextBody, context.Attachments) {
		parsed, isCalendar, err := Ics{}.Parse(text)
		if err != nil || !isCalendar || len(parsed.Events) == 0 {
			continue
		}
		method := parsed.Method
		if method == "" {
			method = "REQUEST"
		}
		// Masters first, so exceptions attach to a stable series id even when a
		// provider writes RECURRENCE-ID components before the master.
		events := append([]map[string]any{}, parsed.Events...)
		sort.SliceStable(events, func(left, right int) bool {
			return !(RecordValues{}).Truthy(events[left]["recurrenceId"]) && (RecordValues{}).Truthy(events[right]["recurrenceId"])
		})
		for _, event := range events {
			applied, err := calendar.apply(context, method, event)
			if err != nil {
				fmt.Fprintf(os.Stderr, "[calendar-import] failed to import VEVENT uid=%v err=%s\n", event["uid"], err)
				continue
			}
			if applied {
				switch method {
				case "CANCEL":
					result.Cancelled++
				case "REPLY":
					result.ReplyApplied++
				default:
					result.Imported++
				}
			}
		}
	}
	return result
}

func (calendar CalendarImport) apply(context ImportContext, method string, event map[string]any) (bool, error) {
	switch method {
	case "CANCEL":
		return calendar.cancel(context, event)
	case "REPLY":
		return calendar.reply(context, event)
	}
	if (RecordValues{}).Truthy(event["recurrenceId"]) {
		return calendar.upsertException(context, event)
	}
	return calendar.upsert(context, event)
}

func (calendar CalendarImport) cancel(context ImportContext, event map[string]any) (bool, error) {
	values := RecordValues{}
	documentId, master, err := calendar.Events.FindByIcsUid(context.Recipient, values.Text(event["uid"]))
	if err != nil || master == nil {
		return false, err
	}
	now := EpochTime{}.NowText()
	if recurrenceId := values.Text(event["recurrenceId"]); recurrenceId != "" {
		existingDocumentId, existing, err := calendar.Events.FindException(values.String(master["id"]), recurrenceId)
		if err != nil {
			return false, err
		}
		base := master
		if existing != nil {
			base = existing
		}
		cancelled := StoredRecords{}.Without(base)
		for key, value := range map[string]any{
			"id":    values.Or(existing["id"], CalendarEventRules{}.NewId()),
			"start": values.Or(values.Or(event["start"], existing["start"]), recurrenceId),
			"end":   values.Or(values.Or(event["end"], existing["end"]), recurrenceId),
			"sourceEmailId": context.SourceEmailId, "sourceEmailIds": calendar.history(base, context.SourceEmailId),
			"seriesId": master["id"], "recurrenceId": recurrenceId, "recurrence": "", "recurrenceDates": []any{}, "exceptionDates": []any{},
			"status": "cancelled", "createdAt": values.Or(existing["createdAt"], now), "updatedAt": now,
		} {
			cancelled[key] = value
		}
		return true, calendar.replace(existingDocumentId, cancelled)
	}
	if documentId == "" {
		return false, nil
	}
	cancelled := StoredRecords{}.Without(master)
	cancelled["sourceEmailId"], cancelled["sourceEmailIds"] = context.SourceEmailId, calendar.history(master, context.SourceEmailId)
	cancelled["status"], cancelled["updatedAt"] = "cancelled", now
	return true, calendar.replace(documentId, cancelled)
}

func (calendar CalendarImport) reply(context ImportContext, event map[string]any) (bool, error) {
	values := RecordValues{}
	documentId, existing, err := calendar.Events.FindByIcsUid(context.Recipient, values.Text(event["uid"]))
	if err != nil || documentId == "" || existing == nil {
		return false, err
	}
	repliers, _ := values.List(event["attendees"])
	if len(repliers) == 0 {
		return false, nil
	}
	replier, _ := values.Object(repliers[0])
	stored, _ := values.List(existing["attendees"])
	attendees := append([]any{}, stored...)
	found := false
	for index, entry := range attendees {
		attendee, _ := values.Object(entry)
		if strings.ToLower(values.String(attendee["email"])) == strings.ToLower(values.String(replier["email"])) {
			answered := StoredRecords{}.Without(attendee)
			answered["status"] = replier["status"]
			attendees[index] = answered
			found = true
			break
		}
	}
	if !found {
		attendees = append(attendees, map[string]any{"email": replier["email"], "name": replier["name"], "status": replier["status"]})
	}
	updated := StoredRecords{}.Without(existing)
	updated["attendees"] = attendees
	updated["sourceEmailId"], updated["sourceEmailIds"] = context.SourceEmailId, calendar.history(existing, context.SourceEmailId)
	updated["updatedAt"] = EpochTime{}.NowText()
	return true, calendar.replace(documentId, updated)
}

func (calendar CalendarImport) upsert(context ImportContext, event map[string]any) (bool, error) {
	values := RecordValues{}
	documentId, existing, err := calendar.Events.FindByIcsUid(context.Recipient, values.Text(event["uid"]))
	if err != nil {
		return false, err
	}
	now := EpochTime{}.NowText()
	next := map[string]any{
		"id": values.Or(existing["id"], CalendarEventRules{}.NewId()), "userEmail": context.Recipient,
		"title": event["summary"], "start": event["start"], "end": event["end"], "allDay": values.Truthy(event["allDay"]),
		"location": values.Or(event["location"], ""), "description": values.Or(event["description"], ""),
		"attendees": values.Or(event["attendees"], []any{}), "organizer": values.Or(event["organizer"], ""),
		"source": "invite", "sourceEmailId": context.SourceEmailId, "sourceEmailIds": calendar.history(existing, context.SourceEmailId),
		"icsUid": event["uid"], "recurrence": values.Or(event["rrule"], ""),
		"recurrenceDates": values.Or(event["recurrenceDates"], []any{}), "exceptionDates": values.Or(event["exceptionDates"], []any{}),
		"timeZone": values.Or(event["timeZone"], "UTC"), "status": values.Or(event["status"], "confirmed"),
		"createdAt": values.Or(existing["createdAt"], now), "updatedAt": now,
	}
	return true, calendar.replace(documentId, next)
}

func (calendar CalendarImport) upsertException(context ImportContext, event map[string]any) (bool, error) {
	values := RecordValues{}
	_, master, err := calendar.Events.FindByIcsUid(context.Recipient, values.Text(event["uid"]))
	recurrenceId := values.Text(event["recurrenceId"])
	if err != nil || master == nil || recurrenceId == "" {
		return false, err
	}
	documentId, existing, err := calendar.Events.FindException(values.String(master["id"]), recurrenceId)
	if err != nil {
		return false, err
	}
	history := master
	if existing != nil {
		history = existing
	}
	now := EpochTime{}.NowText()
	next := map[string]any{
		"id": values.Or(existing["id"], CalendarEventRules{}.NewId()), "userEmail": context.Recipient,
		"title": values.Or(event["summary"], master["title"]), "start": values.Or(event["start"], master["start"]), "end": values.Or(event["end"], master["end"]),
		"allDay": values.Truthy(event["allDay"]), "location": values.Or(event["location"], ""), "description": values.Or(event["description"], ""),
		"attendees": values.Or(values.Or(event["attendees"], master["attendees"]), []any{}), "organizer": values.Or(values.Or(event["organizer"], master["organizer"]), ""),
		"source": "invite", "sourceEmailId": context.SourceEmailId, "sourceEmailIds": calendar.history(history, context.SourceEmailId),
		"icsUid": event["uid"], "seriesId": master["id"], "recurrenceId": recurrenceId,
		"recurrence": "", "recurrenceDates": []any{}, "exceptionDates": []any{},
		"timeZone": values.Or(values.Or(event["timeZone"], master["timeZone"]), "UTC"), "status": values.Or(event["status"], "confirmed"),
		"createdAt": values.Or(existing["createdAt"], now), "updatedAt": now,
	}
	return true, calendar.replace(documentId, next)
}

// replace deletes the prior document (FYLO's put always inserts under a new
// document id) and stores the event.
func (calendar CalendarImport) replace(documentId string, event map[string]any) error {
	if documentId != "" {
		if err := calendar.Events.Delete(documentId); err != nil {
			return err
		}
	}
	_, err := calendar.Events.Put(ProviderAccounts{}.Defined(event))
	return err
}

// history answers the last 32 distinct source email ids of an event, with
// the current one added.
func (CalendarImport) history(event map[string]any, current string) []any {
	values := RecordValues{}
	stored, _ := values.List(event["sourceEmailIds"])
	candidates := append(append([]any{}, stored...), event["sourceEmailId"], current)
	seen := map[string]bool{}
	history := []any{}
	for _, candidate := range candidates {
		text, isText := candidate.(string)
		if isText && text != "" && !seen[text] {
			seen[text] = true
			history = append(history, text)
		}
	}
	if len(history) > 32 {
		history = history[len(history)-32:]
	}
	return history
}

// candidates answers each distinct iCalendar payload of a message: the
// text body when it is one, and every calendar attachment.
func (CalendarImport) candidates(textBody string, attachments []ParsedAttachment) []string {
	candidates := []string{}
	seen := map[string]bool{}
	if (Ics{}).LooksLike(textBody) {
		candidates = append(candidates, textBody)
		seen[textBody] = true
	}
	for _, attachment := range attachments {
		if !(CalendarAttachments{}).IsCalendar(attachment) {
			continue
		}
		content := attachment.Content
		if content == nil {
			content, _ = base64.StdEncoding.DecodeString(attachment.ContentBase64)
		}
		text := strings.TrimPrefix(string(content), "\xef\xbb\xbf")
		if !utf8.ValidString(text) {
			text = strings.ToValidUTF8(text, string(utf8.RuneError))
		}
		if (Ics{}).LooksLike(text) && !seen[text] {
			candidates = append(candidates, text)
			seen[text] = true
		}
	}
	return candidates
}
