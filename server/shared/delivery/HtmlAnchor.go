// HtmlAnchor is one <a href> element of an HTML body: its decoded href and
// text, whether it sits inside quoted content, and the byte span of its href
// attribute in the source.
type HtmlAnchor struct {
	Href      string
	Text      string
	Quoted    bool
	HrefStart int
	HrefEnd   int
}
