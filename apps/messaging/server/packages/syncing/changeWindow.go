package syncing

import (
	"slices"
	"time"

	"mesail.dev/packages/core"
)

type changeWindow struct{}

func (window changeWindow) page(rows []core.Change, from time.Time, after core.ChangeCursor, most int) changePage {
	sorted := window.since(window.sorted(rows), from)
	split := slices.IndexFunc(sorted, func(row core.Change) bool {
		return core.ChangeCursors{}.Of(row).After(after)
	})
	if split < 0 {
		return changePage{}
	}
	consumed := window.consumed(sorted[split:], most)
	return changePage{
		messageIds: window.messageIds(sorted[:split+consumed]),
		next:       core.ChangeCursors{}.Of(sorted[split+consumed-1]).Text(),
		more:       split+consumed < len(sorted),
		fresh:      true,
	}
}

func (changeWindow) since(sorted []core.Change, from time.Time) []core.Change {
	first := slices.IndexFunc(sorted, func(row core.Change) bool {
		return !core.ChangeCursors{}.Of(row).At.Before(from)
	})
	if first < 0 {
		return nil
	}
	return sorted[first:]
}

func (changeWindow) sorted(rows []core.Change) []core.Change {
	cursors := core.ChangeCursors{}
	sorted := slices.Clone(rows)
	slices.SortStableFunc(sorted, func(first core.Change, second core.Change) int {
		return cursors.Of(first).Compare(cursors.Of(second))
	})
	return sorted
}

func (changeWindow) consumed(fresh []core.Change, most int) int {
	seen := map[string]bool{}
	for index, row := range fresh {
		seen[row.MessageId] = true
		if len(seen) > most {
			return index
		}
	}
	return len(fresh)
}

func (changeWindow) messageIds(rows []core.Change) []string {
	seen := map[string]bool{}
	newestFirst := []string{}
	for index := len(rows) - 1; index >= 0; index-- {
		newestFirst = append(newestFirst, rows[index].MessageId)
	}
	kept := slices.DeleteFunc(newestFirst, func(id string) bool {
		repeated := seen[id]
		seen[id] = true
		return repeated
	})
	slices.Reverse(kept)
	return kept
}
