package richtext

import "testing"

func TestTextSplitsParagraphsOnBlankLines(t *testing.T) {
	result := Converter{}.FromText("Hi Ana,\r\nthanks for the note.\n \n\t\nBest,\rSam  ")
	expectStrings(t, "texts", blockTexts(result), []string{"Hi Ana,\nthanks for the note.", "Best,\nSam"})
	expectStrings(t, "kinds", blockKinds(result), []string{"paragraph", "paragraph"})
}

func TestTextKeepsMarkupAndIndentAsText(t *testing.T) {
	result := Converter{}.FromText("  <b>not bold</b> **not bold** https://a.example")
	spans := result.Blocks[0].Spans
	if len(spans) != 1 || spans[0].Text != "  <b>not bold</b> **not bold** https://a.example" || spans[0].Bold || spans[0].Href != "" {
		t.Fatalf("spans: %s", encoded(t, spans))
	}
}

func TestTextReplacesInvalidUtf8(t *testing.T) {
	result := Converter{}.FromText("bad \xff byte")
	if result.Text != "bad � byte" {
		t.Fatalf("text: %q", result.Text)
	}
}
