package mailprovider

import (
	"bytes"
	"html"
	"io"
	"mime"
	"mime/multipart"
	"net/mail"
	"net/textproto"
	"path/filepath"
	"regexp"
	"sort"
	"strings"
	"unicode/utf8"
)

const maxIndexedTextBytes = 64 << 10

var htmlTagPattern = regexp.MustCompile(`(?s)<[^>]*>`)

type messageIndex struct {
	RFCMessageID    string
	InReplyTo       string
	References      []string
	Subject         string
	Sender          string
	To              []string
	CC              []string
	BCC             []string
	Snippet         string
	AttachmentNames []string
}

func indexRFC822(raw []byte) messageIndex {
	parsed, err := mail.ReadMessage(bytes.NewReader(raw))
	if err != nil {
		return messageIndex{}
	}
	result := messageIndex{
		RFCMessageID: firstMessageID(parsed.Header.Get("Message-ID")),
		InReplyTo:    firstMessageID(parsed.Header.Get("In-Reply-To")),
		References:   headerMessageIDs(parsed.Header.Get("References"), 100),
		Subject:      decodeMIMEHeader(parsed.Header.Get("Subject"), 998),
		Sender:       decodeMIMEHeader(parsed.Header.Get("From"), 512),
		To:           headerAddresses(parsed.Header.Get("To")),
		CC:           headerAddresses(parsed.Header.Get("Cc")),
		BCC:          headerAddresses(parsed.Header.Get("Bcc")),
	}
	budget := maxIndexedTextBytes
	plain, rich := indexMIMEEntity(textproto.MIMEHeader(parsed.Header), parsed.Body, 0, &budget, &result.AttachmentNames)
	if plain == "" {
		plain = html.UnescapeString(htmlTagPattern.ReplaceAllString(rich, " "))
	}
	result.Snippet = boundedText(strings.Join(strings.Fields(plain), " "), 4096)
	result.AttachmentNames = uniqueSorted(result.AttachmentNames, 100)
	return result
}

func firstMessageID(value string) string {
	values := headerMessageIDs(value, 1)
	if len(values) == 0 {
		return ""
	}
	return values[0]
}

func headerMessageIDs(value string, maximum int) []string {
	result := []string{}
	seen := map[string]bool{}
	for _, match := range regexp.MustCompile(`<[^<>\s]{1,996}>`).FindAllString(value, maximum) {
		if seen[match] {
			continue
		}
		seen[match] = true
		result = append(result, match)
	}
	return result
}

func indexMIMEEntity(header textproto.MIMEHeader, body io.Reader, depth int, budget *int, attachments *[]string) (string, string) {
	if depth > 16 {
		return "", ""
	}
	contentType, typeParams, _ := mime.ParseMediaType(header.Get("Content-Type"))
	if contentType == "" {
		contentType = "text/plain"
	}
	disposition, dispositionParams, _ := mime.ParseMediaType(header.Get("Content-Disposition"))
	filename := dispositionParams["filename"]
	if filename == "" {
		filename = typeParams["name"]
	}
	if disposition == "attachment" || filename != "" {
		name := safeAttachmentName(decodeMIMEHeader(filename, 255))
		if name == "" {
			name = "attachment"
		}
		*attachments = append(*attachments, name)
		return "", ""
	}
	if strings.HasPrefix(strings.ToLower(contentType), "multipart/") {
		boundary := typeParams["boundary"]
		if boundary == "" {
			return "", ""
		}
		reader := multipart.NewReader(body, boundary)
		plainParts, richParts := []string{}, []string{}
		for count := 0; count < 250; count++ {
			part, err := reader.NextPart()
			if err != nil {
				break
			}
			plain, rich := indexMIMEEntity(part.Header, part, depth+1, budget, attachments)
			_ = part.Close()
			if plain != "" {
				plainParts = append(plainParts, plain)
			}
			if rich != "" {
				richParts = append(richParts, rich)
			}
		}
		return strings.Join(plainParts, " "), strings.Join(richParts, " ")
	}
	if *budget <= 0 {
		return "", ""
	}
	if contentType != "text/plain" && contentType != "text/html" {
		return "", ""
	}
	maximum := *budget
	payload, _ := io.ReadAll(io.LimitReader(body, int64(maximum)+1))
	if len(payload) > maximum {
		payload = payload[:maximum]
	}
	*budget -= len(payload)
	text := string(bytes.ToValidUTF8(payload, []byte("�")))
	if contentType == "text/plain" {
		return text, ""
	}
	return "", text
}

func headerAddresses(value string) []string {
	addresses, err := mail.ParseAddressList(value)
	if err != nil {
		return []string{}
	}
	result := make([]string, 0, len(addresses))
	seen := map[string]bool{}
	for _, address := range addresses {
		normalized := strings.ToLower(strings.TrimSpace(address.Address))
		if normalized == "" || len(normalized) > 254 || seen[normalized] {
			continue
		}
		seen[normalized] = true
		result = append(result, normalized)
		if len(result) == 100 {
			break
		}
	}
	return result
}

func decodeMIMEHeader(value string, limit int) string {
	decoded, err := (&mime.WordDecoder{}).DecodeHeader(value)
	if err == nil {
		value = decoded
	}
	value = strings.Join(strings.Fields(strings.ReplaceAll(strings.ReplaceAll(value, "\r", " "), "\n", " ")), " ")
	return boundedText(value, limit)
}

func safeAttachmentName(value string) string {
	value = filepath.Base(strings.ReplaceAll(strings.TrimSpace(value), "\\", "/"))
	if value == "." || value == ".." || strings.ContainsAny(value, "\x00\r\n") {
		return ""
	}
	return boundedText(value, 255)
}

func boundedText(value string, limit int) string {
	if len(value) <= limit {
		return value
	}
	for len(value) > limit || !utf8.ValidString(value) {
		value = value[:len(value)-1]
	}
	return value
}

func uniqueSorted(values []string, maximum int) []string {
	seen := map[string]bool{}
	result := []string{}
	for _, value := range values {
		if value == "" || seen[value] {
			continue
		}
		seen[value] = true
		result = append(result, value)
	}
	sort.Strings(result)
	if len(result) > maximum {
		result = result[:maximum]
	}
	return result
}
