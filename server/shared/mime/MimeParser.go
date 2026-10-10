import (
	"bytes"
	"encoding/base64"
	"io"
	"mime/quotedprintable"
	"strings"
)

// MimeParser reads an inbound RFC 822 message into its headers, its text and
// HTML bodies and its attachments, on the standard library: multipart
// bodies, base64 and quoted-printable transfer encodings, RFC 2047 words,
// RFC 2231 parameters and the charsets MimeCharset knows.
//
// A part is an attachment when its disposition is attachment or when it is
// not a text/plain or text/html part; every other text part joins the text
// or the HTML body. Nesting stops after a bounded depth.
type MimeParser struct{}

func (MimeParser) deepestNesting() int { return 32 }

func (MimeParser) mostAttachments() int { return 1000 }

// Parse answers the parsed message.
func (parser MimeParser) Parse(raw []byte) ParsedMessage {
	block, body := MimeHeaders{}.Split(raw)
	headers := MimeHeaders{}.Parsed(block)
	message := ParsedMessage{Headers: headers}
	parser.part(&message, headers, body, 0)
	return message
}

func (parser MimeParser) part(message *ParsedMessage, headers MimeHeaders, body []byte, depth int) {
	kind, parameters := headers.MediaType(headers.Get("content-type"))
	if kind == "" {
		kind = "text/plain"
	}
	if strings.HasPrefix(kind, "multipart/") && parameters["boundary"] != "" && depth < parser.deepestNesting() {
		for _, child := range parser.children(body, parameters["boundary"]) {
			childBlock, childBody := MimeHeaders{}.Split(child)
			parser.part(message, MimeHeaders{}.Parsed(childBlock), childBody, depth+1)
		}
		return
	}
	content := parser.decoded(headers.Get("content-transfer-encoding"), body)
	disposition, dispositionParameters := headers.MediaType(headers.Get("content-disposition"))
	filename := dispositionParameters["filename"]
	if filename == "" {
		filename = parameters["name"]
	}
	if disposition != "attachment" && (kind == "text/plain" || kind == "text/html") {
		text := strings.ReplaceAll(MimeCharset{}.Text(parameters["charset"], content), "\r\n", "\n")
		if kind == "text/html" {
			message.HtmlParts = append(message.HtmlParts, text)
		} else {
			message.TextParts = append(message.TextParts, text)
		}
		return
	}
	if len(message.Attachments) >= parser.mostAttachments() {
		return
	}
	if disposition == "" {
		disposition = "attachment"
	}
	message.Attachments = append(message.Attachments, MimeAttachment{
		Filename: filename, ContentType: kind, Content: content,
		Disposition: disposition, ContentId: strings.TrimSpace(headers.Get("content-id")),
	})
}

// children answers the body parts between the boundary delimiters of a
// multipart body (the preamble and epilogue are dropped). The line break
// before each delimiter belongs to the delimiter.
func (parser MimeParser) children(body []byte, boundary string) [][]byte {
	delimiter := "--" + boundary
	children := [][]byte{}
	var current []byte
	inside := false
	for _, line := range bytes.SplitAfter(body, []byte("\n")) {
		trimmed := string(bytes.TrimRight(line, " \t\r\n"))
		if trimmed == delimiter || trimmed == delimiter+"--" {
			if inside {
				children = append(children, parser.withoutLastBreak(current))
			}
			current, inside = nil, trimmed == delimiter
			if !inside {
				return children
			}
			continue
		}
		if inside {
			current = append(current, line...)
		}
	}
	if inside {
		children = append(children, current)
	}
	return children
}

func (MimeParser) withoutLastBreak(part []byte) []byte {
	return bytes.TrimSuffix(bytes.TrimSuffix(part, []byte("\n")), []byte("\r"))
}

// decoded answers a part body without its transfer encoding; a body that
// does not decode is kept as it is.
func (MimeParser) decoded(encoding string, body []byte) []byte {
	switch strings.ToLower(strings.TrimSpace(encoding)) {
	case "base64":
		compact := bytes.Map(func(letter rune) rune {
			if strings.ContainsRune("ABCDEFGHIJKLMNOPQRSTUVWXYZabcdefghijklmnopqrstuvwxyz0123456789+/", letter) {
				return letter
			}
			return -1
		}, body)
		if len(compact)%4 == 1 {
			compact = compact[:len(compact)-1]
		}
		decoded, _ := base64.RawStdEncoding.DecodeString(string(compact))
		return decoded
	case "quoted-printable":
		decoded, err := io.ReadAll(quotedprintable.NewReader(bytes.NewReader(body)))
		if err != nil && len(decoded) == 0 {
			return body
		}
		return decoded
	}
	return body
}
