package adapters

import (
	"regexp"
	"strings"
	"unicode/utf8"

	"mesail.dev/packages/core"
)

type PlainRenderer struct{}

func (renderer PlainRenderer) Markdown(body string) (core.RichText, error) {
	paragraphs := renderer.paragraphs(body)
	kept := renderer.cut(paragraphs[:min(len(paragraphs), renderer.maxBlocks())], renderer.maxCharacters())
	truncated := renderer.characters(kept) < renderer.characters(paragraphs)
	return core.RichText{Blocks: renderer.blocks(kept), Truncated: truncated}, nil
}

func (renderer PlainRenderer) paragraphs(body string) []string {
	normalized := strings.ReplaceAll(body, "\r\n", "\n")
	found := []string{}
	for _, part := range renderer.blankLines().Split(normalized, -1) {
		found = append(found, renderer.nonEmpty(strings.TrimSpace(part))...)
	}
	return found
}

func (PlainRenderer) nonEmpty(text string) []string {
	if text == "" {
		return []string{}
	}
	return []string{text}
}

func (renderer PlainRenderer) cut(paragraphs []string, budget int) []string {
	kept := []string{}
	for _, paragraph := range paragraphs {
		piece := renderer.prefix(paragraph, budget)
		kept = append(kept, renderer.nonEmpty(piece)...)
		budget -= utf8.RuneCountInString(piece)
	}
	return kept
}

func (PlainRenderer) prefix(text string, budget int) string {
	runes := []rune(text)
	if len(runes) <= budget {
		return text
	}
	return string(runes[:max(budget, 0)])
}

func (PlainRenderer) characters(paragraphs []string) int {
	total := 0
	for _, paragraph := range paragraphs {
		total += utf8.RuneCountInString(paragraph)
	}
	return total
}

func (renderer PlainRenderer) blocks(paragraphs []string) []core.Block {
	blocks := []core.Block{}
	for _, paragraph := range paragraphs {
		blocks = append(blocks, core.Block{Kind: renderer.paragraphKind(), Spans: []core.Span{{Text: paragraph}}, Cells: []string{}})
	}
	return blocks
}

func (PlainRenderer) blankLines() *regexp.Regexp {
	return regexp.MustCompile(`\n[ \t]*\n`)
}

func (PlainRenderer) paragraphKind() string {
	return "paragraph"
}

func (PlainRenderer) maxBlocks() int {
	return 2000
}

func (PlainRenderer) maxCharacters() int {
	return 200000
}
