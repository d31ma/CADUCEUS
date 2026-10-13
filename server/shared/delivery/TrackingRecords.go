import (
	"encoding/json"
	"math"
	"regexp"
	"strings"
	"time"
	"unicode/utf16"
)

// TrackingRecords reads the open and click tracking fields of stored mail:
// opensJson, clicksJson and trackedLinksJson hold JSON lists, at most
// MaximumEventsPerMessage events each.
type TrackingRecords struct{}

// MaximumEventsPerMessage is how many opens or clicks one message keeps.
func (TrackingRecords) MaximumEventsPerMessage() int { return 100 }

// MaximumActivityMessages is how many sent messages the feed reads.
func (TrackingRecords) MaximumActivityMessages() int { return 500 }

// MaximumActivityEvents is the most events the feed answers.
func (TrackingRecords) MaximumActivityEvents() float64 { return 100 }

// DayMilliseconds is the length of one day.
func (TrackingRecords) DayMilliseconds() float64 { return 24 * 60 * 60 * 1000 }

// ActivityEvent answers one open or click event of the feed.
func (records TrackingRecords) ActivityEvent(kind string, email map[string]any, item map[string]any, timestamp string, fallbackRecipient string) map[string]any {
	values := RecordValues{}
	id := records.CleanText(item["id"], 160)
	if id == "" {
		id = kind + "-" + values.String(email["id"]) + "-" + timestamp
	}
	recipient := records.NormalizeEmail(item["recipient"])
	if recipient == "" {
		recipient = fallbackRecipient
	}
	event := map[string]any{
		"id":        id,
		"type":      kind,
		"messageId": values.String(values.Or(email["id"], "")),
		"threadId":  values.String(values.Or(email["threadId"], "")),
		"subject":   records.CleanText(email["subject"], 500),
		"recipient": recipient,
		"timestamp": timestamp,
	}
	if kind == "click" {
		event["url"] = values.String(values.Or(item["url"], ""))
		event["title"] = records.CleanText(item["title"], 200)
	}
	return event
}

// IsOwnedSent reports whether email is a Sent copy of the owner: its own
// owner field when it has one, else one of the owner's sender addresses.
func (records TrackingRecords) IsOwnedSent(email map[string]any, owner string, senders map[string]bool, sentFolders map[string]string) bool {
	if records.folder(email, "") != records.namedFolder(sentFolders, email, "sent") {
		return false
	}
	if explicitOwner := records.NormalizeEmail(email["ownerEmail"]); explicitOwner != "" {
		return explicitOwner == owner
	}
	return senders[records.NormalizeEmail(email["sender"])]
}

// IsReceivedByOwner reports whether email reached one of the owner's
// addresses outside the Sent and Drafts folders.
func (records TrackingRecords) IsReceivedByOwner(email map[string]any, senders map[string]bool, sentFolders map[string]string, draftFolders map[string]string) bool {
	folder := records.folder(email, "inbox")
	if folder == records.namedFolder(sentFolders, email, "sent") || folder == records.namedFolder(draftFolders, email, "drafts") {
		return false
	}
	return records.reaches(email, senders)
}

func (records TrackingRecords) reaches(email map[string]any, senders map[string]bool) bool {
	for _, address := range records.RecipientList(email) {
		if senders[address] {
			return true
		}
	}
	return false
}

func (TrackingRecords) folder(email map[string]any, fallback string) string {
	return strings.ToLower(RecordValues{}.String(RecordValues{}.Or(email["folder"], fallback)))
}

func (TrackingRecords) namedFolder(folders map[string]string, email map[string]any, fallback string) string {
	if name := folders[(RecordValues{}).String(email["domain"])]; name != "" {
		return name
	}
	return fallback
}

// RecipientList answers the distinct valid To, Cc and recipient addresses.
func (records TrackingRecords) RecipientList(email map[string]any) []string {
	values := []any{}
	for _, field := range []string{"to", "cc"} {
		list, _ := RecordValues{}.List(email[field])
		values = append(values, list...)
	}
	for _, part := range strings.Split(RecordValues{}.String(RecordValues{}.Or(email["recipient"], "")), ",") {
		values = append(values, part)
	}
	addresses := []string{}
	seen := map[string]bool{}
	for _, value := range values {
		address := records.NormalizeEmail(value)
		if address != "" && !seen[address] {
			seen[address] = true
			addresses = append(addresses, address)
		}
	}
	return addresses
}

// ReplyThreads answers the threads of sent mail that a recipient answered.
func (records TrackingRecords) ReplyThreads(received []map[string]any, sent []map[string]any, senders map[string]bool) map[string]bool {
	recipients := map[string]map[string]bool{}
	for _, email := range sent {
		thread := records.thread(email)
		if thread == "" {
			continue
		}
		if recipients[thread] == nil {
			recipients[thread] = map[string]bool{}
		}
		for _, address := range records.RecipientList(email) {
			recipients[thread][address] = true
		}
	}
	replied := map[string]bool{}
	for _, email := range received {
		thread := records.thread(email)
		if thread != "" && records.reaches(email, senders) && recipients[thread][records.NormalizeEmail(email["sender"])] {
			replied[thread] = true
		}
	}
	return replied
}

func (TrackingRecords) thread(email map[string]any) string {
	if !(RecordValues{}).Truthy(email["threadId"]) {
		return ""
	}
	return RecordValues{}.String(email["threadId"])
}

// Replied reports whether a sent email's thread was answered.
func (records TrackingRecords) Replied(email map[string]any, repliesByThread map[string]bool) bool {
	thread := records.thread(email)
	return thread != "" && repliesByThread[thread]
}

// HasOpen reports whether a sent email was opened.
func (records TrackingRecords) HasOpen(email map[string]any) bool {
	values := RecordValues{}
	return values.Truthy(email["openedAt"]) || values.Numeric(email["openCount"]) > 0 || len(records.ReadArray(email["opensJson"])) > 0
}

// HasClick reports whether a link of a sent email was clicked.
func (records TrackingRecords) HasClick(email map[string]any) bool {
	values := RecordValues{}
	return values.Numeric(email["clickCount"]) > 0 || len(records.ReadArray(values.Or(email["clicksJson"], email["clicks"]))) > 0
}

// ReadArray answers a stored event list (a list or its JSON text), at most
// MaximumEventsPerMessage items.
func (records TrackingRecords) ReadArray(value any) []any {
	list, isList := RecordValues{}.List(value)
	if text, isText := value.(string); isText && text != "" {
		var parsed any
		_ = json.Unmarshal([]byte(text), &parsed)
		list, isList = RecordValues{}.List(parsed)
	}
	if !isList {
		return []any{}
	}
	if len(list) > records.MaximumEventsPerMessage() {
		list = list[:records.MaximumEventsPerMessage()]
	}
	return append([]any{}, list...)
}

// NormalizeTrackingId answers a well-formed tracking id, or "".
func (TrackingRecords) NormalizeTrackingId(value any) string {
	id := Security{}.Trim(RecordValues{}.Text(value))
	if !regexp.MustCompile(`^[A-Za-z0-9_-]{16,128}$`).MatchString(id) {
		return ""
	}
	return id
}

// NormalizeEmail answers a lower-case address of at most 320 characters, or
// "".
func (records TrackingRecords) NormalizeEmail(value any) string {
	email := strings.ToLower(Security{}.Trim(RecordValues{}.Text(value)))
	if !regexp.MustCompile(`^[^\s@]+@[^\s@]+\.[^\s@]+$`).MatchString(email) || records.length(email) > records.maximumAddressLength() {
		return ""
	}
	return email
}

func (TrackingRecords) maximumAddressLength() int { return 320 }

// CleanTimestamp answers a stored time as ISO text, or "".
func (TrackingRecords) CleanTimestamp(value any) string {
	text, isText := value.(string)
	at := math.NaN()
	if isText {
		at = EpochTime{}.Parse(text)
	}
	if math.IsNaN(at) {
		return ""
	}
	return EpochTime{}.Text(int64(at))
}

// TimestampOf answers a stored time in milliseconds, or 0.
func (TrackingRecords) TimestampOf(value any) float64 {
	at := RecordValues{}.Numeric(value)
	if text, isText := value.(string); isText {
		at = EpochTime{}.Parse(text)
	}
	if math.IsNaN(at) || math.IsInf(at, 0) {
		return 0
	}
	return at
}

// Within reports whether a stored time lies between two instants.
func (records TrackingRecords) Within(value any, from float64, to float64) bool {
	at := records.TimestampOf(value)
	return at >= from && at <= to
}

// Rate answers numerator / denominator as a percentage with one decimal.
func (TrackingRecords) Rate(numerator int, denominator int) float64 {
	if denominator <= 0 {
		return 0
	}
	return math.Floor(float64(numerator)/float64(denominator)*1000+0.5) / 10
}

// StartOfDay answers the start of the UTC day of an instant.
func (TrackingRecords) StartOfDay(value float64) float64 {
	date := time.UnixMilli(int64(value)).UTC()
	return float64(time.Date(date.Year(), date.Month(), date.Day(), 0, 0, 0, 0, time.UTC).UnixMilli())
}

// CleanText answers text without control characters, trimmed, and at most
// maximum characters long.
func (records TrackingRecords) CleanText(value any, maximum int) string {
	text, isText := value.(string)
	if !isText {
		return ""
	}
	cleaned := Security{}.Trim(regexp.MustCompile(`[\x00-\x1f\x7f]`).ReplaceAllString(text, " "))
	units := utf16.Encode([]rune(cleaned))
	if len(units) > maximum {
		units = units[:maximum]
	}
	return string(utf16.Decode(units))
}

func (TrackingRecords) length(text string) int {
	return len(utf16.Encode([]rune(text)))
}
