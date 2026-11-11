package richtext

import (
	"strings"
	"testing"
)

func TestHtmlNewsletterKeepsTextInReadingOrder(t *testing.T) {
	source := `<!doctype html><html><head><meta charset="utf-8"><title>Weekly</title>
<style>.hero{color:red}</style></head>
<body style="margin:0"><table width="100%" role="presentation"><tr><td align="center">
<table width="600"><tr><td><img src="https://cdn.example/logo.png" alt="Example News" width="120"></td></tr>
<tr><td><h1 style="font-size:24px">This week in gardening</h1>
<p>Spring is here. Read our <a href="https://news.example/spring?utm=1">planting guide</a>.</p></td></tr>
<tr><td><h2>Top tips</h2><ul><li>Water early</li><li>Mulch &amp; feed</li></ul></td></tr>
<tr><td style="font-size:11px">You get this because you signed up. <a href="https://news.example/unsubscribe">Unsubscribe</a></td></tr>
</table></td></tr></table>
<img src="https://track.example/open.gif" width="1" height="1" alt=""></body></html>`
	result := Converter{}.FromHtml(source, nil)
	expectStrings(t, "kinds", blockKinds(result), []string{"image", "heading", "paragraph", "heading", "listItem", "listItem", "paragraph", "image"})
	expectStrings(t, "texts", blockTexts(result), []string{"Example News", "This week in gardening", "Spring is here. Read our planting guide.", "Top tips", "Water early", "Mulch & feed", "You get this because you signed up. Unsubscribe", ""})
	if result.Blocks[2].Spans[1].Href != "https://news.example/spring?utm=1" || result.Blocks[0].Image.RemoteUrl != "https://cdn.example/logo.png" {
		t.Fatalf("links lost: %s", encoded(t, result.Blocks))
	}
}

func TestHtmlReplyKeepsQuotedHistory(t *testing.T) {
	source := `<div dir="ltr">Sounds good, see you Monday.<br><br>Sam</div><br>
<div class="gmail_quote"><div class="gmail_attr">On Fri, Ana wrote:<br></div>
<blockquote class="gmail_quote" style="margin:0 0 0 .8ex"><div>Can we meet Monday?</div>
<blockquote><div>Earlier question</div></blockquote></blockquote></div>`
	result := Converter{}.FromHtml(source, nil)
	expectStrings(t, "kinds", blockKinds(result), []string{"paragraph", "paragraph", "quote", "quote"})
	expectStrings(t, "texts", blockTexts(result), []string{"Sounds good, see you Monday.\n\nSam", "On Fri, Ana wrote:", "Can we meet Monday?", "Earlier question"})
	if result.Blocks[3].Level != 2 {
		t.Fatalf("nested quote level: %d", result.Blocks[3].Level)
	}
}

func TestHtmlDropsScriptsStylesAndForms(t *testing.T) {
	source := `<p>Before</p><script>alert("x")</script><style>p{}</style><form action="/x"><input name="q"><p>inside form</p></form>
<iframe src="https://evil.example">frame text</iframe><object>object text</object><svg><text>svg text</text></svg>
<math><mi>x</mi></math><template><p>template</p></template><noscript>no script</noscript><p>After</p>`
	result := Converter{}.FromHtml(source, nil)
	expectStrings(t, "texts", blockTexts(result), []string{"Before", "After"})
}

func TestHtmlDropsEventAndStyleAttributes(t *testing.T) {
	result := Converter{}.FromHtml(`<p onclick="steal()" style="color:red" class="x" title="hint">Click <span onmouseover="x()">here</span></p>`, nil)
	if result.Text != "Click here" || strings.Contains(encoded(t, result), "steal") || strings.Contains(encoded(t, result), "color") {
		t.Fatalf("attributes leaked: %s", encoded(t, result))
	}
}

func TestHtmlJavascriptLinkBecomesText(t *testing.T) {
	result := Converter{}.FromHtml(`<p><a href="javascript:alert(1)">Open</a> <a href=" JaVaScRiPt:alert(1)">Again</a></p>`, nil)
	expectStrings(t, "texts", blockTexts(result), []string{"Open Again"})
	if result.Blocks[0].Spans[0].Href != "" || len(result.Blocks[0].Spans) != 1 {
		t.Fatalf("href kept: %s", encoded(t, result.Blocks))
	}
}

func TestHtmlDataLinkBecomesText(t *testing.T) {
	result := Converter{}.FromHtml(`<a href="data:text/html;base64,PHNjcmlwdD4=">Report</a> <a href="/relative">Local</a>`, nil)
	if result.Blocks[0].Spans[0].Href != "" || result.Text != "Report Local" {
		t.Fatalf("href kept: %s", encoded(t, result.Blocks))
	}
}

func TestHtmlMailtoAndHttpLinksKeepHref(t *testing.T) {
	result := Converter{}.FromHtml(`<a href="mailto:ana@example.com">Mail</a> <a href="HTTP://old.example">Old</a>`, nil)
	spans := result.Blocks[0].Spans
	if spans[0].Href != "mailto:ana@example.com" || spans[len(spans)-1].Href != "HTTP://old.example" {
		t.Fatalf("hrefs: %s", encoded(t, spans))
	}
}

func TestHtmlNestedLists(t *testing.T) {
	result := Converter{}.FromHtml(`<ol><li>First<ul><li>Inner <b>bold</b></li></ul>back</li><li><p>Second</p></li></ol>`, nil)
	expectStrings(t, "texts", blockTexts(result), []string{"First", "Inner bold", "back", "Second"})
	levels := []int{1, 2, 1, 1}
	ordered := []bool{true, false, true, true}
	for index, block := range result.Blocks {
		if block.Kind != "listItem" || block.Level != levels[index] || block.Ordered != ordered[index] {
			t.Fatalf("block %d: %s", index, encoded(t, block))
		}
	}
}

func TestHtmlListItemOutsideListHasLevelOne(t *testing.T) {
	result := Converter{}.FromHtml(`<li>alone</li>`, nil)
	if result.Blocks[0].Kind != "listItem" || result.Blocks[0].Level != 1 {
		t.Fatalf("block: %s", encoded(t, result.Blocks))
	}
}

func TestHtmlDataTableBecomesRows(t *testing.T) {
	result := Converter{}.FromHtml(`<table><thead><tr><th>Item</th><th>Price</th></tr></thead>
<tbody><tr>
  <td>Tea <b>green</b></td>
  <td>$3<br>each</td>
</tr><tr><td>Cake</td><td><span><img src="cid:x" alt="pic"></span></td></tr></tbody></table><p>Total</p>`, nil)
	expectStrings(t, "kinds", blockKinds(result), []string{"row", "row", "row", "paragraph"})
	expectStrings(t, "cells", result.Blocks[1].Cells, []string{"Tea green", "$3 each"})
	if len(result.Blocks[0].Spans) != 0 || result.Blocks[0].Cells[1] != "Price" {
		t.Fatalf("header row: %s", encoded(t, result.Blocks[0]))
	}
}

func TestHtmlDataCellKeepsNestedStructureAsText(t *testing.T) {
	result := Converter{}.FromHtml(`<table><tr><td>A</td><td><span><table><tr><td>x</td><td>y</td></tr></table><ul><li>z</li></ul><hr></span></td></tr></table>`, nil)
	expectStrings(t, "kinds", blockKinds(result), []string{"row"})
	expectStrings(t, "cells", result.Blocks[0].Cells, []string{"A", "x y z"})
}

func TestHtmlLayoutTableBecomesParagraphs(t *testing.T) {
	result := Converter{}.FromHtml(`<table><tr><td><div>Left column</div></td><td><p>Right column</p></td></tr><tr><td>Single cell</td></tr></table>`, nil)
	expectStrings(t, "kinds", blockKinds(result), []string{"paragraph", "paragraph", "paragraph"})
	expectStrings(t, "texts", blockTexts(result), []string{"Left column", "Right column", "Single cell"})
}

func TestHtmlCidImageBecomesAttachment(t *testing.T) {
	images := map[string]string{"logo@mail.example": "att_7"}
	result := Converter{}.FromHtml(`<img src="CID:logo%40mail.example" alt="Logo">`, images)
	image := result.Blocks[0].Image
	if image.AttachmentId != "att_7" || image.Alt != "Logo" || image.RemoteUrl != "" {
		t.Fatalf("image: %s", encoded(t, image))
	}
}

func TestHtmlUnknownCidImageKeepsAlt(t *testing.T) {
	result := Converter{}.FromHtml(`<img src="cid:missing%zz" alt="Chart">`, map[string]string{"other": "att_1"})
	if encoded(t, result.Blocks[0].Image) != `{"alt":"Chart"}` {
		t.Fatalf("image: %s", encoded(t, result.Blocks[0].Image))
	}
}

func TestHtmlDataImageKeepsOnlyAlt(t *testing.T) {
	result := Converter{}.FromHtml(`<img src="data:image/png;base64,AAAA" alt="Inline"><img src="javascript:x" alt=""><img src="ftp://x/y.png">`, nil)
	expectStrings(t, "kinds", blockKinds(result), []string{"image"})
	if encoded(t, result.Blocks[0].Image) != `{"alt":"Inline"}` {
		t.Fatalf("image: %s", encoded(t, result.Blocks[0].Image))
	}
}

func TestHtmlDecodesEntities(t *testing.T) {
	result := Converter{}.FromHtml(`<p>Fish &amp; chips &lt;b&gt;not bold&lt;/b&gt; &copy; &#8364;5 &nbsp;x</p>`, nil)
	if result.Text != "Fish & chips <b>not bold</b> © €5  x" || result.Blocks[0].Spans[0].Bold {
		t.Fatalf("text: %q", result.Text)
	}
}

func TestHtmlCollapsesWhitespaceOutsidePre(t *testing.T) {
	result := Converter{}.FromHtml("<p>\n   Hello\t\t<b> big </b>\n\n world   </p><pre>  keep\n\n    this  </pre>", nil)
	expectStrings(t, "texts", blockTexts(result), []string{"Hello big world", "  keep\n\n    this  "})
	expectStrings(t, "kinds", blockKinds(result), []string{"paragraph", "code"})
}

func TestHtmlInlineStyles(t *testing.T) {
	result := Converter{}.FromHtml(`<p><strong>B</strong><em>I</em><del>S</del><code>C</code><b><i>BI</i></b></p>`, nil)
	spans := result.Blocks[0].Spans
	if len(spans) != 5 || !spans[0].Bold || !spans[1].Italic || !spans[2].Strike || !spans[3].Code || !spans[4].Bold || !spans[4].Italic {
		t.Fatalf("spans: %s", encoded(t, spans))
	}
}

func TestHtmlHeadingLevels(t *testing.T) {
	result := Converter{}.FromHtml(`<h1>One</h1><h3>Three</h3><h6>Six</h6>after`, nil)
	for index, level := range []int{1, 3, 6} {
		if result.Blocks[index].Kind != "heading" || result.Blocks[index].Level != level {
			t.Fatalf("heading %d: %s", index, encoded(t, result.Blocks[index]))
		}
	}
	if result.Blocks[3].Kind != "paragraph" {
		t.Fatalf("after heading: %s", encoded(t, result.Blocks[3]))
	}
}

func TestHtmlDropsComments(t *testing.T) {
	result := Converter{}.FromHtml(`<p>A<!-- secret <b>note</b> -->B</p><!--[if mso]><p>Outlook</p><![endif]-->`, nil)
	if result.Text != "AB" {
		t.Fatalf("text: %q", result.Text)
	}
}

func TestHtmlDividerSplitsBlocks(t *testing.T) {
	result := Converter{}.FromHtml(`<p>Above<hr>Below</p>`, nil)
	expectStrings(t, "kinds", blockKinds(result), []string{"paragraph", "divider", "paragraph"})
}

func TestHtmlDeepNestingIsCapped(t *testing.T) {
	depth := 100
	source := strings.Repeat("<blockquote><div><b>", depth) + "deep" + strings.Repeat("</b></div></blockquote>", depth)
	result := Converter{}.FromHtml(source, nil)
	if len(result.Blocks) != 1 || result.Blocks[0].Level != (Limits{}).MaxDepth() || !result.Blocks[0].Spans[0].Bold {
		t.Fatalf("blocks: %s", encoded(t, result.Blocks))
	}
}

func TestHtmlBeyondParserDepthFallsBackToFlatText(t *testing.T) {
	depth := 5000
	source := "<p>Intro</p><script>bad()</script>" + strings.Repeat("<div><svg>hidden</svg>", depth) + "deep<br>text<!-- note --><font>" + strings.Repeat("</div>", depth) + "<p>End <img src=x /></p>"
	result := Converter{}.FromHtml(source, nil)
	expectStrings(t, "texts", blockTexts(result), []string{"Intro", "deep\ntext", "End"})
}
