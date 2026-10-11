import (
	"encoding/base64"
	"encoding/json"
	"regexp"
	"sort"
	"strconv"
	"strings"
)

// ExternalMessage maps connected-account (provider) messages onto the
// mailbox contract: opaque route identities, index rows as inbox rows, and
// an authorized RFC 822 source as the reader's detail.
type ExternalMessage struct{}

func (ExternalMessage) prefix() string { return "extmsg." }

func (ExternalMessage) longestSource() int { return 35_000_000 }

func (ExternalMessage) largestMessage() int { return 25 * 1024 * 1024 }

func (ExternalMessage) mostAttachments() int { return 100 }

// IsAccountId reports whether value is a connected account identifier.
func (ExternalMessage) IsAccountId(value any) bool {
	text, isText := value.(string)
	return isText && regexp.MustCompile(`^ext_[A-Za-z0-9_-]{20,80}$`).MatchString(text)
}

func (ExternalMessage) validStableId(value any) bool {
	text, isText := value.(string)
	return isText && len(text) >= 1 && TextValue{}.Length(text) <= 512 && !strings.ContainsAny(text, "\x00\r\n")
}

// Id answers the opaque route identity of one provider message, or "" for
// an identity that is not valid.
func (message ExternalMessage) Id(accountId any, stableId any) string {
	if !message.IsAccountId(accountId) || !message.validStableId(stableId) {
		return ""
	}
	encoded := TextValue{}.Json(map[string]any{"accountId": accountId, "stableId": stableId})
	return message.prefix() + base64.RawURLEncoding.EncodeToString([]byte(encoded))
}

// ParseId answers the account and stable id of an opaque route identity,
// or false.
func (message ExternalMessage) ParseId(value any) (string, string, bool) {
	text, isText := value.(string)
	if !isText || !strings.HasPrefix(text, message.prefix()) || (TextValue{}).Length(text) > 2048 {
		return "", "", false
	}
	decoded, err := base64.RawURLEncoding.DecodeString(strings.TrimRight(text[len(message.prefix()):], "="))
	if err != nil {
		return "", "", false
	}
	fields := map[string]any{}
	if json.Unmarshal(decoded, &fields) != nil || len(fields) != 2 {
		return "", "", false
	}
	if !message.IsAccountId(fields["accountId"]) || !message.validStableId(fields["stableId"]) {
		return "", "", false
	}
	return fields["accountId"].(string), fields["stableId"].(string), true
}

// IsRoute reports whether value is an external message route identity.
func (message ExternalMessage) IsRoute(value string) bool {
	_, _, valid := message.ParseId(value)
	return valid
}

// Summary answers one provider index row as an inbox row, or nil.
func (message ExternalMessage) Summary(row map[string]any) map[string]any {
	text := TextValue{}
	id := message.Id(row["accountId"], row["stableId"])
	if id == "" {
		return nil
	}
	flags := map[string]bool{}
	for _, flag := range text.List(row["flags"]) {
		flags[strings.ToLower(text.Of(flag))] = true
	}
	folder := strings.ToLower(text.Of(text.Or(row["folderRole"], text.Or(row["folderName"], "inbox"))))
	mailboxAddress := strings.ToLower(text.Of(text.Or(row["mailboxAddress"], "")))
	domain := ""
	if parts := strings.Split(mailboxAddress, "@"); len(parts) > 1 {
		domain = parts[1]
	}
	to := message.list(row["to"])
	recipient := any(mailboxAddress)
	if first := text.List(row["to"]); len(first) > 0 && text.Truthy(first[0]) {
		recipient = first[0]
	}
	size := text.Number(row["size"])
	if size != float64(int64(size)) || size > 9007199254740991 || size < -9007199254740991 {
		size = 0
	}
	names := message.list(row["attachmentNames"])
	return map[string]any{
		"id": id, "external": true, "providerAccountId": row["accountId"], "providerStableId": row["stableId"],
		"mailboxAddress": mailboxAddress, "mailboxLabel": text.Or(row["accountLabel"], mailboxAddress),
		"mailboxColor": text.Or(row["accountColor"], "#64748b"), "domain": domain,
		"sender": text.Or(row["sender"], ""), "recipient": recipient, "to": to,
		"cc": message.list(row["cc"]), "bcc": message.list(row["bcc"]),
		"subject": text.Or(row["subject"], ""), "snippet": text.Or(row["snippet"], ""),
		"messageId": text.Or(row["rfcMessageId"], ""), "rfcMessageId": text.Or(row["rfcMessageId"], ""),
		"inReplyTo": text.Or(row["inReplyTo"], ""), "references": message.list(row["references"]),
		"folder": folder, "systemFolder": folder, "read": flags[`\seen`], "starred": flags[`\flagged`],
		"receivedAt": row["receivedAt"], "rawSize": size,
		"hasAttachments": len(text.List(row["attachmentNames"])) > 0, "attachmentNames": names,
		"labels": message.labels(row["labels"], folder),
	}
}

// list answers `value || []`.
func (ExternalMessage) list(value any) any {
	if (TextValue{}).Truthy(value) {
		return value
	}
	return []any{}
}

func (ExternalMessage) labels(value any, folder string) []string {
	text := TextValue{}
	if list, isList := value.([]any); isList {
		labels := []string{}
		for _, label := range list {
			if trimmed := text.Trim(text.Of(text.Or(label, ""))); trimmed != "" {
				labels = append(labels, trimmed)
			}
		}
		return MailAddresses{}.Unique(labels)
	}
	if folder != "" && !(CategoryMappings{}).IsRole(folder) {
		return []string{folder}
	}
	return []string{}
}

// source answers the bytes of an authorized base64 source, or false when it
// is missing or out of bounds.
func (message ExternalMessage) source(encoded any) ([]byte, bool) {
	text, isText := encoded.(string)
	if !isText || len(text) > message.longestSource() {
		return nil, false
	}
	raw := Base64Text{}.Decoded(text)
	if len(raw) < 1 || len(raw) > message.largestMessage() {
		return nil, false
	}
	return raw, true
}

// Detail answers the reader's view of one authorized provider message from
// its index row and source, or nil.
func (message ExternalMessage) Detail(row map[string]any, encoded any) map[string]any {
	summary := message.Summary(row)
	raw, valid := message.source(encoded)
	if summary == nil || !valid {
		return nil
	}
	parsed := MimeParser{}.Parse(raw)
	headers := parsed.Headers
	flattened := headers.Flattened()
	attachments := []map[string]any{}
	for index, attachment := range parsed.Attachments {
		if index >= message.mostAttachments() {
			break
		}
		disposition := "attachment"
		if attachment.Disposition == "inline" {
			disposition = "inline"
		}
		item := map[string]any{
			"id": "extatt." + strconv.Itoa(index), "filename": message.portableFilename(attachment.Filename),
			"contentType": TextValue{}.Prefix(TextValue{}.Of(TextValue{}.Or(attachment.ContentType, "application/octet-stream")), 255),
			"size": len(attachment.Content), "disposition": disposition,
		}
		if attachment.ContentId != "" {
			item["contentId"] = TextValue{}.Prefix(strings.TrimSuffix(strings.TrimPrefix(attachment.ContentId, "<"), ">"), 998)
		}
		attachments = append(attachments, item)
	}
	detail := TextValue{}.Copy(summary)
	if from := headers.Get("from"); from != "" {
		detail["sender"] = headers.Decoded(from)
	}
	for _, field := range []string{"to", "cc", "bcc"} {
		detail[field] = message.addresses(headers, field)
	}
	if subject := headers.Decoded(headers.Get("subject")); subject != "" {
		detail["subject"] = subject
	}
	detail["body"] = parsed.Text()
	if html, found := parsed.Html(); found {
		detail["htmlBody"] = html
	}
	if replyTo := message.addresses(headers, "reply-to"); len(replyTo) > 0 {
		detail["replyTo"] = replyTo
	}
	detail["headers"] = flattened
	domain := TextValue{}.Of(TextValue{}.Or(summary["domain"], "external.invalid"))
	for key, value := range (Threading{}).ThreadMetadataFromHeaders(flattened, (TextValue{}).Of(summary["id"]), domain) {
		detail[key] = value
	}
	detail["hasRawSource"] = true
	detail["hasAttachments"] = len(attachments) > 0
	detail["attachments"] = attachments
	return detail
}

func (ExternalMessage) addresses(headers MimeHeaders, field string) []string {
	found := []string{}
	for index, name := range headers.Names {
		if name == field {
			found = append(found, headers.Addresses(headers.Values[index])...)
		}
	}
	unique := MailAddresses{}.Unique(found)
	if len(unique) > 100 {
		unique = unique[:100]
	}
	return unique
}

// Attachment answers one attachment of an authorized provider source, with
// its bytes in base64, or nil.
func (message ExternalMessage) Attachment(encoded any, attachmentId string) map[string]any {
	match := regexp.MustCompile(`^extatt\.(\d{1,3})$`).FindStringSubmatch(attachmentId)
	raw, valid := message.source(encoded)
	if match == nil || !valid {
		return nil
	}
	index, _ := strconv.Atoi(match[1])
	attachments := MimeParser{}.Parse(raw).Attachments
	if index >= len(attachments) || index >= message.mostAttachments() {
		return nil
	}
	attachment := attachments[index]
	return map[string]any{
		"id": attachmentId, "filename": message.portableFilename(attachment.Filename),
		"contentType": TextValue{}.Prefix(TextValue{}.Of(TextValue{}.Or(attachment.ContentType, "application/octet-stream")), 255),
		"size": len(attachment.Content), "contentBase64": base64.StdEncoding.EncodeToString(attachment.Content),
	}
}

func (ExternalMessage) portableFilename(value string) string {
	if value == "" {
		value = "attachment"
	}
	parts := strings.Split(strings.ReplaceAll(value, `\`, "/"), "/")
	leaf := TextValue{}.Trim(parts[len(parts)-1])
	if leaf == "" {
		leaf = "attachment"
	}
	leaf = TextValue{}.Prefix(strings.NewReplacer("\x00", "", "\r", "", "\n", "").Replace(leaf), 255)
	if leaf == "" {
		return "attachment"
	}
	return leaf
}

// AttachmentSummaries answers the summaries an index row's attachment names
// give, for search and conversation views.
func (ExternalMessage) AttachmentSummaries(email map[string]any) []map[string]any {
	summaries := []map[string]any{}
	for index, name := range (TextValue{}).List(email["attachmentNames"]) {
		summaries = append(summaries, map[string]any{
			"id": "extatt." + strconv.Itoa(index), "filename": name, "contentType": "application/octet-stream", "size": 0,
		})
	}
	return summaries
}

// Merged answers hosted and provider rows newest first (ties by id), after
// offset and within limit (a negative limit keeps every row).
func (ExternalMessage) Merged(hosted []map[string]any, external []map[string]any, offset int, limit int) []map[string]any {
	text := TextValue{}
	order := TextOrder{}
	merged := append(append([]map[string]any{}, hosted...), external...)
	sort.SliceStable(merged, func(left, right int) bool {
		if time := order.Compare(text.Of(text.Or(merged[right]["receivedAt"], "")), text.Of(text.Or(merged[left]["receivedAt"], ""))); time != 0 {
			return time < 0
		}
		return order.Compare(text.Of(text.Or(merged[left]["id"], "")), text.Of(text.Or(merged[right]["id"], ""))) < 0
	})
	start := min(max(0, offset), len(merged))
	if limit < 0 {
		return merged[start:]
	}
	return merged[start:min(len(merged), start+limit)]
}
