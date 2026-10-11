import (
	"io"
	"mime"
	"mime/multipart"
	"mime/quotedprintable"
	"net/mail"
	"regexp"
	"strings"
	"unicode/utf8"
)

// InboundMime reads raw inbound message bodies (plain text, HTML or RFC 822
// MIME) into text, HTML, attachments and headers, as the former
// InboundService.parseMessage did with mailparser. Mail reading owns MIME
// parsing; this is the minimal private version inbound delivery needs,
// built on net/mail and mime/multipart.
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
		return ParsedMessage{Text: reader.textOr(SentMail{}.StrippedHtml(html), raw), Html: html, Attachments: []ParsedAttachment{}, Headers: map[string]any{}}
	}
	if simple, isSimple := reader.simple(raw); isSimple {
		return simple
	}
	if parsed, err := reader.full(raw); err == nil {
		return parsed
	}
	if fallback, isMultipart := reader.multipartFallback(raw); isMultipart {
		return fallback
	}
	html := ""
	if reader.LooksLikeHtml(raw) {
		html = raw
	}
	return ParsedMessage{Text: reader.textOr(SentMail{}.StrippedHtml(raw), raw), Html: html, Attachments: []ParsedAttachment{}, Headers: map[string]any{}}
}

// LooksLikeHtml reports whether value holds a closed HTML element or a void
// tag.
func (InboundMime) LooksLikeHtml(value string) bool {
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
	return ParsedMessage{Text: reader.textOr(SentMail{}.StrippedHtml(html), body), Html: html, Attachments: []ParsedAttachment{}, Headers: headers}, true
}

// full reads a MIME message with net/mail and mime/multipart.
func (reader InboundMime) full(raw string) (ParsedMessage, error) {
	message, err := mail.ReadMessage(strings.NewReader(raw))
	if err != nil {
		return ParsedMessage{}, err
	}
	parsed := ParsedMessage{Headers: reader.flattened(message.Header), Attachments: []ParsedAttachment{}}
	texts, htmls := []string{}, []string{}
	if err := reader.part(map[string][]string(message.Header), message.Body, &parsed, &texts, &htmls); err != nil {
		return ParsedMessage{}, err
	}
	parsed.Html = strings.Join(htmls, "\n")
	if parsed.Html == "" && reader.LooksLikeHtml(raw) && len(texts) == 0 {
		parsed.Html = raw
	}
	parsed.Text = reader.textOr(strings.Join(texts, "\n"), reader.textOr(SentMail{}.StrippedHtml(parsed.Html), raw))
	return parsed, nil
}

// part reads one entity, recursing into multiparts.
func (reader InboundMime) part(header map[string][]string, body io.Reader, parsed *ParsedMessage, texts *[]string, htmls *[]string) error {
	get := func(name string) string {
		if values := header[name]; len(values) > 0 {
			return values[0]
		}
		return ""
	}
	mediaType, params, err := mime.ParseMediaType(get("Content-Type"))
	if err != nil {
		mediaType, params = "text/plain", map[string]string{}
	}
	if strings.HasPrefix(mediaType, "multipart/") {
		parts := multipart.NewReader(body, params["boundary"])
		for {
			part, err := parts.NextRawPart()
			if err == io.EOF {
				return nil
			}
			if err != nil {
				return err
			}
			if err := reader.part(part.Header, part, parsed, texts, htmls); err != nil {
				return err
			}
		}
	}
	content, err := reader.decoded(body, get("Content-Transfer-Encoding"))
	if err != nil {
		return err
	}
	disposition, dispositionParams, _ := mime.ParseMediaType(get("Content-Disposition"))
	filename := reader.word(dispositionParams["filename"])
	if filename == "" {
		filename = reader.word(params["name"])
	}
	textual := mediaType == "text/plain" || mediaType == "text/html"
	if textual && disposition != "attachment" && filename == "" {
		text := reader.charset(content, params["charset"])
		if mediaType == "text/html" {
			*htmls = append(*htmls, text)
		} else {
			*texts = append(*texts, text)
		}
		return nil
	}
	if filename == "" {
		filename = "attachment"
	}
	if disposition == "" {
		disposition = "attachment"
	}
	parsed.Attachments = append(parsed.Attachments, ParsedAttachment{
		Filename: filename, ContentType: mediaType, Content: content, Disposition: disposition,
		ContentId: strings.Trim(get("Content-Id"), "<> "),
	})
	return nil
}

func (InboundMime) decoded(body io.Reader, encoding string) ([]byte, error) {
	switch strings.ToLower(strings.TrimSpace(encoding)) {
	case "base64":
		data, err := io.ReadAll(body)
		if err != nil {
			return nil, err
		}
		cleaned := regexp.MustCompile(`[^A-Za-z0-9+/]`).ReplaceAll(data, nil)
		return MailValidation{}.Decoded(string(cleaned)), nil
	case "quoted-printable":
		return io.ReadAll(quotedprintable.NewReader(body))
	}
	return io.ReadAll(body)
}

// charset answers text in a part's charset as UTF-8 (Latin-1 and its
// Windows superset are mapped byte for byte; other charsets pass as they
// are).
func (InboundMime) charset(content []byte, charset string) string {
	switch strings.ToLower(charset) {
	case "iso-8859-1", "latin1", "windows-1252", "us-ascii":
		if utf8.Valid(content) && strings.ToLower(charset) == "us-ascii" {
			return string(content)
		}
		runes := make([]rune, len(content))
		for index, value := range content {
			runes[index] = rune(value)
		}
		return string(runes)
	}
	return string(content)
}

func (InboundMime) word(value string) string {
	decoded, err := new(mime.WordDecoder).DecodeHeader(value)
	if err != nil {
		return value
	}
	return decoded
}

// flattened answers the headers by lower-case name; address headers become
// their addresses.
func (reader InboundMime) flattened(header mail.Header) map[string]any {
	flattened := map[string]any{}
	for name, values := range header {
		key := strings.ToLower(name)
		value := reader.word(strings.Join(values, ", "))
		switch key {
		case "from", "to", "cc", "bcc", "reply-to", "sender":
			if addresses, err := header.AddressList(name); err == nil {
				listed := []string{}
				for _, address := range addresses {
					listed = append(listed, address.Address)
				}
				value = strings.Join(listed, ", ")
			}
		}
		flattened[key] = value
	}
	return flattened
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

// multipartFallback is the dependency-free MIME reader used when the full
// parser fails.
func (reader InboundMime) multipartFallback(raw string) (ParsedMessage, bool) {
	split := regexp.MustCompile(`\r?\n\r?\n`).FindStringIndex(raw)
	if split == nil {
		return ParsedMessage{}, false
	}
	top := reader.headerBlock(raw[:split[0]])
	match := regexp.MustCompile(`(?i)boundary=(?:"([^"]+)"|([^;\s]+))`).FindStringSubmatch(RecordValues{}.Text(top["content-type"]))
	if match == nil {
		return ParsedMessage{}, false
	}
	boundary := match[1] + match[2]
	parsed := ParsedMessage{Headers: top, Attachments: []ParsedAttachment{}}
	texts, htmls := []string{}, []string{}
	sections := strings.Split(raw[split[0]:], "--"+boundary)
	for _, section := range sections[1:] {
		part := strings.TrimSpace(regexp.MustCompile(`\r?\n--\s*$`).ReplaceAllString(regexp.MustCompile(`^\r?\n`).ReplaceAllString(section, ""), ""))
		partSplit := regexp.MustCompile(`\r?\n\r?\n`).FindStringIndex(part)
		if part == "" || part == "--" || partSplit == nil {
			continue
		}
		headers := reader.headerBlock(part[:partSplit[0]])
		body := regexp.MustCompile(`\r?\n$`).ReplaceAllString(part[partSplit[1]:], "")
		disposition := RecordValues{}.Text(headers["content-disposition"])
		contentType := strings.ToLower(Security{}.Trim(strings.SplitN(RecordValues{}.Text(RecordValues{}.Or(headers["content-type"], "text/plain")), ";", 2)[0]))
		filename := reader.parameter(disposition, "filename")
		if filename == "" {
			filename = reader.parameter(RecordValues{}.Text(headers["content-type"]), "name")
		}
		switch {
		case regexp.MustCompile(`(?i)attachment`).MatchString(disposition) || filename != "":
			content := []byte(body)
			if strings.ToLower(RecordValues{}.Text(headers["content-transfer-encoding"])) == "base64" {
				content = MailValidation{}.Decoded(regexp.MustCompile(`\s+`).ReplaceAllString(body, ""))
			}
			kind := "attachment"
			if regexp.MustCompile(`(?i)inline`).MatchString(disposition) {
				kind = "inline"
			}
			parsed.Attachments = append(parsed.Attachments, ParsedAttachment{
				Filename: reader.textOr(filename, "attachment"), ContentType: contentType, Content: content, Disposition: kind,
				ContentId: strings.Trim(RecordValues{}.Text(headers["content-id"]), "<>"),
			})
		case contentType == "text/html":
			htmls = append(htmls, body)
		case contentType == "text/plain":
			texts = append(texts, body)
		}
	}
	parsed.Html = strings.Join(htmls, "\n")
	parsed.Text = reader.textOr(strings.Join(texts, "\n"), reader.textOr(SentMail{}.StrippedHtml(parsed.Html), raw))
	return parsed, true
}

func (InboundMime) parameter(value string, name string) string {
	match := regexp.MustCompile(`(?i)` + name + `=(?:"([^"]+)"|([^;\s]+))`).FindStringSubmatch(value)
	if match == nil {
		return ""
	}
	return match[1] + match[2]
}

func (InboundMime) textOr(text string, fallback string) string {
	if text != "" {
		return text
	}
	return fallback
}
