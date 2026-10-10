import (
	"encoding/base64"
	"fmt"
	"os"
	"net/http"
	"strings"
)

// CalendarInvitesRoute answers /calendar/invites/respond: the caller's RSVP
// to an imported invitation.
type CalendarInvitesRoute struct {
	Access *AccessHelper
	Events *CalendarEventsHelper
}

// Respond stores the caller's answer on the event and, when the invitation
// came with a UID and an organizer, sends the organizer an iCalendar REPLY.
// A reply that fails does not undo the stored answer.
func (route CalendarInvitesRoute) Respond(request *http.Request) *http.Response {
	body, refusal := RequestBody{}.Object(request)
	if refusal != nil {
		return refusal
	}
	caller, refusal := route.Access.Signed(request)
	if refusal != nil {
		return refusal
	}
	eventId := RecordValues{}.Text(body["eventId"])
	status := RecordValues{}.Text(body["status"])
	if eventId == "" {
		return Answer{}.BadRequest("eventId required")
	}
	if status != "accepted" && status != "declined" && status != "tentative" {
		return Answer{}.BadRequest("status must be accepted, declined, or tentative")
	}
	documentId, event, err := route.Events.FindById(eventId)
	if err != nil {
		return RouteAccess{}.Failure(err)
	}
	if event == nil || documentId == "" || event["userEmail"] != caller.Email {
		return Answer{}.NotFound("Event not found")
	}
	if event["status"] == "cancelled" {
		return Answer{}.Conflict("Cancelled invitations cannot be answered")
	}
	updated := StoredRecords{}.Without(event)
	updated["attendees"] = route.answered(event["attendees"], strings.ToLower(caller.Email), status)
	updated["updatedAt"] = EpochTime{}.NowText()
	if err := route.Events.Delete(documentId); err != nil {
		return RouteAccess{}.Failure(err)
	}
	if _, err := route.Events.Put(updated); err != nil {
		return RouteAccess{}.Failure(err)
	}
	return Answer{}.Ok(map[string]any{"responded": true, "status": status, "replySent": route.reply(caller, event, status)})
}

// answered answers the attendees with the caller's status set, adding the
// caller when they are not listed.
func (CalendarInvitesRoute) answered(input any, me string, status string) []any {
	attendees, _ := RecordValues{}.List(input)
	answered := append([]any{}, attendees...)
	for index, entry := range answered {
		attendee, _ := RecordValues{}.Object(entry)
		if strings.ToLower(RecordValues{}.Text(RecordValues{}.Or(attendee["email"], ""))) == me {
			changed := StoredRecords{}.Without(attendee)
			changed["status"] = status
			answered[index] = changed
			return answered
		}
	}
	return append(answered, map[string]any{"email": me, "status": status})
}

// reply sends the organizer an iCalendar REPLY and reports whether it went.
func (CalendarInvitesRoute) reply(caller RouteCaller, event map[string]any, status string) bool {
	values := RecordValues{}
	if !values.Truthy(event["icsUid"]) || !values.Truthy(event["organizer"]) {
		return false
	}
	ics := Ics{}.Reply(map[string]any{
		"uid": event["icsUid"], "organizer": event["organizer"], "attendee": caller.Email, "status": status,
		"summary": event["title"], "start": event["start"], "end": event["end"], "allDay": values.Truthy(event["allDay"]),
	})
	verb := map[string]string{"accepted": "Accepted", "declined": "Declined"}[status]
	if verb == "" {
		verb = "Tentative"
	}
	title := values.String(event["title"])
	message := map[string]any{
		"to":      []string{values.String(event["organizer"])},
		"subject": verb + ": " + title,
		"text":    caller.Email + ` responded "` + status + `" to "` + title + `".`,
		"attachments": []map[string]any{{
			"filename": "invite.ics", "contentBase64": base64.StdEncoding.EncodeToString([]byte(ics)), "contentType": "text/calendar; method=REPLY; charset=utf-8",
		}},
	}
	if err := (CalendarReplyMail{}).Send(caller.Email, message); err != nil {
		fmt.Fprintf(os.Stderr, "[calendar] failed to send RSVP reply %s\n", err)
		return false
	}
	return true
}
