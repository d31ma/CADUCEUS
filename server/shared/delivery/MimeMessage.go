import (
	"bytes"
	"crypto/rand"
	"encoding/base64"
	"encoding/hex"
	"mime"
	"mime/quotedprintable"
	"regexp"
	"strconv"
	"strings"
	"time"
)

// MimeMessage builds the standards-compliant RFC 822 source of an outbound
// message, as nodemailer's mail composer did: the Sent copy keeps it and
// SMTP delivery sends it. Text bodies are quoted-printable UTF-8;
// attachments are base64; text and HTML are multipart/alternative, inline
// attachments join the HTML in multipart/related, and other attachments wrap
// everything in multipart/mixed. Bcc never appears in the headers.
type MimeMessage struct{}

// Source answers the message source from the composed fields (to, cc,
// replyTo, subject, text, html, inReplyTo, references, attachments,
// messageId) of message.
func (composer MimeMessage) Source(from string, message map[string]any, messageId string, date time.Time) []byte {
	fields := DeliveryFields{}
	values := RecordValues{}
	var source bytes.Buffer
	composer.header(&source, "From", from)
	for _, header := range [][2]string{{"To", "to"}, {"Cc", "cc"}, {"Reply-To", "replyTo"}} {
		if addresses := fields.Strings(message[header[1]]); len(addresses) > 0 {
			composer.header(&source, header[0], strings.Join(addresses, ", "))
		}
	}
	composer.header(&source, "Subject", composer.encodedWord(values.Text(message["subject"])))
	identifier := values.Text(message["messageId"])
	if identifier == "" {
		identifier = messageId
	}
	composer.header(&source, "Message-ID", composer.messageId(identifier, from))
	if inReplyTo := values.Text(message["inReplyTo"]); inReplyTo != "" {
		composer.header(&source, "In-Reply-To", inReplyTo)
	}
	if references := fields.Strings(message["references"]); len(references) > 0 {
		composer.header(&source, "References", strings.Join(references, " "))
	}
	composer.header(&source, "Date", date.UTC().Format("Mon, 02 Jan 2006 15:04:05 -0700"))
	composer.header(&source, "MIME-Version", "1.0")
	composer.body(&source, message)
	return source.Bytes()
}

func (composer MimeMessage) body(source *bytes.Buffer, message map[string]any) {
	values := RecordValues{}
	inline, attached := []map[string]any{}, []map[string]any{}
	attachments, _ := values.List(message["attachments"])
	for _, item := range attachments {
		if attachment, isObject := values.Object(item); isObject {
			if attachment["disposition"] == "inline" && values.Text(attachment["contentId"]) != "" {
				inline = append(inline, attachment)
			} else {
				attached = append(attached, attachment)
			}
		}
	}
	parts := []string{}
	if text := values.Text(message["text"]); text != "" || values.Text(message["html"]) == "" {
		parts = append(parts, composer.textPart("text/plain", text))
	}
	if html := values.Text(message["html"]); html != "" {
		htmlPart := composer.textPart("text/html", html)
		if len(inline) > 0 {
			related := []string{htmlPart}
			for _, attachment := range inline {
				related = append(related, composer.attachmentPart(attachment))
			}
			htmlPart = composer.multipart("related", related)
		}
		parts = append(parts, htmlPart)
	}
	content := parts[0]
	if len(parts) > 1 {
		content = composer.multipart("alternative", parts)
	}
	if len(attached) > 0 {
		mixed := []string{content}
		for _, attachment := range attached {
			mixed = append(mixed, composer.attachmentPart(attachment))
		}
		content = composer.multipart("mixed", mixed)
	}
	source.WriteString(content)
}

// textPart answers one UTF-8 quoted-printable body part (headers and body).
func (MimeMessage) textPart(contentType string, text string) string {
	var encoded bytes.Buffer
	writer := quotedprintable.NewWriter(&encoded)
	writer.Write([]byte(strings.ReplaceAll(strings.ReplaceAll(text, "\r\n", "\n"), "\n", "\r\n")))
	writer.Close()
	return "Content-Type: " + contentType + "; charset=utf-8\r\nContent-Transfer-Encoding: quoted-printable\r\n\r\n" + encoded.String()
}

func (composer MimeMessage) attachmentPart(attachment map[string]any) string {
	values := RecordValues{}
	filename := values.Text(attachment["filename"])
	contentType := values.Text(attachment["contentType"])
	if contentType == "" {
		contentType = "application/octet-stream"
	}
	disposition := values.Text(attachment["disposition"])
	if disposition == "" {
		disposition = "attachment"
	}
	var part strings.Builder
	part.WriteString("Content-Type: " + mime.FormatMediaType(contentType, map[string]string{"name": filename}) + "\r\n")
	part.WriteString("Content-Transfer-Encoding: base64\r\n")
	part.WriteString("Content-Disposition: " + mime.FormatMediaType(disposition, map[string]string{"filename": filename}) + "\r\n")
	if contentId := values.Text(attachment["contentId"]); contentId != "" {
		part.WriteString("Content-ID: <" + contentId + ">\r\n")
	}
	part.WriteString("\r\n")
	encoded := base64.StdEncoding.EncodeToString(MailValidation{}.Decoded(values.Text(attachment["contentBase64"])))
	for len(encoded) > composer.lineWidth() {
		part.WriteString(encoded[:composer.lineWidth()] + "\r\n")
		encoded = encoded[composer.lineWidth():]
	}
	part.WriteString(encoded + "\r\n")
	return part.String()
}

// multipart answers a multipart entity of the parts (each headers and body).
func (composer MimeMessage) multipart(subtype string, parts []string) string {
	boundary := composer.boundary()
	var entity strings.Builder
	entity.WriteString("Content-Type: multipart/" + subtype + "; boundary=\"" + boundary + "\"\r\n\r\n")
	for _, part := range parts {
		entity.WriteString("--" + boundary + "\r\n" + part)
		if !strings.HasSuffix(part, "\r\n") {
			entity.WriteString("\r\n")
		}
	}
	entity.WriteString("--" + boundary + "--\r\n")
	return entity.String()
}

func (MimeMessage) boundary() string {
	random := make([]byte, 12)
	_, _ = rand.Read(random)
	return "--_CaduceusPart_" + hex.EncodeToString(random)
}

// header writes one header, folded at spaces so no line passes 78
// characters where a space allows.
func (composer MimeMessage) header(source *bytes.Buffer, name string, value string) {
	clean := regexp.MustCompile(`[\r\n]+`).ReplaceAllString(value, " ")
	line := name + ":"
	for _, word := range strings.Split(clean, " ") {
		if len(line)+1+len(word) > composer.foldWidth() && strings.TrimSpace(line) != name+":" {
			source.WriteString(line + "\r\n")
			line = ""
		}
		line += " " + word
	}
	source.WriteString(line + "\r\n")
}

// encodedWord answers text as RFC 2047 encoded words when it is not plain
// ASCII.
func (MimeMessage) encodedWord(text string) string {
	for _, character := range text {
		if character > 126 || (character < 32 && character != '\t') {
			return mime.QEncoding.Encode("utf-8", text)
		}
	}
	return text
}

// messageId answers the bracketed Message-ID: the value itself when it
// names a domain, else the value (or a fresh token) at the sender's domain.
func (MimeMessage) messageId(value string, from string) string {
	clean := Security{}.Trim(regexp.MustCompile(`[<>\r\n]`).ReplaceAllString(value, ""))
	if strings.Contains(clean, "@") {
		return "<" + clean + ">"
	}
	domain := DeliveryFields{}.Domain(from)
	if domain == "" {
		domain = "localhost"
	}
	if clean == "" {
		clean = "caduceus-" + strconv.FormatInt(EpochTime{}.Now(), 10)
	}
	return "<" + clean + "@" + domain + ">"
}

func (MimeMessage) lineWidth() int { return 76 }

func (MimeMessage) foldWidth() int { return 78 }
