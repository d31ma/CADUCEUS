package notifications

import (
	"cmp"
	"slices"
	"sort"
	"strings"
)

type Newest struct{}

func (newest Newest) Page(items []Sortable, cursor string, limit int) ([]int, string, error) {
	order := newest.sorted(items, newest.indexes(len(items)))
	start, err := newest.start(items, order, cursor)
	if err != nil {
		return nil, "", err
	}
	end := min(start+max(limit, 1), len(order))
	return order[start:end], newest.next(items, order, end), nil
}

func (Newest) indexes(count int) []int {
	order := make([]int, count)
	for index := range order {
		order[index] = index
	}
	return order
}

func (newest Newest) sorted(items []Sortable, order []int) []int {
	slices.SortStableFunc(order, func(first int, second int) int {
		return newest.compare(items[first], items[second])
	})
	return order
}

func (Newest) compare(first Sortable, second Sortable) int {
	return cmp.Or(strings.Compare(second.CreatedAt, first.CreatedAt), strings.Compare(second.Id, first.Id))
}

func (newest Newest) start(items []Sortable, order []int, cursor string) (int, error) {
	if cursor == "" {
		return 0, nil
	}
	return newest.after(items, order, cursor)
}

func (newest Newest) after(items []Sortable, order []int, cursor string) (int, error) {
	createdAt, id, err := Cursor{}.Decode(cursor)
	return newest.firstOlder(items, order, Sortable{CreatedAt: createdAt, Id: id}), err
}

func (newest Newest) firstOlder(items []Sortable, order []int, mark Sortable) int {
	return sort.Search(len(order), func(position int) bool {
		return newest.compare(items[order[position]], mark) > 0
	})
}

func (Newest) next(items []Sortable, order []int, end int) string {
	if end == len(order) {
		return ""
	}
	last := items[order[end-1]]
	return Cursor{}.Encode(last.CreatedAt, last.Id)
}
