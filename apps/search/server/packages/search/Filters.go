package search

import (
	"cmp"
	"net/url"
	"strconv"
	"strings"
	"time"
	"unicode/utf8"

	"mesail.dev/packages/textindex"
)

type Filters struct {
	Builder DocumentBuilder
}

func (filters Filters) Parse(values url.Values) (Filter, error) {
	terms, err := filters.terms(values.Get(filters.queryField()))
	if err != nil {
		return Filter{}, err
	}
	from, err := filters.instant(values.Get(filters.fromField()), filters.fromField(), 0)
	if err != nil {
		return Filter{}, err
	}
	to, err := filters.instant(values.Get(filters.toField()), filters.toField(), filters.day()-time.Microsecond)
	if err != nil {
		return Filter{}, err
	}
	hasAttachment, err := filters.flag(values.Get(filters.attachmentField()), filters.attachmentField())
	if err != nil {
		return Filter{}, err
	}
	isEmail, err := filters.flag(values.Get(filters.emailField()), filters.emailField())
	if err != nil {
		return Filter{}, err
	}
	cursor, err := filters.cursor(values.Get(filters.cursorField()))
	if err != nil {
		return Filter{}, err
	}
	limit, err := filters.limit(values.Get(filters.limitField()))
	if err != nil {
		return Filter{}, err
	}
	filter := Filter{Terms: terms, From: from, To: to, HasAttachment: hasAttachment, IsEmail: isEmail, Cursor: cursor, Limit: limit}
	filter.ConversationId = strings.TrimSpace(values.Get(filters.conversationField()))
	filter.Author = strings.TrimSpace(values.Get(filters.authorField()))
	return filter, nil
}

func (filters Filters) terms(text string) ([]string, error) {
	terms := textindex.Tokenizer{}.Terms(text)
	if utf8.RuneCountInString(text) > filters.maxQueryLength() || len(terms) > filters.maxTerms() {
		return nil, InvalidRequest{Message: "the search text is too long", Field: filters.queryField()}
	}
	return terms, nil
}

func (filters Filters) instant(text string, field string, dayEnd time.Duration) (string, error) {
	if text == "" {
		return "", nil
	}
	instant, err := filters.parsedTime(text, dayEnd)
	if err != nil {
		return "", InvalidRequest{Message: field + " must be a date or an RFC 3339 time", Field: field}
	}
	return filters.Builder.Stamp(instant), nil
}

func (Filters) parsedTime(text string, dayEnd time.Duration) (time.Time, error) {
	instant, err := time.Parse(time.RFC3339Nano, text)
	if err == nil {
		return instant, nil
	}
	day, err := time.Parse(time.DateOnly, text)
	return day.Add(dayEnd), err
}

func (Filters) flag(text string, field string) (*bool, error) {
	if text == "" {
		return nil, nil
	}
	value, err := strconv.ParseBool(text)
	if err != nil {
		return nil, InvalidRequest{Message: field + " must be true or false", Field: field}
	}
	return &value, nil
}

func (filters Filters) cursor(text string) (textindex.Cursor, error) {
	if text == "" {
		return textindex.Cursor{}, nil
	}
	cursor, err := textindex.Cursor{}.Decode(text)
	if err != nil {
		return textindex.Cursor{}, InvalidRequest{Message: err.Error(), Field: filters.cursorField()}
	}
	return cursor, nil
}

func (filters Filters) limit(text string) (int, error) {
	limit, err := strconv.Atoi(cmp.Or(text, strconv.Itoa(filters.defaultLimit())))
	if err != nil || limit < 1 || limit > filters.maxLimit() {
		return 0, InvalidRequest{Message: "limit must be a number from 1 to 100", Field: filters.limitField()}
	}
	return limit, nil
}

func (Filters) day() time.Duration {
	return 24 * time.Hour
}

func (Filters) maxQueryLength() int {
	return 500
}

func (Filters) maxTerms() int {
	return 16
}

func (Filters) defaultLimit() int {
	return 20
}

func (Filters) maxLimit() int {
	return 100
}

func (Filters) queryField() string {
	return "q"
}

func (Filters) fromField() string {
	return "from"
}

func (Filters) toField() string {
	return "to"
}

func (Filters) attachmentField() string {
	return "hasAttachment"
}

func (Filters) emailField() string {
	return "isEmail"
}

func (Filters) cursorField() string {
	return "cursor"
}

func (Filters) limitField() string {
	return "limit"
}

func (Filters) conversationField() string {
	return "conversationId"
}

func (Filters) authorField() string {
	return "author"
}
