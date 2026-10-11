import "regexp"

// ProtectedMailUtils wraps message HTML in a document that the share page's
// sandboxed frame may show. Remote resource attributes are renamed so the
// browser never requests them, and the document's Content Security Policy
// blocks whatever remains: remote images, tracking pixels, scripts, forms,
// frames and embedded media. This is a textual pass, not an HTML parser; the
// DOM pass of shared/scripts/email-content-protection.js (quoted-text markers
// and suspicious-link marks) needs the browser and is not repeated here.
type ProtectedMailUtils struct{}

// Document is the locked-down srcdoc for one HTML body, or empty for none.
func (ProtectedMailUtils) Document(html string) string {
	return map[bool]string{true: ProtectedMailUtils{}.wrapped(ProtectedMailUtils{}.withoutRemote(html))}[html != ""]
}

func (ProtectedMailUtils) withoutRemote(html string) string {
	attributes := regexp.MustCompile(`(?i)(\s)(src|srcset|background|poster|data)(\s*=\s*["']?\s*(?:https?:)?//)`)
	styles := regexp.MustCompile(`(?i)url\(\s*(["']?)\s*(?:https?:)?//`)
	return styles.ReplaceAllString(attributes.ReplaceAllString(html, "${1}data-caduceus-blocked-${2}${3}"), "url(${1}about:blank#")
}

func (ProtectedMailUtils) wrapped(html string) string {
	return `<!doctype html><html><head><meta charset="utf-8"><meta http-equiv="Content-Security-Policy" content="default-src 'none'; img-src data:; style-src 'unsafe-inline'; font-src data:; media-src 'none'; connect-src 'none'; frame-src 'none'; object-src 'none'; base-uri 'none'; form-action 'none'; script-src 'none'"><style>html{color-scheme:light;background:#fff;color:#111}body{margin:0;padding:16px;font:14px/1.55 -apple-system,BlinkMacSystemFont,'Segoe UI',sans-serif;overflow-wrap:anywhere}a{color:#0645ad}img{max-width:100%;height:auto}table{max-width:100%}</style></head><body>` + html + `</body></html>`
}
