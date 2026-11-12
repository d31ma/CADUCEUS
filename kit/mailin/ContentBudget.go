package mailin

import (
	"encoding/json"
	"strings"

	"mesail.dev/packages/richtext"
)

type ContentBudget struct{}

func (ContentBudget) MaxBytes() int {
	return 256 << 10
}

func (ContentBudget) PlainTextBytes() int {
	return 64 << 10
}

func (ContentBudget) LastResortTextBytes() int {
	return 16 << 10
}

func (budget ContentBudget) Applied(content InboundContent) InboundContent {
	return budget.lastResort(budget.fitted(budget.capped(content)))
}

func (ContentBudget) capped(content InboundContent) InboundContent {
	characters := []rune(content.Text)
	limit := richtext.Limits{}.MaxCharacters()
	if len(characters) <= limit {
		return content
	}
	content.Text = string(characters[:limit])
	content.Truncated = true
	return content
}

func (budget ContentBudget) fitted(content InboundContent) InboundContent {
	if budget.size(content) <= budget.MaxBytes() {
		return content
	}
	content.Text = budget.cut(content.Text, budget.PlainTextBytes())
	content.Blocks = richtext.Converter{}.FromText(content.Text).Blocks
	content.BodyFormat = BodyFormats{}.Text()
	content.Truncated = true
	return content
}

func (budget ContentBudget) lastResort(content InboundContent) InboundContent {
	if budget.size(content) <= budget.MaxBytes() {
		return content
	}
	unreadable := ContentBuilder{}.Unreadable()
	unreadable.Text = budget.cut(content.Text, budget.LastResortTextBytes())
	return unreadable
}

func (ContentBudget) cut(text string, limit int) string {
	return strings.ToValidUTF8(text[:min(len(text), limit)], "")
}

func (ContentBudget) size(content InboundContent) int {
	encoded, _ := json.Marshal(content)
	return len(encoded)
}
