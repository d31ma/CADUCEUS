package notifications

import (
	"strconv"
	"strings"
)

type SummaryText struct{}

func (summary SummaryText) Build(displayName string, items []SummaryItem) (string, string) {
	count := summary.countText(len(items))
	lines := []string{summary.greeting(displayName), "", count + ":", ""}
	lines = append(lines, summary.itemLines(items[:min(len(items), summary.mostLines())])...)
	lines = append(lines, summary.remainder(len(items))...)
	lines = append(lines, "", "Open Mesail to read them.")
	return count, strings.Join(lines, "\n") + "\n"
}

func (SummaryText) mostLines() int {
	return 20
}

func (summary SummaryText) countText(count int) string {
	return "You have " + strconv.Itoa(count) + " unread " + summary.noun(count) + " in Mesail"
}

func (SummaryText) noun(count int) string {
	if count == 1 {
		return "notification"
	}
	return "notifications"
}

func (SummaryText) greeting(displayName string) string {
	if displayName == "" {
		return "Hello,"
	}
	return "Hello " + displayName + ","
}

func (summary SummaryText) itemLines(items []SummaryItem) []string {
	lines := []string{}
	for _, item := range items {
		lines = append(lines, summary.itemLine(item))
	}
	return lines
}

func (SummaryText) itemLine(item SummaryItem) string {
	_, body := PushText{}.For(item.Kind, item.AuthorDisplayName)
	return "- " + body + " (" + item.CreatedAt + ")"
}

func (summary SummaryText) remainder(count int) []string {
	if count <= summary.mostLines() {
		return nil
	}
	return []string{"and " + strconv.Itoa(count-summary.mostLines()) + " more"}
}
