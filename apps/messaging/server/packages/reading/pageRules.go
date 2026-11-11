package reading

import (
	"math"
	"strconv"

	"mesail.dev/packages/core"
)

type pageRules struct{}

func (rules pageRules) conversationWindow(page core.PageRequest) (pageWindow, error) {
	return rules.window(page, rules.firstOffset())
}

func (rules pageRules) messageWindow(page core.PageRequest) (pageWindow, error) {
	return rules.window(page, strconv.Itoa(math.MaxInt))
}

func (rules pageRules) window(page core.PageRequest, fallbackCursor string) (pageWindow, error) {
	limit, err := rules.checkedLimit(rules.defaultedLimit(page.Limit))
	if err != nil {
		return pageWindow{}, err
	}
	cursor, err := rules.parsedCursor(rules.cursorOr(page.Cursor, fallbackCursor))
	return pageWindow{limit: limit, cursor: cursor}, err
}

func (rules pageRules) defaultedLimit(limit int) int {
	if limit == 0 {
		return rules.defaultLimit()
	}
	return limit
}

func (rules pageRules) checkedLimit(limit int) (int, error) {
	if limit < 1 || limit > rules.largestLimit() {
		return 0, &core.InvalidRequest{Message: "limit must be between 1 and 100", Field: "limit"}
	}
	return limit, nil
}

func (pageRules) cursorOr(cursor string, fallbackCursor string) string {
	if cursor == "" {
		return fallbackCursor
	}
	return cursor
}

func (pageRules) parsedCursor(cursor string) (int, error) {
	value, err := strconv.Atoi(cursor)
	if err != nil || value < 0 {
		return 0, &core.InvalidRequest{Message: "the cursor is not valid", Field: "cursor"}
	}
	return value, nil
}

func (pageRules) firstOffset() string {
	return "0"
}

func (pageRules) defaultLimit() int {
	return 50
}

func (pageRules) largestLimit() int {
	return 100
}
