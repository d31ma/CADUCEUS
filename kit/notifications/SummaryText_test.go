package notifications

import (
	"strconv"
	"strings"
	"testing"
)

func summaryItems(count int) []SummaryItem {
	items := []SummaryItem{}
	for index := range count {
		items = append(items, SummaryItem{Kind: "mention", AuthorDisplayName: "Ana" + strconv.Itoa(index), CreatedAt: "2026-11-10T12:00:00Z"})
	}
	return items
}

func TestSummaryForOneItem(t *testing.T) {
	subject, body := SummaryText{}.Build("Bo", []SummaryItem{{Kind: "dm", AuthorDisplayName: "Ana", CreatedAt: "2026-11-10T12:00:00Z"}})
	expectSame(t, subject, "You have 1 unread notification in Mesail")
	expectSame(t, body, "Hello Bo,\n\nYou have 1 unread notification in Mesail:\n\n- New direct message from Ana (2026-11-10T12:00:00Z)\n\nOpen Mesail to read them.\n")
}

func TestSummaryForThreeItemsWithoutName(t *testing.T) {
	subject, body := SummaryText{}.Build("", summaryItems(3))
	expectSame(t, subject, "You have 3 unread notifications in Mesail")
	expectSame(t, strings.Split(body, "\n")[0], "Hello,")
	expectSame(t, strings.Count(body, "\n- "), 3)
}

func TestSummaryForTwentyItemsHasNoRemainder(t *testing.T) {
	_, body := SummaryText{}.Build("Bo", summaryItems(20))
	expectSame(t, strings.Count(body, "\n- "), 20)
	expectSame(t, strings.Contains(body, " more"), false)
}

func TestSummaryForManyItemsShowsTwentyAndRemainder(t *testing.T) {
	subject, body := SummaryText{}.Build("Bo", summaryItems(25))
	expectSame(t, subject, "You have 25 unread notifications in Mesail")
	expectSame(t, strings.Count(body, "\n- "), 20)
	expectSame(t, strings.Contains(body, "- Ana19 mentioned you"), true)
	expectSame(t, strings.Contains(body, "Ana20"), false)
	expectSame(t, strings.Contains(body, "\nand 5 more\n"), true)
}
