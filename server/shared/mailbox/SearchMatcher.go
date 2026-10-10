import (
	"math"
	"regexp"
	"strconv"
	"strings"
	"time"
)

// SearchMatcher decides whether one email and its attachments match a
// search expression, with relative dates read at Now in local time.
type SearchMatcher struct {
	Email       map[string]any
	Attachments []map[string]any
	Now         time.Time
}

// Matches reports whether the email matches (an empty search matches all).
func (matcher SearchMatcher) Matches(expression *SearchExpression) bool {
	if expression == nil {
		return true
	}
	switch expression.Type {
	case "and":
		return matcher.Matches(expression.Left) && matcher.Matches(expression.Right)
	case "or":
		return matcher.Matches(expression.Left) || matcher.Matches(expression.Right)
	case "not":
		return !matcher.Matches(expression.Expression)
	case "term":
		return matcher.term(expression.Field, strings.ToLower(expression.Value))
	}
	return false
}

func (SearchMatcher) contains(value any, needle string) bool {
	return strings.Contains(strings.ToLower(TextValue{}.Of(value)), needle)
}

func (matcher SearchMatcher) any(values []any, needle string) bool {
	for _, value := range values {
		if matcher.contains(value, needle) {
			return true
		}
	}
	return false
}

func (matcher SearchMatcher) term(field string, value string) bool {
	email := matcher.Email
	switch field {
	case "from":
		return matcher.contains(email["sender"], value)
	case "to":
		return matcher.any(matcher.recipients(), value)
	case "subject":
		return matcher.contains(email["subject"], value)
	case "body":
		return matcher.any([]any{email["body"], email["htmlBody"]}, value)
	case "in":
		return matcher.category(value)
	case "is":
		return matcher.status(value)
	case "has":
		return (value == "attachment" || value == "attachments") && len(matcher.Attachments) > 0
	case "before", "after", "since":
		return matcher.date(field, value)
	case "attachment", "filename":
		return matcher.any(matcher.filenames(), value)
	case "text":
		return matcher.any(matcher.fullText(), value)
	}
	return matcher.any(matcher.fullText(), field+":"+value)
}

func (matcher SearchMatcher) recipients() []any {
	text := TextValue{}
	recipients := []any{matcher.Email["recipient"]}
	for _, field := range []string{"to", "cc", "bcc"} {
		recipients = append(recipients, text.List(matcher.Email[field])...)
	}
	return recipients
}

func (matcher SearchMatcher) filenames() []any {
	names := []any{}
	for _, attachment := range matcher.Attachments {
		names = append(names, attachment["filename"])
	}
	return names
}

func (matcher SearchMatcher) fullText() []any {
	email := matcher.Email
	values := append([]any{email["sender"]}, matcher.recipients()...)
	values = append(values, email["subject"], email["snippet"], email["body"], email["htmlBody"])
	return append(values, matcher.filenames()...)
}

func (matcher SearchMatcher) category(value string) bool {
	if value == "all" {
		return true
	}
	text := TextValue{}
	folder := strings.ToLower(text.Of(text.Or(matcher.Email["systemFolder"], text.Or(matcher.Email["folder"], "inbox"))))
	if folder == value {
		return true
	}
	for _, label := range text.List(matcher.Email["labels"]) {
		if strings.ToLower(text.Of(label)) == value {
			return true
		}
	}
	return false
}

func (matcher SearchMatcher) status(value string) bool {
	read := matcher.Email["read"] == true
	starred := matcher.Email["starred"] == true
	switch value {
	case "read":
		return read
	case "unread":
		return !read
	case "starred":
		return starred
	case "unstarred":
		return !starred
	}
	return false
}

func (matcher SearchMatcher) date(field string, value string) bool {
	boundary, known := matcher.boundary(value)
	if !known {
		return true
	}
	timestamp := EpochTime{}.Parse(TextValue{}.Of(TextValue{}.Or(matcher.Email["receivedAt"], "")))
	if math.IsNaN(timestamp) {
		return false
	}
	if field == "before" {
		return timestamp < boundary
	}
	return timestamp > boundary
}

// boundary answers the instant a date value means: yesterday, N days ago,
// last week and last month start at local midnight; anything else is read
// as a date ("/" as "-"); false when it is no date.
func (matcher SearchMatcher) boundary(raw string) (float64, bool) {
	value := regexp.MustCompile(`^>=\s*`).ReplaceAllString(strings.ToLower(TextValue{}.Trim(raw)), "")
	now := matcher.Now
	midnight := func(moment time.Time) (float64, bool) {
		year, month, day := moment.Date()
		return float64(time.Date(year, month, day, 0, 0, 0, 0, moment.Location()).UnixMilli()), true
	}
	if value == "yesterday" {
		return midnight(now.AddDate(0, 0, -1))
	}
	if days := regexp.MustCompile(`^(\d+)\s+days?\s+ago$`).FindStringSubmatch(value); days != nil {
		count, _ := strconv.Atoi(days[1])
		return midnight(now.AddDate(0, 0, -count))
	}
	if value == "last week" {
		return midnight(now.AddDate(0, 0, -7))
	}
	if value == "last month" {
		return midnight(now.AddDate(0, -1, 0))
	}
	dated := strings.ReplaceAll(value, "/", "-")
	for _, candidate := range []string{dated, strings.ToUpper(dated)} {
		if parsed := (EpochTime{}).Parse(candidate); !math.IsNaN(parsed) {
			return parsed, true
		}
	}
	return 0, false
}
