package mailin

import (
	"strings"
	"testing"
	"unicode/utf8"

	"mesail.dev/packages/richtext"
)

func manyBlocks(count int, text string) []richtext.Block {
	blocks := []richtext.Block{}
	for range count {
		blocks = append(blocks, richtext.Block{Kind: "paragraph", Spans: []richtext.Span{{Text: text}}, Cells: []string{}})
	}
	return blocks
}

func TestContentUnderTheBudgetIsKept(t *testing.T) {
	content := InboundContent{Text: "Hi", BodyFormat: "email", Blocks: manyBlocks(1, "Hi"), Attachments: []InboundAttachment{}}
	kept := ContentBudget{}.Applied(content)
	check(t, kept.BodyFormat == "email" && !kept.Truncated && len(kept.Blocks) == 1, "the content changed")
}

func TestALongTextIsCutAtTheCharacterLimit(t *testing.T) {
	limit := richtext.Limits{}.MaxCharacters()
	content := InboundContent{Text: strings.Repeat("é", limit+5), BodyFormat: "text", Blocks: manyBlocks(1, "é"), Attachments: []InboundAttachment{}}
	cut := ContentBudget{}.Applied(content)
	check(t, utf8.RuneCountInString(cut.Text) == limit && cut.Truncated, "want the text cut at the limit")
}

func TestContentOverTheBudgetKeepsBlocksMadeFromItsText(t *testing.T) {
	content := InboundContent{Text: "Hello\nthere", BodyFormat: "email", Blocks: manyBlocks(2000, strings.Repeat("x", 1200)), Attachments: []InboundAttachment{}}
	fitted := ContentBudget{}.Applied(content)
	check(t, fitted.BodyFormat == "text" && fitted.Truncated, "want plain text blocks")
	check(t, ContentBudget{}.size(fitted) <= ContentBudget{}.MaxBytes(), "still over the budget")
	check(t, len(fitted.Blocks) > 0 && strings.HasPrefix(fitted.Blocks[0].Spans[0].Text, "Hello"), "wrong blocks from the text")
}
