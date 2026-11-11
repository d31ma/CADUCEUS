package search

import (
	"cmp"
	"slices"
	"strings"

	"mesail.dev/packages/textindex"
)

type Matches struct{}

func (matches Matches) Where(filter Filter) map[string]any {
	where := map[string]any{"deleted": false}
	matches.addText(where, "conversationId", filter.ConversationId)
	matches.addFlag(where, "hasAttachment", filter.HasAttachment)
	matches.addFlag(where, "isEmail", filter.IsEmail)
	return where
}

func (matches Matches) Months(months []string, filter Filter) []string {
	kept := []string{}
	for _, month := range months {
		kept = matches.appendWhen(kept, month, matches.monthInRange(month, filter))
	}
	return kept
}

func (matches Matches) Keep(documents []Document, domainId string, filter Filter, readable map[string]bool) []Document {
	kept := []Document{}
	for _, document := range documents {
		kept = matches.appendDocumentWhen(kept, document, matches.admits(document, domainId, filter, readable))
	}
	slices.SortFunc(kept, matches.newestFirst)
	return slices.CompactFunc(kept, matches.sameMessage)
}

func (matches Matches) admits(document Document, domainId string, filter Filter, readable map[string]bool) bool {
	checks := []bool{
		document.DomainId == domainId,
		readable[document.ConversationId],
		!document.Deleted,
		matches.authoredBy(document, filter.Author),
		document.CreatedAt >= filter.From,
		document.CreatedAt <= cmp.Or(filter.To, matches.endOfTime()),
		filter.Cursor.Admits(document.CreatedAt, document.MessageId),
		textindex.Matcher{}.Matches(document.Terms, filter.Terms),
	}
	return !slices.Contains(checks, false)
}

func (Matches) authoredBy(document Document, author string) bool {
	byAddress := slices.Contains([]string{"", document.AuthorAddress}, strings.ToLower(author))
	return slices.Contains([]bool{byAddress, document.AuthorMemberId == author}, true)
}

func (matches Matches) monthInRange(month string, filter Filter) bool {
	newest := cmp.Or(matches.month(filter.Cursor.CreatedAt), matches.month(filter.To), month)
	oldest := cmp.Or(matches.month(filter.From), month)
	return !slices.Contains([]bool{month >= oldest, month <= newest}, false)
}

func (Matches) month(stamp string) string {
	return strings.ReplaceAll(stamp[:min(len(stamp), len("2006-01"))], "-", "")
}

func (Matches) newestFirst(left Document, right Document) int {
	return cmp.Or(strings.Compare(right.CreatedAt, left.CreatedAt), strings.Compare(right.MessageId, left.MessageId))
}

func (Matches) sameMessage(left Document, right Document) bool {
	return left.MessageId == right.MessageId
}

func (Matches) appendWhen(kept []string, month string, wanted bool) []string {
	if wanted {
		return append(kept, month)
	}
	return kept
}

func (Matches) appendDocumentWhen(kept []Document, document Document, wanted bool) []Document {
	if wanted {
		return append(kept, document)
	}
	return kept
}

func (Matches) addText(where map[string]any, field string, value string) {
	if value != "" {
		where[field] = value
	}
}

func (Matches) addFlag(where map[string]any, field string, value *bool) {
	if value != nil {
		where[field] = *value
	}
}

func (Matches) endOfTime() string {
	return "9999"
}
