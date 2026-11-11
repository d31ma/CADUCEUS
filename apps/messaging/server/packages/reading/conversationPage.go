package reading

import (
	"cmp"
	"slices"
	"strconv"

	"mesail.dev/packages/core"
)

type conversationPage struct{}

func (page conversationPage) of(views []core.ConversationView, window pageWindow) core.ConversationList {
	sorted := page.sorted(views)
	start := min(window.cursor, len(sorted))
	end := min(start+window.limit, len(sorted))
	items := append([]core.ConversationView{}, sorted[start:end]...)
	return core.ConversationList{Items: items, NextCursor: page.nextCursor(end, len(sorted))}
}

func (page conversationPage) sorted(views []core.ConversationView) []core.ConversationView {
	sorted := append([]core.ConversationView{}, views...)
	slices.SortStableFunc(sorted, page.compare)
	return sorted
}

func (conversationPage) compare(first core.ConversationView, second core.ConversationView) int {
	return cmp.Or(
		cmp.Compare(second.Conversation.LastMessageAt, first.Conversation.LastMessageAt),
		cmp.Compare(first.ConversationId, second.ConversationId),
	)
}

func (conversationPage) nextCursor(end int, total int) string {
	if end >= total {
		return ""
	}
	return strconv.Itoa(end)
}
