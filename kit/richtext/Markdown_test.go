package richtext

import (
	"strings"
	"testing"
)

func TestMarkdownParagraphsKeepLineBreaks(t *testing.T) {
	result := Converter{}.FromMarkdown("First line\r\nsecond line\n\n\nNext paragraph")
	expectStrings(t, "texts", blockTexts(result), []string{"First line\nsecond line", "Next paragraph"})
	expectStrings(t, "kinds", blockKinds(result), []string{"paragraph", "paragraph"})
}

func TestMarkdownAtxHeadings(t *testing.T) {
	result := Converter{}.FromMarkdown("# One\n### Three ###\n###### Six\n####### Seven\n#hashtag\n## C#")
	expectStrings(t, "kinds", blockKinds(result), []string{"heading", "heading", "heading", "paragraph", "heading"})
	expectStrings(t, "texts", blockTexts(result), []string{"One", "Three", "Six", "####### Seven\n#hashtag", "C#"})
	if result.Blocks[1].Level != 3 || result.Blocks[2].Level != 6 {
		t.Fatalf("levels: %s", encoded(t, result.Blocks))
	}
}

func TestMarkdownNestedQuotes(t *testing.T) {
	result := Converter{}.FromMarkdown("> outer\n> still outer\n>> inner\n> > also inner\n\nplain")
	expectStrings(t, "texts", blockTexts(result), []string{"outer\nstill outer", "inner\nalso inner", "plain"})
	if result.Blocks[0].Kind != "quote" || result.Blocks[0].Level != 1 || result.Blocks[1].Level != 2 || result.Blocks[2].Kind != "paragraph" {
		t.Fatalf("quotes: %s", encoded(t, result.Blocks))
	}
}

func TestMarkdownNestedLists(t *testing.T) {
	result := Converter{}.FromMarkdown("- apple\n  * green\n    + small\n- pear\ncontinued\n\n1. first\n2) second\n   - mixed")
	expectStrings(t, "texts", blockTexts(result), []string{"apple", "green", "small", "pear\ncontinued", "first", "second", "mixed"})
	levels := []int{1, 2, 3, 1, 1, 1, 2}
	ordered := []bool{false, false, false, false, true, true, false}
	for index, block := range result.Blocks {
		if block.Kind != "listItem" || block.Level != levels[index] || block.Ordered != ordered[index] {
			t.Fatalf("block %d: %s", index, encoded(t, block))
		}
	}
}

func TestMarkdownFencedCode(t *testing.T) {
	result := Converter{}.FromMarkdown("```go\nfunc main() {\n\n    print(\"<b>**x**</b>\")\n}\n```\n~~~\ntilde\n~~~~\nafter")
	expectStrings(t, "kinds", blockKinds(result), []string{"code", "code", "paragraph"})
	expectStrings(t, "texts", blockTexts(result), []string{"func main() {\n\n    print(\"<b>**x**</b>\")\n}", "tilde", "after"})
}

func TestMarkdownUnclosedFenceRunsToEnd(t *testing.T) {
	result := Converter{}.FromMarkdown("text\n```\ncode\n# not heading")
	expectStrings(t, "kinds", blockKinds(result), []string{"paragraph", "code"})
	expectStrings(t, "texts", blockTexts(result), []string{"text", "code\n# not heading"})
}

func TestMarkdownFenceInsideQuote(t *testing.T) {
	result := Converter{}.FromMarkdown("> ```\n> > kept\n> ```\n> after")
	expectStrings(t, "kinds", blockKinds(result), []string{"code", "quote"})
	expectStrings(t, "texts", blockTexts(result), []string{"> kept", "after"})
}

func TestMarkdownInlineBackticksAreNotFence(t *testing.T) {
	result := Converter{}.FromMarkdown("```inline```")
	if result.Blocks[0].Kind != "paragraph" || !result.Blocks[0].Spans[0].Code || result.Text != "inline" {
		t.Fatalf("blocks: %s", encoded(t, result.Blocks))
	}
}

func TestMarkdownIndentedCode(t *testing.T) {
	result := Converter{}.FromMarkdown("Intro\n\n    line one\n\n\tline two\n\nOutro")
	expectStrings(t, "kinds", blockKinds(result), []string{"paragraph", "code", "paragraph"})
	expectStrings(t, "texts", blockTexts(result), []string{"Intro", "line one\n\nline two", "Outro"})
}

func TestMarkdownDivider(t *testing.T) {
	result := Converter{}.FromMarkdown("above\n\n---\n* * *\n___\n--\nbelow")
	expectStrings(t, "kinds", blockKinds(result), []string{"paragraph", "divider", "divider", "divider", "paragraph"})
}

func TestMarkdownBoldItalicStrike(t *testing.T) {
	result := Converter{}.FromMarkdown("**b** __b2__ *i* _i2_ ~~s~~ ***bi***")
	spans := result.Blocks[0].Spans
	styles := []string{}
	for _, span := range spans {
		styles = append(styles, span.Text+":"+markers(span))
	}
	expectStrings(t, "styles", styles, []string{"b:B", " :", "b2:B", " :", "i:I", " :", "i2:I", " :", "s:S", " :", "bi:BI"})
}

func markers(span Span) string {
	text := ""
	for _, flag := range []struct {
		on     bool
		letter string
	}{{span.Bold, "B"}, {span.Italic, "I"}, {span.Strike, "S"}, {span.Code, "C"}} {
		if flag.on {
			text += flag.letter
		}
	}
	return text
}

func TestMarkdownUnmatchedDelimitersStayText(t *testing.T) {
	result := Converter{}.FromMarkdown("2 * 3 = 6, snake_case_name, a ~ b, **open, [no link, ![no image, `tick, <notag")
	if result.Text != "2 * 3 = 6, snake_case_name, a ~ b, **open, [no link, ![no image, `tick, <notag" || len(result.Blocks[0].Spans) != 1 {
		t.Fatalf("blocks: %s", encoded(t, result.Blocks))
	}
}

func TestMarkdownCodeSpan(t *testing.T) {
	result := Converter{}.FromMarkdown("Use `` a`b `` and `**raw**`")
	spans := result.Blocks[0].Spans
	if spans[1].Text != "a`b" || !spans[1].Code || spans[3].Text != "**raw**" || !spans[3].Code {
		t.Fatalf("spans: %s", encoded(t, spans))
	}
}

func TestMarkdownLinks(t *testing.T) {
	result := Converter{}.FromMarkdown("[site](https://a.example \"title\") [**mail**](<mailto:x@example.com>) <https://auto.example/x> [bad](javascript:alert(1)) <javascript:x>")
	spans := result.Blocks[0].Spans
	if spans[0].Href != "https://a.example" || spans[2].Href != "mailto:x@example.com" || !spans[2].Bold || spans[4].Href != "https://auto.example/x" {
		t.Fatalf("spans: %s", encoded(t, spans))
	}
	if result.Text != "site mail https://auto.example/x bad) <javascript:x>" {
		t.Fatalf("text: %q", result.Text)
	}
}

func TestMarkdownImages(t *testing.T) {
	result := Converter{}.FromMarkdown("Look ![cat](https://img.example/cat.png) here ![local](data:image/png;base64,AA) ![](javascript:x)")
	expectStrings(t, "kinds", blockKinds(result), []string{"paragraph", "image", "paragraph", "image"})
	if result.Blocks[1].Image.RemoteUrl != "https://img.example/cat.png" || encoded(t, result.Blocks[3].Image) != `{"alt":"local"}` {
		t.Fatalf("images: %s", encoded(t, result.Blocks))
	}
}

func TestMarkdownRawHtmlStaysText(t *testing.T) {
	result := Converter{}.FromMarkdown("<script>alert(1)</script> <b onclick=\"x\">bold</b>")
	if result.Text != "<script>alert(1)</script> <b onclick=\"x\">bold</b>" || result.Blocks[0].Spans[0].Bold {
		t.Fatalf("blocks: %s", encoded(t, result.Blocks))
	}
}

func TestMarkdownBackslashEscapes(t *testing.T) {
	result := Converter{}.FromMarkdown("\\*not italic\\* \\[x\\] c:\\path \\")
	if result.Text != "*not italic* [x] c:\\path \\" || len(result.Blocks[0].Spans) != 1 {
		t.Fatalf("blocks: %s", encoded(t, result.Blocks))
	}
}

func TestMarkdownDeepNestingIsCapped(t *testing.T) {
	result := Converter{}.FromMarkdown(strings.Repeat(">", 1000) + " deep")
	if result.Blocks[0].Level != (Limits{}).MaxDepth() {
		t.Fatalf("level: %d", result.Blocks[0].Level)
	}
}
