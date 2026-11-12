package mailin

import (
	"encoding/json"

	"mesail.dev/packages/richtext"
)

type ContentBudget struct{}

func (ContentBudget) MaxBytes() int {
	return 2 << 20
}

func (budget ContentBudget) Applied(content InboundContent) InboundContent {
	return budget.fitted(budget.capped(content))
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
	content.Blocks = richtext.Converter{}.FromText(content.Text).Blocks
	content.BodyFormat = BodyFormats{}.Text()
	content.Truncated = true
	return content
}

func (ContentBudget) size(content InboundContent) int {
	encoded, _ := json.Marshal(content)
	return len(encoded)
}
