import (
	"regexp"
	"strings"
)

// InboundMime reads raw inbound message bodies (plain text, HTML or RFC 822
// MIME) into text, HTML, attachments and headers, as the former
// InboundService.parseMessage did with mailparser: plain text and HTML that
// do not look like an RFC message, and unencoded single-part messages, are
// read here; every other message goes through the mail-reading MimeParser.
type InboundMime struct{}

// Parsed answers the message read from raw.
func (reader InboundMime) Parsed(raw string) ParsedMessage {
	if raw == "" {
		return ParsedMessage{Headers: map[string]any{}, Attachments: []ParsedAttachment{}}
	}
	if !reader.LooksLikeRfcMessage(raw) {
		html := ""
		if reader.LooksLikeHtml(raw) {
			html = raw
		}
		return ParsedMessage{Text: reader.textOr(HtmlText{}.Stripped(html), raw), Html: html, Attachments: []ParsedAttachment{}, Headers: map[string]any{}}
	}
	if simple, isSimple := reader.simple(raw); isSimple {
		return simple
	}
	return reader.full(raw)
}

// LooksLikeHtml reports whether value holds a closed HTML element or a void
// tag.
func (InboundMime) LooksLikeHtml(value string) bool {
	return HtmlText{}.LooksLikeHtml(value)
}

// LooksLikeRfcMessage reports whether the text begins with a plausible RFC
// header block: every line before the first blank line is a header or a
// continuation, and one of them is a header mail carries.
func (reader InboundMime) LooksLikeRfcMessage(raw string) bool {
	split := regexp.MustCompile(`\r?\n\r?\n`).FindStringIndex(raw)
	if split == nil {
		return false
	}
	lines := regexp.MustCompile(`\r?\n`).Split(raw[:split[0]], -1)
	header := regexp.MustCompile("^[!#$%&'*+\\-.^_`|~0-9A-Za-z]+:\\s*")
	known := regexp.MustCompile(`(?i)^(?:authentication-results|cc|content-[^:]+|date|from|message-id|mime-version|received|reply-to|subject|to|x-[^:]+):`)
	recognizable := false
	for _, line := range lines {
		if !header.MatchString(line) && !regexp.MustCompile(`^[ \t]+`).MatchString(line) {
			return false
		}
		recognizable = recognizable || known.MatchString(line)
	}
	return len(lines) > 0 && recognizable
}

// simple reads an unencoded single-part message without the full parser.
func (reader InboundMime) simple(raw string) (ParsedMessage, bool) {
	split := regexp.MustCompile(`\r?\n\r?\n`).FindStringIndex(raw)
	block := raw[:split[0]]
	headers := reader.headerBlock(block)
	contentType := strings.ToLower(RecordValues{}.Text(RecordValues{}.Or(headers["content-type"], "text/plain")))
	encoding := strings.ToLower(RecordValues{}.Text(headers["content-transfer-encoding"]))
	if strings.Contains(contentType, "multipart/") || regexp.MustCompile(`(?i)content-disposition:\s*attachment`).MatchString(block) ||
		(encoding != "" && encoding != "7bit" && encoding != "8bit" && encoding != "binary") {
		return ParsedMessage{}, false
	}
	body := raw[split[1]:]
	html := ""
	if strings.Contains(contentType, "text/html") || reader.LooksLikeHtml(body) {
		html = body
	}
	return ParsedMessage{Text: reader.textOr(HtmlText{}.Stripped(html), body), Html: html, Attachments: []ParsedAttachment{}, Headers: headers}, true
}

// full reads a MIME message with the mail-reading MimeParser: its headers
// flattened as the former route stored them, its text and HTML parts, and
// its attachments with the defaults mailparser gave them.
func (reader InboundMime) full(raw string) ParsedMessage {
	message := MimeParser{}.Parse([]byte(raw))
	parsed := ParsedMessage{Headers: message.Headers.Flattened(), Attachments: []ParsedAttachment{}}
	for _, attachment := range message.Attachments {
		parsed.Attachments = append(parsed.Attachments, ParsedAttachment{
			Filename: reader.textOr(attachment.Filename, "attachment"), ContentType: reader.textOr(attachment.ContentType, "application/octet-stream"),
			Content: attachment.Content, Disposition: attachment.Disposition, ContentId: strings.Trim(attachment.ContentId, "<> "),
		})
	}
	parsed.Html, _ = message.Html()
	if parsed.Html == "" && reader.LooksLikeHtml(raw) && len(message.TextParts) == 0 {
		parsed.Html = raw
	}
	parsed.Text = reader.textOr(message.Text(), reader.textOr(HtmlText{}.Stripped(parsed.Html), raw))
	return parsed
}

// headerBlock reads an unfolded header block by lower-case name (the last
// occurrence wins).
func (InboundMime) headerBlock(block string) map[string]any {
	headers := map[string]any{}
	unfolded := regexp.MustCompile(`\r?\n[ \t]+`).ReplaceAllString(block, " ")
	for _, line := range regexp.MustCompile(`\r?\n`).Split(unfolded, -1) {
		colon := strings.IndexByte(line, ':')
		if colon <= 0 {
			continue
		}
		headers[strings.ToLower(Security{}.Trim(line[:colon]))] = Security{}.Trim(line[colon+1:])
	}
	return headers
}

func (InboundMime) textOr(text string, fallback string) string {
	if text != "" {
		return text
	}
	return fallback
}
