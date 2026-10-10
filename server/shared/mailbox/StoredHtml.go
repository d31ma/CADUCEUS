import (
	"regexp"
	"strings"
)

// StoredHtml strips the active content signatures and templates must not
// carry: script-like blocks and tags, event handlers, javascript: and
// vbscript: links, and (for signatures) scriptable inline styles. It keeps
// the JavaScript sanitizer's regular expressions; their backreferences are
// written out as alternatives.
type StoredHtml struct{}

// Sanitized answers input without active content; styles also removes
// dangerous style attributes.
func (html StoredHtml) Sanitized(input string, styles bool) string {
	space := html.space()
	text := strings.ReplaceAll(input, "\x00", "")
	text = html.withoutBlocks(text)
	text = regexp.MustCompile(`(?i)<(?:script|style|iframe|frame|frameset|object|embed|applet|form|input|button|textarea|select|option|base|meta|link)\b[^>]*/?`+space+`*>`).ReplaceAllString(text, "")
	text = regexp.MustCompile(`(?i)`+space+`+on[a-z0-9_-]+`+space+`*=`+space+`*(?:"[^"]*"|'[^']*'|[^\t\n\x0B\f\r \x{00A0}\x{1680}\x{2000}-\x{200A}\x{2028}\x{2029}\x{202F}\x{205F}\x{3000}\x{FEFF}>]+)`).ReplaceAllString(text, "")
	scheme := `(?:javascript|vbscript):`
	text = regexp.MustCompile(`(?i)`+space+`+(href|src|xlink:href)`+space+`*=`+space+`*(?:"`+space+`*`+scheme+`[\s\S]*?"|'`+space+`*`+scheme+`[\s\S]*?')`).ReplaceAllString(text, ` $1="#"`)
	if styles {
		dangerous := `(?:expression` + space + `*\(|url` + space + `*\(` + space + `*["']?` + space + `*` + scheme + `)`
		text = regexp.MustCompile(`(?i)`+space+`+style`+space+`*=`+space+`*(?:"[^"']*`+dangerous+`[^"']*"|'[^"']*`+dangerous+`[^"']*')`).ReplaceAllString(text, "")
	}
	return text
}

// withoutBlocks removes each <tag ...>...</tag> block of a dangerous tag,
// scanning left to right as the JavaScript /<(tag)\b[^>]*>[\s\S]*?<\/\1\s*>/gi
// replacement did.
func (html StoredHtml) withoutBlocks(text string) string {
	var kept strings.Builder
	position := 0
	for start := 0; start < len(text); start++ {
		if text[start] != '<' {
			continue
		}
		end := html.blockEnd(text, start)
		if end < 0 {
			continue
		}
		kept.WriteString(text[position:start])
		position = end
		start = end - 1
	}
	kept.WriteString(text[position:])
	return kept.String()
}

// blockEnd answers where the block opening at start ends, or -1.
func (html StoredHtml) blockEnd(text string, start int) int {
	for _, tag := range []string{"script", "style", "iframe", "frame", "frameset", "object", "embed", "applet", "form", "template", "noscript"} {
		after := start + 1 + len(tag)
		if after > len(text) || !strings.EqualFold(text[start+1:after], tag) || (after < len(text) && html.wordByte(text[after])) {
			continue
		}
		opened := strings.IndexByte(text[after:], '>')
		if opened < 0 {
			continue
		}
		closing := regexp.MustCompile(`(?i)</` + tag + html.space() + `*>`).FindStringIndex(text[after+opened+1:])
		if closing == nil {
			continue
		}
		return after + opened + 1 + closing[1]
	}
	return -1
}

func (StoredHtml) wordByte(character byte) bool {
	return (character >= 'a' && character <= 'z') || (character >= 'A' && character <= 'Z') || (character >= '0' && character <= '9') || character == '_'
}

// space is JavaScript's \s.
func (StoredHtml) space() string {
	return `[\t\n\x0B\f\r \x{00A0}\x{1680}\x{2000}-\x{200A}\x{2028}\x{2029}\x{202F}\x{205F}\x{3000}\x{FEFF}]`
}
