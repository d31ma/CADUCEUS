import (
	"regexp"
	"slices"
	"strings"
)

// SignatureUtils holds the composer-signatures rules that Go needs:
// sanitizing a stored signature, choosing the default signature of a From
// address and finding the signature a body holds. Inserting a signature
// before quoted mail stays in client/shared/scripts/composer-signatures.js,
// next to the live editor.
type SignatureUtils struct{}

// SanitizeAll removes executable markup from every signature that the server
// answered, like sanitizeSignatureHTML in composer-signatures.js.
func (SignatureUtils) SanitizeAll(signatures []ComposeSignature) []ComposeSignature {
	return SignatureUtils{}.sanitized(signatures)
}

// DefaultFor finds the signature whose defaultFor list holds the address.
func (SignatureUtils) DefaultFor(signatures []ComposeSignature, address string) ComposeSignature {
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

func (SignatureUtils) sanitize(text string) string {
	cleaned := strings.ReplaceAll(text, string(rune(0)), "")
	cleaned = SignatureUtils{}.withoutBlocks(cleaned)
	cleaned = SignatureUtils{}.unsafeSingle().ReplaceAllString(cleaned, "")
	cleaned = SignatureUtils{}.eventHandlers().ReplaceAllString(cleaned, "")
	return SignatureUtils{}.withoutScriptAttributes(cleaned)
}

func (SignatureUtils) withoutScriptAttributes(text string) string {
	for _, quote := range (SignatureUtils{}).quotes() {
		text = SignatureUtils{}.scriptAddresses(quote).ReplaceAllString(text, ` ${1}="#"`)
		text = SignatureUtils{}.scriptStyles(quote).ReplaceAllString(text, "")
	}
	return text
}

func (SignatureUtils) sanitized(signatures []ComposeSignature) []ComposeSignature {
	cleaned := []ComposeSignature{}
	for _, signature := range signatures {
		signature.Text = SignatureUtils{}.sanitize(signature.Text)
		signature.DefaultFor = append([]string{}, signature.DefaultFor...)
		cleaned = append(cleaned, signature)
	}
	return cleaned
}

func (SignatureUtils) firstDefault(signatures []ComposeSignature, address string) ComposeSignature {
	matches := []ComposeSignature{}
	for _, signature := range signatures {
		matches = append(matches, SignatureUtils{}.whenDefaultFor(signature, address)...)
	}
	return append(matches, ComposeSignature{})[0]
}

func (SignatureUtils) whenDefaultFor(signature ComposeSignature, address string) []ComposeSignature {
	if slices.Contains(RecipientUtils{}.Lowered(signature.DefaultFor), address) {
		return []ComposeSignature{signature}
	}
	return nil
}

func (SignatureUtils) withoutBlocks(text string) string {
	for _, tag := range (SignatureUtils{}).blockTags() {
		text = regexp.MustCompile(`(?i)<`+tag+`\b[^>]*>[\s\S]*?</`+tag+`\s*>`).ReplaceAllString(text, "")
	}
	return text
}

func (SignatureUtils) quotes() []string {
	return []string{`"`, `'`}
}

func (SignatureUtils) blockTags() []string {
	return []string{"script", "style", "iframe", "frame", "frameset", "object", "embed", "applet", "form", "template", "noscript"}
}

func (SignatureUtils) unsafeSingle() *regexp.Regexp {
	return regexp.MustCompile(`(?i)<(?:script|style|iframe|frame|frameset|object|embed|applet|form|input|button|textarea|select|option|base|meta|link)\b[^>]*/?\s*>`)
}

func (SignatureUtils) eventHandlers() *regexp.Regexp {
	return regexp.MustCompile(`(?i)\s+on[a-z0-9_-]+\s*=\s*(?:"[^"]*"|'[^']*'|[^\s>]+)`)
}

func (SignatureUtils) scriptAddresses(quote string) *regexp.Regexp {
	return regexp.MustCompile(`(?i)\s+(href|src|xlink:href)\s*=\s*` + quote + `\s*(?:javascript|vbscript):[\s\S]*?` + quote)
}

func (SignatureUtils) scriptStyles(quote string) *regexp.Regexp {
	return regexp.MustCompile(`(?i)\s+style\s*=\s*` + quote + `[^"']*(?:expression\s*\(|url\s*\(\s*["']?\s*(?:javascript|vbscript):)[^"']*` + quote)
}
