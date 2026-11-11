package search

import (
	"cmp"
	"strings"

	"mesail.dev/packages/textindex"
)

type Hits struct{}

func (hits Hits) Page(found []Document, filter Filter) Page {
	shown := found[:min(len(found), filter.Limit)]
	return Page{Items: hits.hits(shown, filter.Terms), NextCursor: hits.nextCursor(shown, len(found) > filter.Limit)}
}

func (hits Hits) hits(documents []Document, terms []string) []Hit {
	items := []Hit{}
	for _, document := range documents {
		items = append(items, hits.hit(document, terms))
	}
	return items
}

func (hits Hits) hit(document Document, terms []string) Hit {
	source := cmp.Or(document.Text, document.Subject, strings.Join(document.FileNames, " "))
	snippet := textindex.Snippet{Width: hits.snippetWidth()}.Around(source, terms)
	return Hit{MessageId: document.MessageId, ConversationId: document.ConversationId, Snippet: snippet, CreatedAt: document.CreatedAt}
}

func (Hits) nextCursor(shown []Document, more bool) string {
	if !more {
		return ""
	}
	last := shown[len(shown)-1]
	return textindex.Cursor{CreatedAt: last.CreatedAt, DocumentId: last.MessageId}.Encode()
}

func (Hits) snippetWidth() int {
	return 160
}
