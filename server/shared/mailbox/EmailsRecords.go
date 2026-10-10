import (
	"encoding/json"
	"sort"
	"strings"
)

// EmailsRecords normalises stored emails and decides which mailbox may see
// them and which list filters they match.
type EmailsRecords struct{}

// CanAccessEmail enforces exact mailbox ownership below every caller query.
// A stored ownerEmail is authoritative; historical records without one
// belong to the sender (sent), the draft owner or sender (drafts), or a
// visible recipient (every other folder). Ambiguous legacy rows fail closed.
func (records EmailsRecords) CanAccessEmail(email map[string]any, ownerEmail string) bool {
	owner := records.NormalizeAddress(ownerEmail)
	if owner == "" {
		return false
	}
	text := TextValue{}
	if email["ownerEmail"] != nil && text.Trim(text.Of(email["ownerEmail"])) != "" {
		explicit := records.NormalizeAddress(email["ownerEmail"])
		return explicit != "" && explicit == owner
	}
	switch strings.ToLower(text.Of(text.Or(email["folder"], "inbox"))) {
	case "drafts":
		return records.NormalizeAddress(email["draftOwner"]) == owner || records.NormalizeAddress(email["sender"]) == owner
	case "sent":
		return records.NormalizeAddress(email["sender"]) == owner
	}
	for _, address := range records.RecipientAddresses(email) {
		if address == owner {
			return true
		}
	}
	return false
}

// RecipientAddresses answers the unique envelope and visible recipients.
func (EmailsRecords) RecipientAddresses(email map[string]any) []string {
	values := []any{email["originalRecipient"], email["recipient"]}
	for _, field := range []string{"to", "cc", "bcc"} {
		values = append(values, TextValue{}.List(email[field])...)
	}
	addresses := MailAddresses{}
	found := []string{}
	for _, value := range values {
		found = append(found, addresses.Found(value)...)
	}
	return addresses.Unique(found)
}

// NormalizeAddress answers the one address in String(value), lower-cased,
// or "" when it holds none or several.
func (EmailsRecords) NormalizeAddress(value any) string {
	found := MailAddresses{}.Found(value)
	if len(found) != 1 {
		return ""
	}
	return found[0]
}

// Normalize answers a stored email as routes see it: default folder and
// flags, unique lower-case labels, tracking lists decoded from their JSON
// text and empty spam fields removed.
func (records EmailsRecords) Normalize(email map[string]any) map[string]any {
	text := TextValue{}
	normalized := text.Copy(email)
	normalized["folder"] = text.Or(email["folder"], "inbox")
	normalized["read"] = records.orFalse(email["read"])
	normalized["starred"] = records.orFalse(email["starred"])
	labels := []string{}
	if list, isList := email["labels"].([]any); isList {
		for _, label := range list {
			labels = append(labels, strings.ToLower(text.Of(label)))
		}
	}
	normalized["labels"] = MailAddresses{}.Unique(labels)
	normalized["trackedLinks"] = records.ParseJsonArray(normalized["trackedLinksJson"])
	normalized["opens"] = records.ParseJsonArray(normalized["opensJson"])
	normalized["clicks"] = records.ParseJsonArray(text.Or(normalized["clicksJson"], normalized["clicks"]))
	delete(normalized, "trackedLinksJson")
	delete(normalized, "opensJson")
	delete(normalized, "clicksJson")
	for _, field := range []string{"spamScore", "spamReasons"} {
		if normalized[field] == nil {
			delete(normalized, field)
		}
	}
	return normalized
}

func (EmailsRecords) orFalse(value any) any {
	if value == nil {
		return false
	}
	return value
}

// ParseJsonArray answers value when it is a list, the list its JSON text
// holds, or an empty list.
func (EmailsRecords) ParseJsonArray(value any) []any {
	if list, isList := value.([]any); isList {
		return list
	}
	encoded, isText := value.(string)
	if !isText || encoded == "" {
		return []any{}
	}
	var parsed any
	if json.Unmarshal([]byte(encoded), &parsed) != nil {
		return []any{}
	}
	if list, isList := parsed.([]any); isList {
		return list
	}
	return []any{}
}

// MatchesFolder reports whether an email is in the selected folder (the
// domain's own selection first; none or "all" match every folder).
func (EmailsRecords) MatchesFolder(email map[string]any, folder string, folderByDomain map[string]string) bool {
	text := TextValue{}
	selected := folderByDomain[text.Of(email["domain"])]
	if selected == "" {
		selected = folder
	}
	return selected == "" || selected == "all" || text.Of(text.Or(email["folder"], "inbox")) == selected
}

// MatchesHasAttachment reports whether an email has attachments when the
// filter asks (nil matches every email).
func (EmailsRecords) MatchesHasAttachment(email map[string]any, hasAttachment *bool, attachmentsByEmail map[string][]map[string]any) bool {
	if hasAttachment == nil {
		return true
	}
	return (len(attachmentsByEmail[TextValue{}.Of(email["id"])]) > 0) == *hasAttachment
}

// MatchesQuery reports whether an email matches every term of a search.
func (records EmailsRecords) MatchesQuery(email map[string]any, query string, attachments []map[string]any) bool {
	text := TextValue{}
	for _, term := range text.Words(strings.ToLower(text.Trim(query))) {
		if !records.matchesTerm(email, term, attachments) {
			return false
		}
	}
	return true
}

func (records EmailsRecords) matchesTerm(email map[string]any, term string, attachments []map[string]any) bool {
	text := TextValue{}
	switch term {
	case "has:attachment":
		return len(attachments) > 0
	case "is:read":
		return text.Truthy(email["read"])
	case "is:unread":
		return !text.Truthy(email["read"])
	case "is:starred":
		return text.Truthy(email["starred"])
	}
	if field, value, found := strings.Cut(term, ":"); found && field != "" && value != "" {
		if matched, known := records.matchesField(email, field, value, attachments); known {
			return matched
		}
	}
	for _, field := range []string{"sender", "recipient", "subject", "body"} {
		if records.includes(email[field], term) {
			return true
		}
	}
	return records.attachmentNamed(attachments, term)
}

func (records EmailsRecords) matchesField(email map[string]any, field string, value string, attachments []map[string]any) (bool, bool) {
	fields := map[string]string{"from": "sender", "to": "recipient", "subject": "subject", "body": "body"}
	if stored, known := fields[field]; known {
		return records.includes(email[stored], value), true
	}
	if field == "filename" || field == "attachment" {
		return records.attachmentNamed(attachments, value), true
	}
	return false, false
}

func (records EmailsRecords) attachmentNamed(attachments []map[string]any, value string) bool {
	for _, attachment := range attachments {
		if records.includes(attachment["filename"], value) {
			return true
		}
	}
	return false
}

func (EmailsRecords) includes(value any, term string) bool {
	return strings.Contains(strings.ToLower(TextValue{}.Of(value)), term)
}

// Listed answers the normalised stored emails of the allowed domains that
// match the filters, newest first, after the filter's offset and within its
// limit.
func (records EmailsRecords) Listed(stored []FyloRecord, allowedDomains []string, filters EmailListFilters, attachmentsByEmail map[string][]map[string]any) []map[string]any {
	emails := []map[string]any{}
	for _, record := range stored {
		email := records.Normalize(record.Data)
		if records.matches(email, allowedDomains, filters, attachmentsByEmail) {
			emails = append(emails, email)
		}
	}
	text := TextValue{}
	sort.SliceStable(emails, func(left, right int) bool {
		return text.Of(emails[left]["receivedAt"]) > text.Of(emails[right]["receivedAt"])
	})
	offset := min(max(0, filters.Offset), len(emails))
	if filters.Limit == nil {
		return emails[offset:]
	}
	return emails[offset:min(len(emails), offset+max(0, *filters.Limit))]
}

func (records EmailsRecords) matches(email map[string]any, allowedDomains []string, filters EmailListFilters, attachmentsByEmail map[string][]map[string]any) bool {
	text := TextValue{}
	return text.Includes(allowedDomains, email["domain"]) &&
		(filters.OwnerEmail == "" || records.CanAccessEmail(email, filters.OwnerEmail)) &&
		records.MatchesFolder(email, filters.Folder, filters.FolderByDomain) &&
		(filters.Read == nil || email["read"] == *filters.Read) &&
		(filters.Starred == nil || email["starred"] == *filters.Starred) &&
		records.MatchesHasAttachment(email, filters.HasAttachment, attachmentsByEmail) &&
		records.MatchesQuery(email, filters.Query, attachmentsByEmail[text.Of(email["id"])]) &&
		(filters.Since == "" || text.Of(email["receivedAt"]) > filters.Since)
}

// Matched answers the stored records whose normalised email has one of the
// logical ids, as patches of fields, and those emails as they were.
func (records EmailsRecords) Matched(stored []FyloRecord, ids []string, fields map[string]any) ([]FyloRecord, []map[string]any) {
	patches := []FyloRecord{}
	matched := []map[string]any{}
	for _, record := range stored {
		email := records.Normalize(record.Data)
		if (TextValue{}).Includes(ids, email["id"]) {
			patches = append(patches, FyloRecord{Id: record.Id, Data: fields})
			matched = append(matched, email)
		}
	}
	return patches, matched
}
