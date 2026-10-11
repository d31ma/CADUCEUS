import (
	"errors"
	"fmt"
	"regexp"
	"strings"
)

// McpDrafts creates the durable drafts the create_draft and reply_to_thread
// tools make. It is a private copy of the parts of the send area's
// DraftInput and DraftService.create those tools need (no attachments, no
// Reply-To) until that area is merged.
type McpDrafts struct {
	Users  *UsersHelper
	Emails *EmailsHelper
	Fylo   *FyloHelper
}

// Create stores a draft for the principal from tool input and answers
// {draftId, version, recovered}.
func (drafts McpDrafts) Create(principal McpPrincipal, source map[string]any) (any, error) {
	request := map[string]any{"clientId": "mcp-" + CapabilityToken{}.Random(16), "to": []any{}, "cc": []any{}, "bcc": []any{}, "subject": "", "text": "", "html": ""}
	for key, value := range source {
		request[key] = value
	}
	input, err := drafts.parsed(request)
	if err != nil {
		return nil, err
	}
	addresses, err := SenderAddresses{Users: drafts.Users}.Owned(principal.OwnerEmail)
	if err != nil {
		return nil, err
	}
	sender := principal.OwnerEmail
	if from, present := input["from"].(string); present {
		sender = from
	}
	sender = Security{}.NormalizeEmailAddress(sender)
	if sender == "" || !(PreferenceChoices{}).Includes(addresses, sender) {
		return nil, errors.New("From address is not owned by this mailbox")
	}
	if !(PreferenceChoices{}).Includes(principal.AllowedDomains, strings.SplitN(sender, "@", 2)[1]) {
		return nil, errors.New("Draft account is outside this MCP scope")
	}
	input["from"] = sender
	return drafts.store(principal.OwnerEmail, input)
}

// store creates the draft once per client id, so a lost answer is safely
// retried.
func (drafts McpDrafts) store(ownerEmail string, input map[string]any) (any, error) {
	owner := strings.ToLower(ownerEmail)
	domain := ""
	if parts := strings.SplitN(owner, "@", 2); len(parts) == 2 {
		domain = parts[1]
	}
	folder, err := McpFolders{Users: drafts.Users, Emails: drafts.Emails}.PathFor(owner, domain, "drafts")
	if err != nil {
		return nil, err
	}
	if folder == "" {
		folder = "drafts"
	}
	existing, err := drafts.Emails.List([]string{domain}, EmailListFilters{Folder: folder})
	if err != nil {
		return nil, err
	}
	for _, email := range existing {
		if email["draftOwner"] == owner && email["draftClientId"] == input["clientId"] {
			return map[string]any{"draftId": email["id"], "version": email["draftVersion"], "recovered": true}, nil
		}
	}
	record := drafts.record("draft-"+RandomText{}.Hex(16), owner, domain, input, folder)
	previous := drafts.Fylo.ActForDomain(domain)
	defer drafts.Fylo.Restore(previous)
	if _, err := drafts.Emails.Put(record); err != nil {
		return nil, err
	}
	return map[string]any{"draftId": record["id"], "version": record["draftVersion"], "recovered": false}, nil
}

func (McpDrafts) record(id string, owner string, domain string, input map[string]any, folder string) map[string]any {
	now := EpochTime{}.NowText()
	to, _ := input["to"].([]string)
	references, _ := input["references"].([]string)
	root := "draft:" + RecordValues{}.Text(input["clientId"])
	if inReplyTo, present := input["inReplyTo"].(string); present {
		root = inReplyTo
	}
	if len(references) > 0 {
		root = references[0]
	}
	recipient := owner
	if len(to) > 0 {
		recipient = to[0]
	}
	record := map[string]any{
		"id": id, "domain": domain, "ownerEmail": owner, "recipient": recipient, "to": to,
		"sender": RecordValues{}.Or(input["from"], owner), "subject": input["subject"], "body": input["text"],
		"threadId": Threading{}.DeriveThreadId(domain, root), "folder": folder, "read": true, "starred": false,
		"receivedAt": now, "processed": true, "hasAttachments": false, "draftOwner": owner,
		"draftClientId": input["clientId"], "draftVersion": 1, "draftUpdatedAt": now,
	}
	for _, field := range []string{"cc", "bcc"} {
		if list, _ := input[field].([]string); len(list) > 0 {
			record[field] = list
		}
	}
	if html := (RecordValues{}).Text(input["html"]); html != "" {
		record["htmlBody"] = html
	}
	for _, field := range []string{"inReplyTo", "sourceEmailId"} {
		if value, present := input[field]; present {
			record[field] = value
		}
	}
	if len(references) > 0 {
		record["references"] = references
	}
	return record
}

// parsed validates and normalizes a draft snapshot as DraftInput did.
func (drafts McpDrafts) parsed(value map[string]any) (map[string]any, error) {
	clientId := Security{}.Trim(RecordValues{}.Text(value["clientId"]))
	if clientId == "" || (TextValue{}).Length(clientId) > 120 || !regexp.MustCompile(`^[A-Za-z0-9._:-]+$`).MatchString(clientId) {
		return nil, errors.New("clientId is required")
	}
	input := map[string]any{"clientId": clientId}
	for _, field := range []string{"to", "cc", "bcc"} {
		addresses, err := drafts.addresses(value[field], field)
		if err != nil {
			return nil, err
		}
		input[field] = addresses
	}
	if from := value["from"]; from != nil {
		text, isText := from.(string)
		normalized := Security{}.NormalizeEmailAddress(text)
		if !isText || normalized == "" || strings.ContainsAny(text, "\x00\r\n") {
			return nil, errors.New("from must be a valid owned email address")
		}
		input["from"] = normalized
	}
	subject := Security{}.Trim(RecordValues{}.Text(value["subject"]))
	if (TextValue{}).Length(subject) > 500 || strings.ContainsAny(subject, "\x00\r\n") {
		return nil, errors.New("subject is invalid")
	}
	text, html := RecordValues{}.Text(value["text"]), RecordValues{}.Text(value["html"])
	if len(text)+len(html) > 5*1024*1024 {
		return nil, errors.New("Draft body is too large (max 5MB)")
	}
	input["subject"], input["text"], input["html"] = subject, text, html
	return drafts.threading(value, input)
}

func (McpDrafts) threading(value map[string]any, input map[string]any) (map[string]any, error) {
	if raw := value["inReplyTo"]; raw != nil {
		inReplyTo := Threading{}.NormalizeMessageId(raw)
		if inReplyTo == "" {
			return nil, errors.New("inReplyTo must be a valid Message-ID")
		}
		input["inReplyTo"] = inReplyTo
	}
	input["references"] = []string{}
	if raw := value["references"]; raw != nil {
		list, isList := RecordValues{}.List(raw)
		for _, item := range list {
			if _, isText := item.(string); !isText {
				isList = false
			}
		}
		if !isList || len(list) > 100 {
			return nil, errors.New("references must be an array of at most 100 Message-IDs")
		}
		references := Threading{}.ParseMessageIdList(list)
		if len(references) != len(list) {
			return nil, errors.New("references must contain valid unique Message-IDs")
		}
		input["references"] = references
	}
	if raw := value["sourceEmailId"]; raw != nil {
		text, isText := raw.(string)
		if !isText || (Security{}).Trim(text) == "" || (TextValue{}).Length(text) > 200 || strings.ContainsAny(text, "\r\n") {
			return nil, errors.New("sourceEmailId is invalid")
		}
		input["sourceEmailId"] = Security{}.Trim(text)
	}
	return input, nil
}

func (McpDrafts) addresses(value any, name string) ([]string, error) {
	if value == nil {
		return []string{}, nil
	}
	list, isList := RecordValues{}.List(value)
	if !isList || len(list) > 100 {
		return nil, fmt.Errorf("%s must be an array of at most 100 addresses", name)
	}
	normalized := []string{}
	for _, item := range list {
		address := Security{}.NormalizeEmailAddress(TextValue{}.Of(item))
		if address == "" || strings.ContainsAny(TextValue{}.Of(item), "\x00\r\n") {
			return nil, fmt.Errorf("%s contains an invalid email address", name)
		}
		if !(PreferenceChoices{}).Includes(normalized, address) {
			normalized = append(normalized, address)
		}
	}
	return normalized, nil
}
