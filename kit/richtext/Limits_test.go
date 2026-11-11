package richtext

import (
	"strings"
	"testing"
	"unicode/utf8"
)

func TestBlockLimitTruncates(t *testing.T) {
	limits := Limits{}
	result := Converter{}.FromText(strings.Repeat("paragraph\n\n", limits.MaxBlocks()+10))
	if len(result.Blocks) != limits.MaxBlocks() || !result.Truncated {
		t.Fatalf("blocks: %d, truncated: %v", len(result.Blocks), result.Truncated)
	}
}

func TestCharacterLimitTruncates(t *testing.T) {
	limits := Limits{}
	result := Converter{}.FromMarkdown(strings.Repeat("é", limits.MaxCharacters()+5) + "\n\nlater")
	if utf8.RuneCountInString(result.Text) != limits.MaxCharacters() || !result.Truncated || len(result.Blocks) != 1 {
		t.Fatalf("characters: %d, truncated: %v", utf8.RuneCountInString(result.Text), result.Truncated)
	}
}

func TestHtmlLimitsHold(t *testing.T) {
	limits := Limits{}
	result := Converter{}.FromHtml(strings.Repeat("<p>x</p><hr><img alt=a src=https://i.example/a>", limits.MaxBlocks()), nil)
	if len(result.Blocks) != limits.MaxBlocks() || !result.Truncated {
		t.Fatalf("blocks: %d, truncated: %v", len(result.Blocks), result.Truncated)
	}
}

func TestShortInputIsNotTruncated(t *testing.T) {
	result := Converter{}.FromHtml("<p>short</p>", nil)
	if result.Truncated {
		t.Fatal("short input marked truncated")
	}
}

func TestJsonListsAreNeverNull(t *testing.T) {
	empty := encoded(t, Converter{}.FromMarkdown(""))
	blocks := encoded(t, Converter{}.FromMarkdown("---\n![a](https://x.example/a.png)\ntext"))
	if empty != `{"blocks":[],"text":"","truncated":false}` || strings.Contains(blocks, "null") {
		t.Fatalf("json: %s %s", empty, blocks)
	}
}

func TestJsonUsesLowerCamelNames(t *testing.T) {
	result := Converter{}.FromHtml(`<a href="https://a.example">x</a><img src="cid:c" alt="y">`, map[string]string{"c": "att_1"})
	expected := `{"blocks":[{"kind":"paragraph","level":0,"ordered":false,"spans":[{"text":"x","bold":false,"italic":false,"code":false,"strike":false,"href":"https://a.example"}],"cells":[]},` +
		`{"kind":"image","level":0,"ordered":false,"spans":[],"image":{"alt":"y","attachmentId":"att_1"},"cells":[]}],"text":"x\ny","truncated":false}`
	if encoded(t, result) != expected {
		t.Fatalf("json: %s", encoded(t, result))
	}
}

func TestKindWords(t *testing.T) {
	kinds := Kinds{}
	words := []string{kinds.Paragraph(), kinds.Heading(), kinds.Quote(), kinds.ListItem(), kinds.Code(), kinds.Divider(), kinds.Image(), kinds.Row()}
	expectStrings(t, "kinds", words, []string{"paragraph", "heading", "quote", "listItem", "code", "divider", "image", "row"})
}
