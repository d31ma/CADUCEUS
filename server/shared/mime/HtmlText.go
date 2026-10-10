import (
	"regexp"
	"strings"
)

// HtmlText recognises HTML bodies and reduces HTML to the plain text the
// server indexes: styles, scripts and tags removed, the common entities
// decoded and white space collapsed.
type HtmlText struct{}

// LooksLikeHtml reports whether value holds an element with its closing tag
// or a void element such as <br> or <img>.
func (HtmlText) LooksLikeHtml(value string) bool {
	if regexp.MustCompile(`(?i)<(br|hr|img|meta|link|input)\b[^>]*>`).MatchString(value) {
		return true
	}
	lower := strings.ToLower(value)
	for _, match := range regexp.MustCompile(`(?i)<([a-z][\w:-]*)(\s[^>]*)?>`).FindAllStringSubmatchIndex(value, -1) {
		name := strings.ToLower(value[match[2]:match[3]])
		if strings.Contains(lower[match[1]:], "</"+name+">") {
			return true
		}
	}
	return false
}

// Stripped answers html as collapsed plain text.
func (HtmlText) Stripped(html string) string {
	text := regexp.MustCompile(`(?is)<style\b.*?</style>`).ReplaceAllString(html, " ")
	text = regexp.MustCompile(`(?is)<script\b.*?</script>`).ReplaceAllString(text, " ")
	text = regexp.MustCompile(`<[^>]+>`).ReplaceAllString(text, " ")
	for _, entity := range [][2]string{{"(?i)&nbsp;", " "}, {"(?i)&amp;", "&"}, {"(?i)&lt;", "<"}, {"(?i)&gt;", ">"}, {"(?i)&quot;", `"`}, {"&#39;", "'"}} {
		text = regexp.MustCompile(entity[0]).ReplaceAllLiteralString(text, entity[1])
	}
	return TextValue{}.Trim(TextValue{}.Collapsed(text))
}
