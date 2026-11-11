package richtext

import (
	"encoding/json"
	"slices"
	"strings"
	"testing"
	"unicode/utf8"

	"golang.org/x/net/html"
)

func FuzzHtml(f *testing.F) {
	for _, seed := range htmlSeeds() {
		f.Add(seed)
	}
	f.Fuzz(func(t *testing.T, source string) {
		result := Converter{}.FromHtml(source, map[string]string{"logo": "att_1"})
		checkResult(t, result)
		checkNoMarkup(t, result, visibleHtmlText(source))
	})
}

func FuzzMarkdown(f *testing.F) {
	for _, seed := range markdownSeeds() {
		f.Add(seed)
	}
	f.Fuzz(func(t *testing.T, source string) {
		result := Converter{}.FromMarkdown(source)
		checkResult(t, result)
		checkNoMarkup(t, result, source)
	})
}

func htmlSeeds() []string {
	return []string{
		`<p>Hello <b>world</b> &lt;b&gt;</p><script>alert(1)</script>`,
		`<a href="javascript:x" onclick="y">a</a><a href="https://a">b</a><img src="cid:logo" alt="<i>">`,
		`<table><tr><td>a</td><td>b<table><tr><td>c</td><td>d</td></tr></table></td></tr></table>`,
		`<ul><li>a<ol><li>b</li></ol></li></ul><blockquote><blockquote>q</blockquote></blockquote><pre> x
 y</pre>`,
		`<svg><style><b>x</b></style></svg><math><mtext><b>y</b></mtext></math><textarea><b></textarea><xmp><i></xmp>`,
		`<image alt="a<b" src="data:x"><title><t></title><plaintext><p>tail`,
		strings.Repeat("<div>", 600) + "deep &lt;x" + strings.Repeat("</div>", 600),
	}
}

func markdownSeeds() []string {
	return []string{
		"# Title\n\n**bold** _it_ `code` ~~s~~ [a](https://x) <https://y> ![i](https://z/p.png)",
		"> q\n>> qq\n- a\n  - b\n1. c\n\n```\n<b>\n```\n\n    code\n\n---",
		"<script>x</script> a < b \\* [x](javascript:y) <javascript:z> ![a](data:q)",
		"***a*** __b__ * c * _d_e_ `` x ` y `` [[a](b)](c) ![![i](j)](k)",
	}
}

func visibleHtmlText(source string) string {
	var text strings.Builder
	tokenizer := html.NewTokenizer(strings.NewReader(strings.ToValidUTF8(source, "�")))
	for tokenizer.Next() != html.ErrorToken {
		token := tokenizer.Token()
		text.WriteString(tokenText(token))
	}
	return text.String()
}

func tokenText(token html.Token) string {
	if token.Type == html.TextToken {
		return token.Data
	}
	alts := ""
	for _, attribute := range token.Attr {
		if attribute.Key == "alt" && slices.Contains([]string{"img", "image"}, token.Data) {
			alts += attribute.Val
		}
	}
	return alts
}

func outputStrings(result Result) []string {
	texts := []string{result.Text}
	for _, block := range result.Blocks {
		texts = append(texts, block.Cells...)
		for _, span := range block.Spans {
			texts = append(texts, span.Text, span.Href)
		}
		if block.Image != nil {
			texts = append(texts, block.Image.Alt, block.Image.AttachmentId, block.Image.RemoteUrl)
		}
	}
	return texts
}

func checkNoMarkup(t *testing.T, result Result, visible string) {
	t.Helper()
	allowed := strings.Count(visible, "<")
	found := 0
	for _, block := range result.Blocks {
		found += strings.Count(block.plainText(), "<")
	}
	if found > allowed {
		t.Fatalf("markup passed through: %d '<' in output, %d in visible input text: %s", found, allowed, encoded(t, result.Blocks))
	}
}

func checkResult(t *testing.T, result Result) {
	t.Helper()
	limits := Limits{}
	characters := 0
	for _, block := range result.Blocks {
		characters += utf8.RuneCountInString(block.spanText() + strings.Join(block.Cells, "") + block.altText())
		checkBlock(t, block)
	}
	if len(result.Blocks) > limits.MaxBlocks() || characters > limits.MaxCharacters() {
		t.Fatalf("limits broken: %d blocks, %d characters", len(result.Blocks), characters)
	}
	for _, text := range outputStrings(result) {
		if !utf8.ValidString(text) {
			t.Fatalf("invalid UTF-8: %q", text)
		}
	}
	if _, err := json.Marshal(result); err != nil || result.Blocks == nil {
		t.Fatalf("json: %v", err)
	}
}

func checkBlock(t *testing.T, block Block) {
	t.Helper()
	kinds := Kinds{}
	known := []string{kinds.Paragraph(), kinds.Heading(), kinds.Quote(), kinds.ListItem(), kinds.Code(), kinds.Divider(), kinds.Image(), kinds.Row()}
	if !slices.Contains(known, block.Kind) || block.Spans == nil || block.Cells == nil || block.Level > (Limits{}).MaxDepth() {
		t.Fatalf("bad block: %s", encoded(t, block))
	}
	for _, span := range block.Spans {
		if span.Href != "" && !slices.Contains([]string{"http", "https", "mailto"}, strings.ToLower(strings.SplitN(span.Href, ":", 2)[0])) {
			t.Fatalf("href scheme not allowed: %q", span.Href)
		}
	}
	if block.Image != nil && block.Image.RemoteUrl != "" && !slices.Contains([]string{"http", "https"}, strings.ToLower(strings.SplitN(block.Image.RemoteUrl, ":", 2)[0])) {
		t.Fatalf("image scheme not allowed: %q", block.Image.RemoteUrl)
	}
}
