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
	content := InboundContent{Text: strings.Repeat("a", limit+5), BodyFormat: "text", Blocks: []richtext.Block{}, Attachments: []InboundAttachment{}}
	cut := ContentBudget{}.capped(content)
	check(t, len(cut.Text) == limit && cut.Truncated, "want the text cut at the limit")
}

func TestContentOverTheBudgetKeepsBlocksMadeFromAShorterText(t *testing.T) {
	content := InboundContent{Text: "Hello\n" + strings.Repeat("é", 100000), BodyFormat: "email", Blocks: manyBlocks(200, strings.Repeat("x", 1200)), Attachments: []InboundAttachment{}}
	fitted := ContentBudget{}.Applied(content)
	check(t, fitted.BodyFormat == "text" && fitted.Truncated, "want plain text blocks")
	check(t, len(fitted.Text) <= ContentBudget{}.PlainTextBytes() && utf8.ValidString(fitted.Text), "want a shorter valid text")
	check(t, ContentBudget{}.size(fitted) <= ContentBudget{}.MaxBytes(), "still over the budget")
	check(t, strings.HasPrefix(fitted.Blocks[0].Spans[0].Text, "Hello"), "wrong blocks from the text")
}

func TestContentStillOverTheBudgetBecomesTheUnreadableForm(t *testing.T) {
	attachments := []InboundAttachment{}
	for range 256 {
		attachments = append(attachments, InboundAttachment{FileName: strings.Repeat("n", 2000), BlobKey: "example.com/a"})
	}
	content := InboundContent{Text: strings.Repeat("a", 70000), BodyFormat: "email", Blocks: manyBlocks(1, "a"), Attachments: attachments}
	last := ContentBudget{}.Applied(content)
	check(t, last.Truncated && len(last.Blocks) == 0 && len(last.Attachments) == 0, "want empty blocks and attachments")
	check(t, len(last.Text) == ContentBudget{}.LastResortTextBytes(), "want the text cut to the last resort size")
	check(t, ContentBudget{}.size(last) <= ContentBudget{}.MaxBytes(), "still over the budget")
}
