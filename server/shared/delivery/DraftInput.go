import (
	"regexp"
	"strings"
)

// DraftInput validates and normalizes one complete draft autosave snapshot.
type DraftInput struct{}

// Parsed answers the normalized snapshot ({clientId, from?, to, cc, bcc,
// replyTo, subject, text, html, inReplyTo?, sourceEmailId?, references,
// attachments}), or the reason it is invalid.
func (input DraftInput) Parsed(value any) (map[string]any, string) {
	body, isObject := value.(map[string]any)
	if !isObject {
		return nil, "Draft payload required"
	}
	security := Security{}
	clientIdText, _ := body["clientId"].(string)
	clientId := security.Trim(clientIdText)
	if clientId == "" || len(clientId) > 120 || !regexp.MustCompile(`^[A-Za-z0-9._:-]+$`).MatchString(clientId) {
		return nil, "clientId is required"
	}
	draft := map[string]any{"clientId": clientId}
	for _, field := range []string{"to", "cc", "bcc", "replyTo"} {
		maximum := 100
		if field == "replyTo" {
			maximum = 1
		}
		addresses, problem := input.addresses(body[field], field, maximum)
		if problem != "" {
			return nil, problem
		}
		draft[field] = addresses
	}
	if body["from"] != nil {
		text, _ := body["from"].(string)
		from := security.NormalizeEmailAddress(text)
		if from == "" || (MailValidation{}).ControlCharacters(RecordValues{}.String(body["from"])) {
			return nil, "from must be a valid owned email address"
		}
		draft["from"] = from
	}
	subjectText, _ := body["subject"].(string)
	subject := security.Trim(subjectText)
	if (TextValue{}).Length(subject) > 500 || strings.ContainsAny(subject, "\x00\r\n") {
		return nil, "subject is invalid"
	}
	text, _ := body["text"].(string)
	html, _ := body["html"].(string)
	if len(text)+len(html) > 5*1024*1024 {
		return nil, "Draft body is too large (max 5MB)"
	}
	draft["subject"], draft["text"], draft["html"] = subject, text, html
	if problem := input.threading(body, draft); problem != "" {
		return nil, problem
	}
	attachmentsInput := RecordValues{}.Coalesce(body["attachments"], []any{})
	attachments, problem := MailValidation{}.Attachments(attachmentsInput)
	if problem != "" {
		return nil, problem
	}
	draft["attachments"] = attachments
	return draft, ""
}

func (DraftInput) threading(body map[string]any, draft map[string]any) string {
	threading := Threading{}
	if body["inReplyTo"] != nil {
		inReplyTo := threading.NormalizeMessageId(body["inReplyTo"])
		if inReplyTo == "" {
			return "inReplyTo must be a valid Message-ID"
		}
		draft["inReplyTo"] = inReplyTo
	}
	draft["references"] = []any{}
	if body["references"] != nil {
		list, isList := RecordValues{}.List(body["references"])
		if !isList || len(list) > 100 || len(DeliveryFields{}.Strings(list)) != len(list) {
			return "references must be an array of at most 100 Message-IDs"
		}
		references := threading.ParseMessageIdList(list)
		if len(references) != len(list) {
			return "references must contain valid unique Message-IDs"
		}
		draft["references"] = DeliveryFields{}.Items(references)
	}
	if body["sourceEmailId"] != nil {
		source, isText := body["sourceEmailId"].(string)
		if !isText || (Security{}).Trim(source) == "" || len(source) > 200 || strings.ContainsAny(source, "\r\n") {
			return "sourceEmailId is invalid"
		}
		draft["sourceEmailId"] = Security{}.Trim(source)
	}
	return ""
}

func (DraftInput) addresses(value any, name string, maximum int) ([]any, string) {
	if value == nil {
		return []any{}, ""
	}
	list, isList := RecordValues{}.List(value)
	if !isList || len(list) > maximum {
		return nil, name + " must be an array of at most " + strings.TrimSpace(RecordValues{}.String(float64(maximum))) + " addresses"
	}
	normalized := []string{}
	for _, item := range list {
		text, _ := item.(string)
		address := Security{}.NormalizeEmailAddress(text)
		if address == "" || (MailValidation{}).ControlCharacters(RecordValues{}.String(item)) {
			return nil, name + " contains an invalid email address"
		}
		if !(TextValue{}).Includes(normalized, address) {
			normalized = append(normalized, address)
		}
	}
	return DeliveryFields{}.Items(normalized), ""
}
