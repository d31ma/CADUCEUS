package mime

import (
	"io"
	stdmime "mime"
	"net/textproto"
	"strings"
)

type entity struct {
	header            textproto.MIMEHeader
	body              io.Reader
	depth             int
	mediaType         string
	typeParams        map[string]string
	disposition       string
	dispositionParams map[string]string
}

func newEntity(header textproto.MIMEHeader, body io.Reader, depth int) entity {
	part := entity{header: header, body: body, depth: depth}
	part.mediaType, part.typeParams = part.media(header.Get("Content-Type"), part.plainText())
	part.disposition, part.dispositionParams = part.media(header.Get("Content-Disposition"), "")
	return part
}

func (entity) plainText() string {
	return "text/plain"
}

func (entity) media(value string, fallback string) (string, map[string]string) {
	parsed, params, _ := stdmime.ParseMediaType(value)
	if parsed == "" {
		return fallback, map[string]string{}
	}
	return parsed, params
}

func (part entity) isMultipart() bool {
	return strings.HasPrefix(part.mediaType, "multipart/") && part.boundary() != ""
}

func (part entity) boundary() string {
	return part.typeParams["boundary"]
}

func (part entity) charset() string {
	return part.typeParams["charset"]
}

func (part entity) encoding() string {
	return part.header.Get("Content-Transfer-Encoding")
}

func (part entity) content() []byte {
	raw, _ := io.ReadAll(part.body)
	return transferEncoding{}.Decode(part.encoding(), raw)
}

func (part entity) fileName() string {
	name := part.firstNonEmpty(part.dispositionParams["filename"], part.typeParams["name"])
	return encodedWords{}.Decode(name)
}

func (entity) firstNonEmpty(first string, second string) string {
	if first != "" {
		return first
	}
	return second
}

func (part entity) contentId() string {
	return messageIds{}.First(part.header.Get("Content-Id"))
}

func (part entity) inline() bool {
	return part.disposition == "inline" || (part.disposition == "" && part.contentId() != "")
}

func (part entity) bodyKind() string {
	if part.disposition == "attachment" || part.fileName() != "" {
		return ""
	}
	return part.mediaType
}
