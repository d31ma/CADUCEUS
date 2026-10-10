import (
	"math"
	"regexp"
	"strings"
	"unicode/utf8"
)

// CalendarInvite presents the calendar event the inbound importer linked to
// an email, with the message's own calendar attachment, so the reader can
// show an Accept/Decline/Tentative card.
type CalendarInvite struct {
	Events      *CalendarEventsHelper
	Attachments *AttachmentsHelper
}

// Best answers the attachment a calendar client would read: an .ics file,
// then text/calendar, then .vcs, then application/ics; nil for none.
func (invite CalendarInvite) Best(attachments []map[string]any) map[string]any {
	var best map[string]any
	bestRank := math.MaxInt
	for _, attachment := range attachments {
		if rank := invite.rank(attachment); rank < bestRank {
			best, bestRank = attachment, rank
		}
	}
	return best
}

func (invite CalendarInvite) rank(attachment map[string]any) int {
	filename := strings.ToLower(TextValue{}.Of(TextValue{}.Or(attachment["filename"], "")))
	contentType := invite.contentType(attachment["contentType"])
	switch {
	case strings.HasSuffix(filename, ".ics"):
		return 0
	case contentType == "text/calendar":
		return 1
	case strings.HasSuffix(filename, ".vcs"):
		return 2
	case contentType == "application/ics":
		return 3
	}
	return math.MaxInt
}

func (CalendarInvite) contentType(value any) string {
	return strings.ToLower(TextValue{}.Trim(strings.SplitN(TextValue{}.Of(TextValue{}.Or(value, "")), ";", 2)[0]))
}

// Visible answers the attachments without the selected calendar file, which
// the invite card offers, and without same-size calendar duplicates of it.
func (invite CalendarInvite) Visible(attachments []map[string]any, selected map[string]any) []map[string]any {
	if selected == nil {
		return attachments
	}
	visible := []map[string]any{}
	for _, attachment := range attachments {
		if (TextValue{}).Truthy(attachment["id"]) && attachment["id"] == selected["id"] {
			continue
		}
		contentType := invite.contentType(attachment["contentType"])
		size, isNumber := RecordValues{}.Number(attachment["size"])
		duplicate := contentType == "text/calendar" || contentType == "application/ics"
		if duplicate && isNumber && size == (RecordValues{}).NumberOf(selected["size"]) {
			continue
		}
		visible = append(visible, attachment)
	}
	return visible
}

// Find answers the invite card of one email for the user, or nil when no
// event of the user was imported from it.
func (invite CalendarInvite) Find(userEmail string, emailId string, email map[string]any, attachment map[string]any) (map[string]any, error) {
	events, err := invite.Events.List(userEmail, "", "")
	if err != nil {
		return nil, err
	}
	event := invite.linked(events, emailId)
	if event == nil {
		return nil, nil
	}
	return invite.presented(event, userEmail, email, attachment), nil
}

func (CalendarInvite) linked(events []map[string]any, emailId string) map[string]any {
	for _, event := range events {
		if event["sourceEmailId"] == emailId {
			return event
		}
		for _, id := range (TextValue{}).List(event["sourceEmailIds"]) {
			if id == emailId {
				return event
			}
		}
	}
	return nil
}

func (invite CalendarInvite) presented(event map[string]any, userEmail string, email map[string]any, attachment map[string]any) map[string]any {
	text := TextValue{}
	attendees := text.Or(event["attendees"], []any{})
	myStatus := any("needsAction")
	for _, attendee := range text.List(attendees) {
		fields, _ := attendee.(map[string]any)
		if strings.EqualFold(text.Of(text.Or(fields["email"], "")), userEmail) {
			myStatus = text.Or(fields["status"], "needsAction")
			break
		}
	}
	calendar, readable := invite.messageCalendar(attachment)
	method := calendar.Method
	if method == "" && event["status"] == "cancelled" {
		method = "CANCEL"
	} else if method == "" {
		method = "REQUEST"
	}
	var senderResponse any
	if method == "REPLY" {
		senderResponse = invite.senderResponse(calendar, readable, event, email)
	}
	var attached any
	if attachment != nil {
		attached = map[string]any{"id": attachment["id"], "filename": attachment["filename"], "contentType": attachment["contentType"], "size": attachment["size"]}
	}
	return map[string]any{
		"id": event["id"], "title": event["title"], "start": event["start"], "end": event["end"],
		"allDay": text.Truthy(event["allDay"]), "location": text.Or(event["location"], ""),
		"description": text.Or(event["description"], ""), "organizer": text.Or(event["organizer"], ""),
		"attendees": attendees, "status": text.Or(event["status"], "confirmed"), "myStatus": myStatus,
		"icsUid": text.Or(event["icsUid"], ""), "recurrence": text.Or(event["recurrence"], ""),
		"recurrenceId": text.Or(event["recurrenceId"], ""),
		"isRecurring":  text.Truthy(event["recurrence"]) || text.Truthy(event["seriesId"]),
		"messageMethod": method, "canRespond": method == "REQUEST" && event["status"] != "cancelled",
		"attachment": attached, "senderResponse": senderResponse,
	}
}

func (CalendarInvite) senderResponse(calendar InviteCalendar, readable bool, event map[string]any, email map[string]any) any {
	if !readable || len(calendar.Events) == 0 {
		return nil
	}
	chosen := calendar.Events[0]
	for _, candidate := range calendar.Events {
		if candidate.Uid == event["icsUid"] {
			chosen = candidate
			break
		}
	}
	if len(chosen.Attendees) == 0 {
		return nil
	}
	sender := (CalendarInvite{}).senderEmail(email["sender"])
	attendee := chosen.Attendees[0]
	for _, candidate := range chosen.Attendees {
		if strings.ToLower(TextValue{}.Of(candidate["email"])) == sender {
			attendee = candidate
			break
		}
	}
	return map[string]any{
		"email": attendee["email"], "name": TextValue{}.Or(attendee["name"], attendee["email"]),
		"status": TextValue{}.Or(attendee["status"], "needsAction"),
	}
}

// senderEmail answers the bracketed address of a sender, or its first bare
// address, in lower case.
func (CalendarInvite) senderEmail(sender any) string {
	value := strings.ToLower(TextValue{}.Trim(TextValue{}.Of(TextValue{}.Or(sender, ""))))
	if bracketed := regexp.MustCompile(`<([^<>\s]+@[^<>\s]+)>`).FindStringSubmatch(value); bracketed != nil {
		return bracketed[1]
	}
	return regexp.MustCompile("[a-z0-9.!#$%&'*+/=?^_`{|}~-]+@[a-z0-9.-]+").FindString(value)
}

// messageCalendar parses the selected calendar attachment; a malformed one
// never makes the email unreadable.
func (invite CalendarInvite) messageCalendar(attachment map[string]any) (InviteCalendar, bool) {
	if attachment == nil {
		return InviteCalendar{}, false
	}
	content, err := invite.Attachments.ReadContent(attachment)
	if err != nil {
		return InviteCalendar{}, false
	}
	text := string(content)
	if !utf8.ValidString(text) {
		text = strings.ToValidUTF8(text, "�")
	}
	return InviteCalendar{}.Parsed(text)
}
