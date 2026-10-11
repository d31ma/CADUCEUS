import (
	"crypto/rand"
	"math"
	"math/big"
	"net/http"
	"regexp"
	"strconv"
	"strings"
	"unicode/utf16"
)

// CalendarEventRules validate and build the events the calendar routes
// store, and decide which events and occurrences a caller may still change.
type CalendarEventRules struct{}

// Times answers the trimmed title, start and end of an event body, or the
// problem with them.
func (CalendarEventRules) Times(body map[string]any) (string, string, string, string) {
	title := Security{}.Trim(RecordValues{}.Text(body["title"]))
	if title == "" {
		return "", "", "", "title is required"
	}
	start, end, problem := CalendarDate{}.ValidateRange(body["start"], body["end"], body["allDay"] == true)
	return title, start, end, problem
}

// Attendees answers the attendees of a body: entries without an email are
// dropped, emails are lower-cased and names and statuses defaulted.
func (rules CalendarEventRules) Attendees(input any) []any {
	entries, isList := RecordValues{}.List(input)
	attendees := []any{}
	if !isList {
		return attendees
	}
	for _, entry := range entries {
		attendee, _ := RecordValues{}.Object(entry)
		email, isText := attendee["email"].(string)
		if attendee == nil || !isText {
			continue
		}
		status := RecordValues{}.Text(attendee["status"])
		if !rules.isStatus(status) {
			status = "needsAction"
		}
		attendees = append(attendees, map[string]any{"email": strings.ToLower(email), "name": RecordValues{}.Text(attendee["name"]), "status": status})
	}
	return attendees
}

// Built answers a new manual event of the caller from a validated body.
func (rules CalendarEventRules) Built(body map[string]any, title string, start string, end string, owner string) map[string]any {
	values := RecordValues{}
	now := EpochTime{}.NowText()
	organizer := owner
	if text, isText := body["organizer"].(string); isText {
		organizer = strings.ToLower(text)
	}
	event := map[string]any{
		"id":              rules.NewId(),
		"userEmail":       owner,
		"title":           title,
		"start":           start,
		"end":             end,
		"allDay":          values.Truthy(body["allDay"]),
		"location":        Security{}.Trim(values.Text(body["location"])),
		"description":     values.Text(body["description"]),
		"attendees":       rules.Attendees(body["attendees"]),
		"organizer":       organizer,
		"source":          "manual",
		"recurrence":      values.Text(body["recurrence"]),
		"recurrenceDates": []any{},
		"exceptionDates":  []any{},
		"timeZone":        "UTC",
		"status":          "confirmed",
		"createdAt":       now,
		"updatedAt":       now,
	}
	if key, isText := body["idempotencyKey"].(string); isText {
		event["idempotencyKey"] = key
	}
	if minutes, isNumber := values.Number(body["reminderMinutes"]); isNumber {
		event["reminderMinutes"] = minutes
	}
	if zone, isText := body["timeZone"].(string); isText {
		event["timeZone"] = zone
	}
	return event
}

// IdempotencyKey answers the client's key when it is usable, or a fresh one
// for the operation.
func (rules CalendarEventRules) IdempotencyKey(value any, operation string) string {
	candidate := Security{}.Trim(RecordValues{}.Text(value))
	if candidate != "" && rules.length(candidate) <= rules.longestKey() {
		return candidate
	}
	return "calendar-" + operation + "-" + Security{}.RandomUuid()
}

// NewId answers a new event id: evt-, the time in base 36, a dash and eight
// random base-36 characters.
func (CalendarEventRules) NewId() string {
	digits := "0123456789abcdefghijklmnopqrstuvwxyz"
	random := make([]byte, 8)
	for index := range random {
		pick, _ := rand.Int(rand.Reader, big.NewInt(int64(len(digits))))
		random[index] = digits[pick.Int64()]
	}
	return "evt-" + strconv.FormatInt(EpochTime{}.Now(), 36) + "-" + string(random)
}

// ProviderFailure answers a failed provider mutation: a changed resource is
// a conflict, a missing one is not found, and the rest are account
// failures.
func (CalendarEventRules) ProviderFailure(err error) *http.Response {
	message := err.Error()
	if regexp.MustCompile(`(?i)changed|conflict`).MatchString(message) {
		if message == "" {
			message = "Provider calendar changed"
		}
		return Answer{}.Conflict(message)
	}
	if regexp.MustCompile(`(?i)not found`).MatchString(message) {
		return Answer{}.NotFound(message)
	}
	return ExternalAccounts{}.Failure(err)
}

// MutationRefusal answers why the caller may not change an event (or one
// occurrence of it), or nil: invitations from someone else are read-only,
// and so are past events and past occurrences.
func (rules CalendarEventRules) MutationRefusal(event map[string]any, owner string, occurrenceStart string) *http.Response {
	values := RecordValues{}
	if rules.ReadOnly(event, owner) {
		return Answer{}.Forbidden("Invited events are read-only; respond from the invitation instead")
	}
	now := EpochTime{}.Now()
	if occurrenceStart != "" {
		if values.Truthy(event["allDay"]) && (CalendarDate{}).IsDate(occurrenceStart) {
			if occurrenceStart < (CalendarDate{}).TodayInTimeZone(values.Text(values.Or(event["timeZone"], "UTC")), now) {
				return Answer{}.Conflict("Past event occurrences are read-only")
			}
			return nil
		}
		instant := EpochTime{}.Parse(occurrenceStart)
		if math.IsNaN(instant) {
			return Answer{}.BadRequest("occurrenceStart must be an ISO 8601 date-time string")
		}
		if instant < float64(now) {
			return Answer{}.Conflict("Past event occurrences are read-only")
		}
		return nil
	}
	if !values.Truthy(event["recurrence"]) && (CalendarDate{}).IsPast(event, now) {
		return Answer{}.Conflict("Past events are read-only")
	}
	return nil
}

// ReadOnly reports whether the caller only sees an event: an invitation
// from someone else.
func (CalendarEventRules) ReadOnly(event map[string]any, owner string) bool {
	values := RecordValues{}
	return event["source"] == "invite" && values.Truthy(event["organizer"]) && strings.ToLower(values.String(event["organizer"])) != strings.ToLower(owner)
}

// Occurrence answers the generated occurrence of a series with a recurrence
// id, or nil.
func (CalendarEventRules) Occurrence(series map[string]any, recurrenceId string) map[string]any {
	instant := recurrenceId
	if (CalendarDate{}).IsDate(recurrenceId) {
		instant = recurrenceId + "T00:00:00.000Z"
	}
	moment := EpochTime{}.Parse(instant)
	padding := float64(2 * 24 * 60 * 60 * 1000)
	if math.IsNaN(moment) {
		return nil
	}
	occurrences, problem := CalendarRecurrence{}.Expand([]map[string]any{series}, EpochTime{}.Text(int64(moment-padding)), EpochTime{}.Text(int64(moment+padding)))
	if problem != "" {
		return nil
	}
	for _, candidate := range occurrences {
		if candidate["recurrenceId"] == recurrenceId {
			return candidate
		}
	}
	return nil
}

// MasterRange applies an edited occurrence's move and new length to its
// recurring master: the occurrence date itself never becomes the master's.
func (CalendarEventRules) MasterRange(master map[string]any, occurrence map[string]any, start string, end string, allDay bool) (string, string) {
	values := RecordValues{}
	if allDay {
		moved := CalendarDate{}.DaysBetween(values.Text(occurrence["start"]), start)
		length := CalendarDate{}.DaysBetween(start, end)
		masterStart := CalendarDate{}.AddDays(values.Text(master["start"]), moved)
		return masterStart, CalendarDate{}.AddDays(masterStart, length)
	}
	parse := func(value any) float64 { return EpochTime{}.Parse(values.String(value)) }
	move := parse(start) - parse(occurrence["start"])
	length := parse(end) - parse(start)
	masterStart := parse(master["start"]) + move
	return EpochTime{}.Text(int64(masterStart)), EpochTime{}.Text(int64(masterStart + length))
}

func (CalendarEventRules) isStatus(status string) bool {
	for _, known := range []string{"accepted", "declined", "tentative", "needsAction", "delegated", "completed"} {
		if status == known {
			return true
		}
	}
	return false
}

// length counts text in UTF-16 code units, as JavaScript's length did.
func (CalendarEventRules) length(text string) int {
	return len(utf16.Encode([]rune(text)))
}

func (CalendarEventRules) longestKey() int { return 128 }
