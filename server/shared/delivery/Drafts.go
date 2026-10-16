import (
	"encoding/base64"
	"strings"
)

// Drafts is the durable, optimistic-concurrency draft lifecycle: a draft is
// created once per client id (so a lost response is safely retryable),
// updated only by compare-and-swap on its version, and private to the
// mailbox that owns it.
type Drafts struct {
	Emails      *EmailsHelper
	Attachments *AttachmentsHelper
	Users       *UsersHelper
	Storage     TenantStorage
}

// Create answers the owner's draft for the input's client id, created now
// or recovered, and whether it was recovered.
func (drafts Drafts) Create(ownerEmail string, input map[string]any) (map[string]any, bool, error) {
	owner := strings.ToLower(ownerEmail)
	domain := DeliveryFields{}.Domain(owner)
	folder := CategoryMappings{}.OfOwner(drafts.Users, owner).PathFor(domain, "drafts")
	listed, err := drafts.Emails.List([]string{domain}, EmailListFilters{Folder: folder})
	if err != nil {
		return nil, false, err
	}
	for _, email := range listed {
		if email["draftOwner"] == owner && email["draftClientId"] == input["clientId"] {
			presented, err := drafts.Present(email)
			return presented, true, err
		}
	}
	id := "draft-" + RandomText{}.Hex(16)
	draft := drafts.record(id, owner, domain, input, 1, folder)
	err = drafts.Storage.Within(domain, func() error {
		documentId, err := drafts.Emails.Put(draft)
		if err != nil {
			return err
		}
		if _, err := drafts.Attachments.SaveForEmail(id, domain, (SentMail{}).parsedAttachments(input)); err != nil {
			_ = drafts.Emails.Delete(documentId, id)
			return err
		}
		return nil
	})
	if err != nil {
		return nil, false, err
	}
	presented, err := drafts.Present(draft)
	return presented, false, err
}

// Update replaces the draft when expectedVersion is its current version,
// answering ("updated", draft), ("conflict", {currentVersion, updatedAt})
// or ("not-found", nil).
func (drafts Drafts) Update(id string, ownerEmail string, allowedDomains []string, expectedVersion int, input map[string]any) (string, map[string]any, error) {
	documentId, draft, err := drafts.FindOwned(id, ownerEmail, allowedDomains)
	if err != nil || documentId == "" {
		return "not-found", nil, err
	}
	if clientId := (RecordValues{}).Text(draft["draftClientId"]); clientId != "" && clientId != input["clientId"] {
		return "not-found", nil, nil
	}
	current := int(RecordValues{}.NumberOf(draft["draftVersion"]))
	if current != expectedVersion {
		return "conflict", map[string]any{"currentVersion": current, "updatedAt": RecordValues{}.Or(draft["draftUpdatedAt"], draft["receivedAt"])}, nil
	}
	domain := RecordValues{}.Text(draft["domain"])
	patch := drafts.record(id, strings.ToLower(ownerEmail), domain, input, current+1, RecordValues{}.Text(draft["folder"]))
	err = drafts.Storage.Within(domain, func() error {
		if _, err := drafts.Attachments.ReplaceForEmail(id, domain, (SentMail{}).parsedAttachments(input)); err != nil {
			return err
		}
		return drafts.Emails.Update(documentId, patch)
	})
	if err != nil {
		return "", nil, err
	}
	presented, err := drafts.Present(patch)
	return "updated", presented, err
}

// Get answers the owner's draft, or nil.
func (drafts Drafts) Get(id string, ownerEmail string, allowedDomains []string) (map[string]any, error) {
	documentId, draft, err := drafts.FindOwned(id, ownerEmail, allowedDomains)
	if err != nil || documentId == "" {
		return nil, err
	}
	return drafts.Present(draft)
}

// Discard deletes the owner's draft with its files and reports whether
// there was one.
func (drafts Drafts) Discard(id string, ownerEmail string, allowedDomains []string) (bool, error) {
	documentId, draft, err := drafts.FindOwned(id, ownerEmail, allowedDomains)
	if err != nil || documentId == "" {
		return false, err
	}
	return true, drafts.Storage.Within(RecordValues{}.Text(draft["domain"]), func() error {
		return drafts.Emails.Delete(documentId, id)
	})
}

// FindOwned answers the document id and draft when it is a draft of the
// owner in one of the allowed domains, else "".
func (drafts Drafts) FindOwned(id string, ownerEmail string, allowedDomains []string) (string, map[string]any, error) {
	documentId, draft, err := drafts.Emails.FindById(id)
	if err != nil || documentId == "" || draft == nil {
		return "", nil, err
	}
	owner := strings.ToLower(ownerEmail)
	domain := RecordValues{}.Text(draft["domain"])
	if draft["folder"] != (CategoryMappings{}).OfOwner(drafts.Users, owner).PathFor(domain, "drafts") {
		return "", nil, nil
	}
	recorded := strings.ToLower(RecordValues{}.Text(draft["draftOwner"]))
	if recorded == "" {
		recorded = strings.ToLower(RecordValues{}.Text(draft["sender"]))
	}
	if recorded != owner || !(TextValue{}).Includes(allowedDomains, domain) {
		return "", nil, nil
	}
	return documentId, draft, nil
}

// Present answers the draft with its attachments' bytes, which only the
// draft API returns.
func (drafts Drafts) Present(draft map[string]any) (map[string]any, error) {
	records, err := drafts.Attachments.List(RecordValues{}.Text(draft["id"]))
	if err != nil {
		return nil, err
	}
	attachments := []any{}
	for _, record := range records {
		content, err := drafts.Attachments.ReadContent(record)
		if err != nil {
			return nil, err
		}
		summary := StoredRecords{}.Without(record, "docId", "bucketId", "storagePath")
		summary["contentBase64"] = base64.StdEncoding.EncodeToString(content)
		attachments = append(attachments, summary)
	}
	return DeliveryFields{}.Merged(draft, map[string]any{"attachments": attachments}), nil
}

func (Drafts) record(id string, owner string, domain string, input map[string]any, version int, folder string) map[string]any {
	fields := DeliveryFields{}
	values := RecordValues{}
	now := EpochTime{}.NowText()
	to := fields.Strings(input["to"])
	references := fields.Strings(input["references"])
	root := "draft:" + values.Text(input["clientId"])
	if len(references) > 0 {
		root = references[0]
	} else if inReplyTo := values.Text(input["inReplyTo"]); inReplyTo != "" {
		root = inReplyTo
	}
	recipient := owner
	if len(to) > 0 {
		recipient = to[0]
	}
	sender := values.Text(input["from"])
	if sender == "" {
		sender = owner
	}
	attachments, _ := values.List(input["attachments"])
	record := map[string]any{
		"id": id, "domain": domain, "ownerEmail": owner, "recipient": recipient, "to": input["to"], "sender": sender,
		"subject": input["subject"], "body": input["text"], "threadId": Threading{}.DeriveThreadId(domain, root),
		"folder": folder, "read": true, "starred": false, "receivedAt": now, "processed": true,
		"hasAttachments": len(attachments) > 0, "draftOwner": owner, "draftClientId": input["clientId"],
		"draftVersion": version, "draftUpdatedAt": now,
		// Nil is intentional: FYLO patches merge, so omission would retain an old proof.
		"quoteProof": input["quoteProof"],
	}
	for _, field := range []string{"cc", "bcc", "replyTo", "references"} {
		if len(fields.Strings(input[field])) > 0 {
			record[field] = input[field]
		}
	}
	for field, stored := range map[string]string{"html": "htmlBody", "inReplyTo": "inReplyTo", "sourceEmailId": "sourceEmailId"} {
		if values.Text(input[field]) != "" {
			record[stored] = input[field]
		}
	}
	return record
}
