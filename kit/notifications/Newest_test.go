package notifications

import "testing"

func newestItems() []Sortable {
	return []Sortable{
		{CreatedAt: "2026-11-10T10:00:00Z", Id: "a"},
		{CreatedAt: "2026-11-10T12:00:00Z", Id: "b"},
		{CreatedAt: "2026-11-10T11:00:00Z", Id: "c"},
		{CreatedAt: "2026-11-10T12:00:00Z", Id: "d"},
		{CreatedAt: "2026-11-10T09:00:00Z", Id: "e"},
	}
}

func TestNewestFirstPage(t *testing.T) {
	order, next, err := Newest{}.Page(newestItems(), "", 2)
	expectNoError(t, err)
	expectSame(t, order, []int{3, 1})
	expectSame(t, next, Cursor{}.Encode("2026-11-10T12:00:00Z", "b"))
}

func TestNewestFollowsCursor(t *testing.T) {
	order, next, err := Newest{}.Page(newestItems(), Cursor{}.Encode("2026-11-10T12:00:00Z", "b"), 2)
	expectNoError(t, err)
	expectSame(t, order, []int{2, 0})
	expectSame(t, next, Cursor{}.Encode("2026-11-10T10:00:00Z", "a"))
}

func TestNewestLastPageHasNoCursor(t *testing.T) {
	order, next, err := Newest{}.Page(newestItems(), Cursor{}.Encode("2026-11-10T10:00:00Z", "a"), 2)
	expectNoError(t, err)
	expectSame(t, order, []int{4})
	expectSame(t, next, "")
}

func TestNewestCursorOfRemovedItem(t *testing.T) {
	order, _, err := Newest{}.Page(newestItems(), Cursor{}.Encode("2026-11-10T11:30:00Z", "z"), 10)
	expectNoError(t, err)
	expectSame(t, order, []int{2, 0, 4})
}

func TestNewestExactFitHasNoCursor(t *testing.T) {
	order, next, err := Newest{}.Page(newestItems(), "", 5)
	expectNoError(t, err)
	expectSame(t, order, []int{3, 1, 2, 0, 4})
	expectSame(t, next, "")
}

func TestNewestEmptyItems(t *testing.T) {
	order, next, err := Newest{}.Page(nil, "", 50)
	expectNoError(t, err)
	expectSame(t, order, []int{})
	expectSame(t, next, "")
}

func TestNewestTreatsZeroLimitAsOne(t *testing.T) {
	order, _, err := Newest{}.Page(newestItems(), "", 0)
	expectNoError(t, err)
	expectSame(t, order, []int{3})
}

func TestNewestRefusesBadCursor(t *testing.T) {
	_, _, err := Newest{}.Page(newestItems(), "%%", 2)
	expectInvalidPage(t, err)
}
