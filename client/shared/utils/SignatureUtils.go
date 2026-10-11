import (
	"regexp"
	"slices"
	"strings"
)

// SignatureUtils is the Go form of sanitizeSignatureHTML in
// shared/scripts/composer-signatures.js. Go regular expressions have no
// back-references, so each paired tag and quote gets its own expression.
// The composer also picks the default signature of a From address and finds
// the signature a body holds.
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

// SanitizeAll removes executable markup from every signature that the server
// answered, like sanitizeSignatureHTML in composer-signatures.js.
func (SignatureUtils) SanitizeAll(signatures []MailSignature) []MailSignature {
	return SignatureUtils{}.sanitized(signatures)
}

// DefaultFor finds the signature whose defaultFor list holds the address.
func (SignatureUtils) DefaultFor(signatures []MailSignature, address string) MailSignature {
	return SignatureUtils{}.firstDefault(signatures, strings.ToLower(strings.TrimSpace(address)))
}

// CurrentID is the id of the signature in the editable part of a body, like
// currentSignatureId in composer-signatures.js.
func (SignatureUtils) CurrentID(body string) string {
	quote := regexp.MustCompile(`(?i)<(?:blockquote\b|[^>]+class\s*=\s*["'][^"']*(?:gmail_quote|quoted-message)[^"']*["'])`).FindStringIndex(body)
	editable := body[:append(quote, len(body))[0]]
	found := append(regexp.MustCompile(`(?i)<signature\b[^>]*\bid\s*=\s*["']([^"']+)["']`).FindStringSubmatch(editable), "", "")
	return TextUtils{}.FirstOf(found[1], SignatureUtils{}.divID(editable))
}

func (SignatureUtils) divID(editable string) string {
	tag := regexp.MustCompile(`(?i)<div\b[^>]*caduceus-signature[^>]*>`).FindString(editable)
	return append(regexp.MustCompile(`(?i)\bdata-signature-id\s*=\s*["']([^"']+)["']`).FindStringSubmatch(tag), "", "")[1]
}

func (SignatureUtils) sanitized(signatures []MailSignature) []MailSignature {
	cleaned := []MailSignature{}
	for _, signature := range signatures {
		signature.Text = SignatureUtils{}.Sanitize(signature.Text)
		signature.DefaultFor = append([]string{}, signature.DefaultFor...)
		cleaned = append(cleaned, signature)
	}
	return cleaned
}

func (SignatureUtils) firstDefault(signatures []MailSignature, address string) MailSignature {
	matches := []MailSignature{}
	for _, signature := range signatures {
		matches = append(matches, SignatureUtils{}.whenDefaultFor(signature, address)...)
	}
	return append(matches, MailSignature{})[0]
}

func (SignatureUtils) whenDefaultFor(signature MailSignature, address string) []MailSignature {
	if slices.Contains(RecipientUtils{}.Lowered(signature.DefaultFor), address) {
		return []MailSignature{signature}
	}
	return nil
}
