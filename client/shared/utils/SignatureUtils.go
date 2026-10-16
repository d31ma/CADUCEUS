import (
	"regexp"
	"strings"
)

// SignatureUtils is the Go form of sanitizeSignatureHTML in
// shared/scripts/composer-signatures.js. Go regular expressions have no
// back-references, so each paired tag and quote gets its own expression.
type SignatureUtils struct{}

func (utils SignatureUtils) Sanitize(input string) string {
	text := utils.blocks(strings.ReplaceAll(input, "\x00", ""))
	text = regexp.MustCompile(`(?i)<(?:script|style|iframe|frame|frameset|object|embed|applet|form|input|button|textarea|select|option|base|meta|link)\b[^>]*/?\s*>`).ReplaceAllString(text, "")
	text = regexp.MustCompile(`(?i)\s+on[a-z0-9_-]+\s*=\s*(?:"[^"]*"|'[^']*'|[^\s>]+)`).ReplaceAllString(text, "")
	text = regexp.MustCompile(`(?i)\s+(href|src|xlink:href)\s*=\s*"\s*(?:javascript|vbscript):[\s\S]*?"`).ReplaceAllString(text, ` ${1}="#"`)
	text = regexp.MustCompile(`(?i)\s+(href|src|xlink:href)\s*=\s*'\s*(?:javascript|vbscript):[\s\S]*?'`).ReplaceAllString(text, ` ${1}="#"`)
	text = regexp.MustCompile(`(?i)\s+style\s*=\s*"[^"']*(?:expression\s*\(|url\s*\(\s*["']?\s*(?:javascript|vbscript):)[^"']*"`).ReplaceAllString(text, "")
	return regexp.MustCompile(`(?i)\s+style\s*=\s*'[^"']*(?:expression\s*\(|url\s*\(\s*["']?\s*(?:javascript|vbscript):)[^"']*'`).ReplaceAllString(text, "")
}

func (SignatureUtils) blocks(text string) string {
	for _, tag := range []string{"script", "style", "iframe", "frame", "frameset", "object", "embed", "applet", "form", "template", "noscript"} {
		text = regexp.MustCompile(`(?i)<`+tag+`\b[^>]*>[\s\S]*?</`+tag+`\s*>`).ReplaceAllString(text, "")
	}
	return text
}
