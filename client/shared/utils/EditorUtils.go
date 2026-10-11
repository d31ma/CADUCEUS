import (
	"html"
	"regexp"
	"slices"
	"strconv"
	"strings"
)

// EditorUtils builds the document of the composer's message editor. Tac
// replaces a contenteditable element whenever it renders without focus, and a
// Tac script may not write markup (TY1364), so the message body lives in a
// sandboxed iframe whose srcdoc the view binds to this document, as the shared
// conversation page does for message HTML. Inline images show their bytes as
// data: URLs; the editor turns them back into cid: references when it reads
// the body.
type EditorUtils struct{}

// Document is the editor document for one body. The generation makes every
// document differ, so the frame reloads even when the same body is set again.
func (EditorUtils) Document(body string, attachments []ComposeAttachment, generation int) string {
	return `<!doctype html><html><head><meta charset="utf-8"><meta name="caduceus-editor" content="` + strconv.Itoa(generation) + `">` +
		`<link rel="stylesheet" href="/shared/assets/duvay/duvay.min.css"><link rel="stylesheet" href="/shared/assets/themes.css"><style>` + EditorUtils{}.styles() + `</style></head>` +
		`<body contenteditable="true" data-compose-editor role="textbox" aria-multiline="true" aria-label="Message body" spellcheck="true" data-placeholder="Write your message…">` +
		EditorUtils{}.hydrated(body, EditorUtils{}.byContentID(attachments)) + `</body></html>`
}

// hydrated shows each cid: image of the body with the bytes of its inline
// attachment, like editableComposerHTML in rich-compose-editor.js.
func (EditorUtils) hydrated(body string, byID map[string]ComposeAttachment) string {
	return EditorUtils{}.contentSource().ReplaceAllStringFunc(body, func(source string) string {
		return EditorUtils{}.imageSource(source, byID)
	})
}

func (EditorUtils) byContentID(attachments []ComposeAttachment) map[string]ComposeAttachment {
	byID := map[string]ComposeAttachment{}
	for _, attachment := range attachments {
		byID[strings.ToLower(attachment.ContentID)] = attachment
	}
	return byID
}

// InlineImage reports whether a file type may be shown inline.
func (EditorUtils) InlineImage(contentType string) bool {
	return slices.Contains([]string{"image/png", "image/jpeg", "image/gif", "image/webp", "image/avif"}, strings.ToLower(contentType))
}

func (EditorUtils) imageSource(source string, byID map[string]ComposeAttachment) string {
	parts := EditorUtils{}.contentSource().FindStringSubmatch(source)
	cid := strings.Trim(parts[1]+parts[2], "<>")
	attachment, found := byID[strings.ToLower(cid)]
	if !found || !(EditorUtils{}).InlineImage(attachment.ContentType) {
		return `src="data:image/gif;base64,R0lGODlhAQABAIAAAAAAAP///ywAAAAAAQABAAACAUwAOw==#missing-inline-image" data-caduceus-cid="` + html.EscapeString(cid) + `"`
	}
	return `src="data:` + html.EscapeString(attachment.ContentType) + `;base64,` + attachment.ContentBase64 + `" data-caduceus-cid="` + html.EscapeString(attachment.ContentID) +
		`" data-caduceus-attachment-id="` + html.EscapeString(attachment.ID) + `" data-caduceus-inline-image=""`
}

func (EditorUtils) contentSource() *regexp.Regexp {
	return regexp.MustCompile(`(?i)\bsrc\s*=\s*(?:"cid:([^"]+)"|'cid:([^']+)')`)
}

func (EditorUtils) styles() string {
	return `html{color-scheme:light dark}body{margin:0;min-height:340px;padding:1rem;font-family:var(--caduceus-composer-font,system-ui,sans-serif);font-size:var(--ms-font-md,0.9375rem);line-height:1.7;color:var(--ms-on-surface,CanvasText);background:var(--ms-surface,Canvas);outline:none;overflow-wrap:anywhere}` +
		`body:empty::before{content:attr(data-placeholder);color:var(--ms-on-surface-variant,GrayText);opacity:.6}body[contenteditable="plaintext-only"]{white-space:pre-wrap}` +
		`signature.caduceus-signature{display:block;margin-top:.5rem;padding-top:.5rem;border-top:1px dashed var(--ms-outline-variant,GrayText);color:var(--ms-on-surface-variant,GrayText);cursor:default}` +
		`body.compose-quote-collapsed [data-caduceus-quote]{display:none}[data-caduceus-quote]{margin-top:1rem;color:var(--ms-on-surface-variant,GrayText)}` +
		`blockquote{margin:.5rem 0;padding-inline-start:.75rem;border-inline-start:2px solid var(--ms-outline,GrayText)}` +
		`img{max-width:100%;height:auto}img[data-caduceus-inline-image]{display:block;margin:.5rem 0;border-radius:4px}img[data-selected]{outline:3px solid var(--ms-primary,Highlight);outline-offset:2px}` +
		`code.var,code[data-template-var]{border-radius:3px;background:color-mix(in srgb,var(--ms-primary,Highlight) 16%,transparent);box-shadow:inset 0 -1px 0 color-mix(in srgb,var(--ms-primary,Highlight) 55%,transparent);color:inherit;font:inherit;padding:0 .15em}` +
		`code.var.empty,code[data-template-var].empty{min-width:2.5ch;display:inline-block;background:color-mix(in srgb,#9a6700 22%,transparent);box-shadow:inset 0 -2px 0 #9a6700}` +
		`::highlight(caduceus-grammar-errors){text-decoration:underline wavy var(--ms-error,red);text-decoration-thickness:1.5px;text-underline-offset:3px}`
}
