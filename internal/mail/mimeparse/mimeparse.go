// Package mimeparse extracts the few fields the SMTP servers read from a
// received message: the decoded subject, the From/To/Cc addresses, and the
// text and HTML bodies.
//
// It replaces the subset of mailparser's simpleParser that
// background/InboundSmtpServer.js and SubmissionServer.js used. It is
// deliberately lenient, as mailparser was: malformed header lines are
// skipped rather than failing the whole message.
package mimeparse

import (
	"bytes"
	"encoding/base64"
	"fmt"
	"html"
	"io"
	"mime"
	"mime/multipart"
	"mime/quotedprintable"
	"net/mail"
	"net/textproto"
	"regexp"
	"strings"
	"unicode/utf8"
)

// Message is the parsed subset.
type Message struct {
	Subject    string
	HasSubject bool
	From       []string
	To         []string
	Cc         []string
	Text       string
	HTML       string
	Header     textproto.MIMEHeader
}

var decoder = &mime.WordDecoder{CharsetReader: charsetReader}

// Parse parses raw. It fails only when raw has no header section at all.
func Parse(raw []byte) (*Message, error) {
	header, body := splitHeader(raw)
	if len(header) == 0 && len(bytes.TrimSpace(raw)) == 0 {
		return nil, fmt.Errorf("empty message")
	}
	fields := parseHeader(header)
	message := &Message{Header: fields}
	if values, ok := fields["Subject"]; ok {
		message.HasSubject = true
		message.Subject = decodeWords(values[0])
	}
	message.From = addresses(fields.Get("From"))
	message.To = addressesOf(fields["To"])
	message.Cc = addressesOf(fields["Cc"])

	var texts, htmls []string
	walk(fields, body, 0, &texts, &htmls)
	message.Text = strings.Join(texts, "\n")
	message.HTML = strings.Join(htmls, "\n")
	if message.Text == "" && message.HTML != "" {
		message.Text = HTMLToText(message.HTML)
	}
	return message, nil
}

func splitHeader(raw []byte) ([]byte, []byte) {
	crlf := bytes.Index(raw, []byte("\r\n\r\n"))
	lf := bytes.Index(raw, []byte("\n\n"))
	switch {
	case crlf >= 0 && (lf < 0 || crlf <= lf):
		return raw[:crlf+2], raw[crlf+4:]
	case lf >= 0:
		return raw[:lf+1], raw[lf+2:]
	default:
		return raw, nil
	}
}

func parseHeader(header []byte) textproto.MIMEHeader {
	fields := textproto.MIMEHeader{}
	lines := strings.Split(strings.ReplaceAll(string(header), "\r\n", "\n"), "\n")
	var name, value string
	flush := func() {
		if name != "" {
			fields.Add(name, strings.TrimSpace(value))
		}
		name, value = "", ""
	}
	for _, line := range lines {
		if line == "" {
			continue
		}
		if line[0] == ' ' || line[0] == '\t' {
			if name != "" {
				value += " " + strings.TrimSpace(line)
			}
			continue
		}
		flush()
		colon := strings.IndexByte(line, ':')
		if colon <= 0 || strings.ContainsAny(line[:colon], " \t") {
			continue
		}
		name = textproto.CanonicalMIMEHeaderKey(line[:colon])
		value = line[colon+1:]
	}
	flush()
	return fields
}

func decodeWords(value string) string {
	decoded, err := decoder.DecodeHeader(value)
	if err != nil {
		return value
	}
	return decoded
}

var groupPattern = regexp.MustCompile(`[^,:;"<>]+:[^;]*;`)

var looseAddress = regexp.MustCompile(`[^\s<>,;:"()\[\]]+@[^\s<>,;:"()\[\]]+`)

func addressesOf(values []string) []string {
	var all []string
	for _, value := range values {
		all = append(all, addresses(value)...)
	}
	return all
}

func addresses(value string) []string {
	if strings.TrimSpace(value) == "" {
		return nil
	}
	// mailparser reports a group ("Team: a@x, b@x;") as one entry without an
	// address, so its members are not header recipients.
	value = groupPattern.ReplaceAllString(value, "")
	if strings.TrimSpace(strings.Trim(value, ", ")) == "" {
		return nil
	}
	parser := mail.AddressParser{WordDecoder: decoder}
	if list, err := parser.ParseList(value); err == nil {
		out := make([]string, 0, len(list))
		for _, address := range list {
			out = append(out, address.Address)
		}
		return out
	}
	return looseAddress.FindAllString(value, -1)
}

func walk(fields textproto.MIMEHeader, body []byte, depth int, texts, htmls *[]string) {
	if depth > 20 {
		return
	}
	mediaType, params, err := mime.ParseMediaType(fields.Get("Content-Type"))
	if err != nil || mediaType == "" {
		mediaType, params = "text/plain", map[string]string{}
	}
	disposition, _, _ := mime.ParseMediaType(fields.Get("Content-Disposition"))
	if strings.HasPrefix(mediaType, "multipart/") {
		boundary := params["boundary"]
		if boundary == "" {
			return
		}
		reader := multipart.NewReader(bytes.NewReader(body), boundary)
		for {
			part, err := reader.NextRawPart()
			if err != nil {
				return
			}
			content, err := io.ReadAll(part)
			if err != nil {
				return
			}
			walk(part.Header, content, depth+1, texts, htmls)
		}
	}
	if disposition == "attachment" {
		return
	}
	switch mediaType {
	case "text/plain":
		*texts = append(*texts, normalize(decodeBody(fields, body, params["charset"])))
	case "text/html":
		*htmls = append(*htmls, normalize(decodeBody(fields, body, params["charset"])))
	}
}

func normalize(text string) string {
	return strings.ReplaceAll(text, "\r\n", "\n")
}

func decodeBody(fields textproto.MIMEHeader, body []byte, charset string) string {
	var decoded []byte
	switch strings.ToLower(strings.TrimSpace(fields.Get("Content-Transfer-Encoding"))) {
	case "base64":
		cleaned := bytes.Map(func(r rune) rune {
			if r == '\r' || r == '\n' || r == ' ' || r == '\t' {
				return -1
			}
			return r
		}, body)
		cleaned = bytes.TrimRight(cleaned, "=")
		out := make([]byte, base64.RawStdEncoding.DecodedLen(len(cleaned)))
		n, _ := base64.RawStdEncoding.Decode(out, cleaned)
		decoded = out[:n]
	case "quoted-printable":
		out, err := io.ReadAll(quotedprintable.NewReader(bytes.NewReader(body)))
		if err != nil && len(out) == 0 {
			out = body
		}
		decoded = out
	default:
		decoded = body
	}
	return toUTF8(decoded, charset)
}

func toUTF8(data []byte, charset string) string {
	switch strings.ToLower(strings.TrimSpace(charset)) {
	case "iso-8859-1", "latin1", "latin-1", "iso8859-1":
		return latin1(data)
	case "windows-1252", "cp1252":
		return windows1252(data)
	}
	if utf8.Valid(data) {
		return string(data)
	}
	return strings.ToValidUTF8(string(data), "�")
}

func latin1(data []byte) string {
	runes := make([]rune, len(data))
	for index, b := range data {
		runes[index] = rune(b)
	}
	return string(runes)
}

var cp1252 = map[byte]rune{
	0x80: '€', 0x82: '‚', 0x83: 'ƒ', 0x84: '„', 0x85: '…', 0x86: '†', 0x87: '‡', 0x88: 'ˆ', 0x89: '‰',
	0x8a: 'Š', 0x8b: '‹', 0x8c: 'Œ', 0x8e: 'Ž', 0x91: '‘', 0x92: '’', 0x93: '“', 0x94: '”', 0x95: '•',
	0x96: '–', 0x97: '—', 0x98: '˜', 0x99: '™', 0x9a: 'š', 0x9b: '›', 0x9c: 'œ', 0x9e: 'ž', 0x9f: 'Ÿ',
}

func windows1252(data []byte) string {
	runes := make([]rune, len(data))
	for index, b := range data {
		if mapped, ok := cp1252[b]; ok {
			runes[index] = mapped
		} else {
			runes[index] = rune(b)
		}
	}
	return string(runes)
}

func charsetReader(charset string, input io.Reader) (io.Reader, error) {
	data, err := io.ReadAll(input)
	if err != nil {
		return nil, err
	}
	return strings.NewReader(toUTF8(data, charset)), nil
}

var (
	breakTags   = regexp.MustCompile(`(?i)<\s*(br|/p|/div|/li|/tr|/h[1-6])\b[^>]*>`)
	hiddenBlock = regexp.MustCompile(`(?is)<\s*(script|style|head)\b.*?<\s*/\s*(script|style|head)\s*>`)
	anyTag      = regexp.MustCompile(`(?s)<[^>]*>`)
	blankLines  = regexp.MustCompile(`\n{3,}`)
	spaces      = regexp.MustCompile(`[ \t]+`)
)

// HTMLToText is a small stand-in for the html-to-text conversion mailparser
// applied when a message carried only HTML.
func HTMLToText(markup string) string {
	text := hiddenBlock.ReplaceAllString(markup, "")
	text = breakTags.ReplaceAllString(text, "\n")
	text = anyTag.ReplaceAllString(text, "")
	text = html.UnescapeString(text)
	lines := strings.Split(text, "\n")
	for index, line := range lines {
		lines[index] = strings.TrimSpace(spaces.ReplaceAllString(line, " "))
	}
	return strings.TrimSpace(blankLines.ReplaceAllString(strings.Join(lines, "\n"), "\n\n"))
}
