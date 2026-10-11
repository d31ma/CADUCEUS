import (
	"encoding/base64"
	"fmt"
	"regexp"
	"strings"
)

// MailValidation checks outbound send requests: recipients, subject,
// tracking switches, threading headers and attachments. Each check answers
// the normalized value, or the message the route answers with a 400.
type MailValidation struct{}

// SendRequest answers the normalized request fields of a send body (from,
// to, cc, bcc, replyTo, inReplyTo, references, subject, text, html,
// openTracking, linkTracking), or the reason it is invalid.
func (validation MailValidation) SendRequest(value any) (map[string]any, string) {
	body, isBody := value.(map[string]any)
	if !isBody {
		return nil, "Request body required"
	}
	subject, _ := body["subject"].(string)
	if (Security{}).Trim(subject) == "" {
		return nil, "Subject is required"
	}
	if validation.headerInjection(subject) {
		return nil, "Subject contains invalid characters"
	}
	to, problem := validation.recipients(body["to"])
	if problem != "" {
		return nil, problem
	}
	if len(to) > validation.mostToRecipients() {
		return nil, validation.tooMany()
	}
	values := RecordValues{}
	if !values.Truthy(body["text"]) && !values.Truthy(body["html"]) {
		return nil, "text or html required"
	}
	cc, problem := validation.optionalRecipients(body["cc"], "cc")
	if problem != "" {
		return nil, problem
	}
	bcc, problem := validation.optionalRecipients(body["bcc"], "bcc")
	if problem != "" {
		return nil, problem
	}
	if len(to)+len(cc)+len(bcc) > validation.mostRecipients() {
		return nil, validation.tooMany()
	}
	for _, field := range []string{"openTracking", "linkTracking"} {
		if _, isBool := body[field].(bool); body[field] != nil && !isBool {
			return nil, field + " must be a boolean"
		}
	}
	if (body["openTracking"] == true || body["linkTracking"] == true) && !values.Truthy(body["html"]) {
		return nil, "Tracking requires an HTML body"
	}
	request := map[string]any{"to": DeliveryFields{}.Items(to), "subject": Security{}.Trim(subject)}
	if problem := validation.headers(body, request); problem != "" {
		return nil, problem
	}
	if len(cc) > 0 {
		request["cc"] = DeliveryFields{}.Items(cc)
	}
	if len(bcc) > 0 {
		request["bcc"] = DeliveryFields{}.Items(bcc)
	}
	for _, field := range []string{"text", "html"} {
		if values.Truthy(body[field]) {
			request[field] = body[field]
		}
	}
	for _, field := range []string{"openTracking", "linkTracking"} {
		if body[field] == true {
			request[field] = true
		}
	}
	return request, ""
}

// headers checks from, replyTo, inReplyTo and references into request.
func (validation MailValidation) headers(body map[string]any, request map[string]any) string {
	if body["from"] != nil {
		from, _ := body["from"].(string)
		normalized := Security{}.NormalizeEmailAddress(from)
		if normalized == "" || validation.ControlCharacters(from) {
			return "from must be a valid owned email address"
		}
		request["from"] = normalized
	}
	if body["replyTo"] != nil {
		list, isList := RecordValues{}.List(body["replyTo"])
		if !isList || len(list) != 1 {
			return "replyTo must contain exactly one email address"
		}
		text, _ := list[0].(string)
		normalized := Security{}.NormalizeEmailAddress(text)
		if normalized == "" || validation.ControlCharacters(RecordValues{}.String(list[0])) {
			return "replyTo must contain a valid email address"
		}
		request["replyTo"] = []any{normalized}
	}
	threading := Threading{}
	if body["inReplyTo"] != nil {
		inReplyTo := threading.NormalizeMessageId(body["inReplyTo"])
		if inReplyTo == "" {
			return "inReplyTo must be a valid Message-ID"
		}
		request["inReplyTo"] = inReplyTo
	}
	if body["references"] != nil {
		list, isList := RecordValues{}.List(body["references"])
		if !isList || len(list) > validation.mostReferences() || len(DeliveryFields{}.Strings(list)) != len(list) {
			return "references must be an array of at most 100 Message-IDs"
		}
		references := threading.ParseMessageIdList(list)
		if len(references) != len(list) {
			return "references must contain valid unique Message-IDs"
		}
		if len(references) > 0 {
			request["references"] = DeliveryFields{}.Items(references)
		}
	}
	return ""
}

// Attachments answers the normalized attachments of a send or draft body
// ({filename, contentType, contentBase64, disposition, contentId?}), or the
// reason they are invalid.
func (validation MailValidation) Attachments(value any) ([]any, string) {
	if value == nil {
		return []any{}, ""
	}
	items, isList := RecordValues{}.List(value)
	if !isList {
		return nil, "attachments must be an array"
	}
	if len(items) > validation.mostAttachments() {
		return nil, fmt.Sprintf("Too many attachments (max %d)", validation.mostAttachments())
	}
	total := 0
	attachments := []any{}
	contentIds := map[string]bool{}
	for _, item := range items {
		attachment, size, problem := validation.attachment(item, total)
		if problem != "" {
			return nil, problem
		}
		if contentId, _ := attachment["contentId"].(string); contentId != "" {
			if contentIds[strings.ToLower(contentId)] {
				return nil, "Attachment contentId values must be unique"
			}
			contentIds[strings.ToLower(contentId)] = true
		}
		total += size
		attachments = append(attachments, attachment)
	}
	return attachments, ""
}

func (validation MailValidation) attachment(item any, totalSoFar int) (map[string]any, int, string) {
	fields, isObject := RecordValues{}.Object(item)
	if !isObject {
		return nil, 0, "Invalid attachment"
	}
	filename, _ := fields["filename"].(string)
	if (Security{}).Trim(filename) == "" {
		return nil, 0, "Attachment filename is required"
	}
	if validation.ControlCharacters(filename) {
		return nil, 0, "Attachment filename contains invalid characters"
	}
	content, _ := fields["contentBase64"].(string)
	if (Security{}).Trim(content) == "" {
		return nil, 0, "Attachment contentBase64 is required"
	}
	if !regexp.MustCompile(`^[A-Za-z0-9+/]+={0,2}$`).MatchString(content) {
		return nil, 0, "Attachment contentBase64 must be valid base64"
	}
	contentType := "application/octet-stream"
	if text, _ := fields["contentType"].(string); (Security{}).Trim(text) != "" {
		contentType = strings.ToLower(Security{}.Trim(text))
	}
	if !regexp.MustCompile(`^[a-z]+/[a-z0-9.+-]+$`).MatchString(contentType) {
		return nil, 0, "Attachment contentType is invalid"
	}
	contentId := ""
	if fields["contentId"] != nil && fields["contentId"] != "" {
		contentId = AttachmentPolicy{}.NormalizeAttachmentContentId(RecordValues{}.String(fields["contentId"]))
		if contentId == "" {
			return nil, 0, "Attachment contentId is invalid"
		}
	}
	size := len(validation.Decoded(content))
	switch {
	case size <= 0:
		return nil, 0, "Attachment content is empty"
	case size > validation.largestAttachment():
		return nil, 0, "Attachment is too large (max 10MB)"
	case totalSoFar+size > validation.largestTotal():
		return nil, 0, "Attachments are too large (max 25MB total)"
	}
	disposition := "attachment"
	if fields["disposition"] == "inline" {
		disposition = "inline"
	}
	attachment := map[string]any{
		"filename":      AttachmentPolicy{}.SanitizeAttachmentFilename(filename, "attachment"),
		"contentType":   contentType,
		"contentBase64": content,
		"disposition":   disposition,
	}
	if contentId != "" {
		attachment["contentId"] = contentId
	}
	return attachment, size, ""
}

// Decoded answers base64 text as bytes, as Node's lenient decoder read it.
func (MailValidation) Decoded(text string) []byte {
	trimmed := strings.TrimRight(strings.Map(func(letter rune) rune {
		if strings.ContainsRune(" \t\r\n", letter) {
			return -1
		}
		return letter
	}, text), "=")
	decoded, err := base64.RawStdEncoding.DecodeString(trimmed)
	if err != nil {
		decoded, _ = base64.RawStdEncoding.DecodeString(trimmed[:len(trimmed)-len(trimmed)%4])
	}
	return decoded
}

func (validation MailValidation) recipients(value any) ([]string, string) {
	list, isList := RecordValues{}.List(value)
	if !isList || len(list) == 0 {
		return nil, "Missing to field"
	}
	if len(list) > validation.mostRecipients() {
		return nil, validation.tooMany()
	}
	valid := []string{}
	for _, item := range list {
		text, isText := item.(string)
		if !isText || (Security{}).Trim(text) == "" {
			return nil, "Invalid recipient"
		}
		normalized := Security{}.NormalizeEmailAddress(text)
		if normalized == "" || validation.ControlCharacters(normalized) {
			return nil, "Invalid email: " + text
		}
		valid = append(valid, normalized)
	}
	return valid, ""
}

func (validation MailValidation) optionalRecipients(value any, field string) ([]string, string) {
	if value == nil {
		return []string{}, ""
	}
	list, isList := RecordValues{}.List(value)
	if !isList {
		return nil, field + " must be an array"
	}
	if len(list) > validation.mostRecipients() {
		return nil, validation.tooMany()
	}
	normalized := []string{}
	for _, item := range list {
		text, isText := item.(string)
		address := Security{}.NormalizeEmailAddress(text)
		if !isText || address == "" || validation.ControlCharacters(text) {
			return nil, field + " must contain valid email addresses"
		}
		normalized = append(normalized, address)
	}
	return normalized, ""
}

// ControlCharacters reports whether text holds NUL, CR or LF.
func (MailValidation) ControlCharacters(text string) bool {
	return strings.ContainsAny(text, "\x00\r\n")
}

func (MailValidation) headerInjection(subject string) bool {
	pattern := `(?im)^(?:bcc|cc|to|from|reply-to|sender|return-path|message-id|mime-version|content-type|content-transfer-encoding):`
	return regexp.MustCompile(pattern).MatchString(Security{}.Trim(subject))
}

func (validation MailValidation) tooMany() string {
	return fmt.Sprintf("Too many recipients (max %d)", validation.mostRecipients())
}

func (MailValidation) mostAttachments() int { return 10 }

func (MailValidation) largestAttachment() int { return 10 * 1024 * 1024 }

func (MailValidation) largestTotal() int { return 25 * 1024 * 1024 }

func (MailValidation) mostRecipients() int { return 100 }

func (MailValidation) mostToRecipients() int { return 50 }

func (MailValidation) mostReferences() int { return 100 }
