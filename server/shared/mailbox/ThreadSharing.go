import (
	"encoding/base64"
	"math"
	"sort"
	"strings"
	"time"
)

// ThreadSharing creates and resolves self-hosted conversation-sharing
// capabilities. The bearer secret is answered once and never stored; FYLO
// keeps only its SHA-256 hash, the exact owner, the fixed message list, the
// expiry and the revocation.
type ThreadSharing struct {
	Shares      *ThreadSharesHelper
	Threads     *ThreadsHelper
	Emails      *EmailsHelper
	Attachments *AttachmentsHelper
	Fylo        *FyloHelper
}

// Create mints a link and revokes the thread's earlier active link; nil when
// the conversation is not the owner's.
func (sharing ThreadSharing) Create(request ThreadShareRequest) (map[string]any, error) {
	days := RecordValues{}.Numeric(request.ExpiresInDays)
	if days != math.Trunc(days) || (days != 1 && days != 7 && days != 30) {
		return nil, &ThreadShareInputError{Message: "expiresInDays must be one of: 1, 7, 30"}
	}
	emailId := Security{}.Trim(TextValue{}.Of(RecordValues{}.Or(request.EmailId, "")))
	if emailId == "" || (TextValue{}).Length(emailId) > 500 {
		return nil, &ThreadShareInputError{Message: "emailId is required"}
	}
	thread, err := sharing.Threads.FindForEmail(request.Domains, emailId, request.OwnerEmail)
	if err != nil || thread == nil {
		return nil, err
	}
	domain := Security{}.NormalizeDomain(TextValue{}.Of(thread.LatestEmail["domain"]))
	if domain == "" || !(TextValue{}).Includes(request.Domains, domain) {
		return nil, nil
	}
	emailIds := []any{}
	for _, email := range thread.Emails {
		emailIds = append(emailIds, email["id"])
	}
	if len(TextValue{}.Json(emailIds)) > 512*1024 {
		return nil, &ThreadShareInputError{Message: "This conversation is too large to share safely"}
	}
	now := EpochTime{}.Now()
	if err := sharing.revokeActive(request.OwnerEmail, thread.Id, now); err != nil {
		return nil, err
	}
	token := CapabilityToken{}.Create(domain)
	record := map[string]any{
		"id":         "share-" + CapabilityToken{}.Random(16),
		"ownerEmail": strings.ToLower(request.OwnerEmail),
		"domain":     domain,
		"threadId":   thread.Id,
		"subject":    TextValue{}.Prefix(TextValue{}.Of(RecordValues{}.Or(thread.Subject, "(No subject)")), 500),
		"emailIds":   emailIds,
		"tokenHash":  CapabilityToken{}.Hash(token),
		"createdAt":  EpochTime{}.Text(now),
		"expiresAt":  EpochTime{}.Text(now + int64(days)*86_400_000),
	}
	if _, err := sharing.Shares.Put(record); err != nil {
		return nil, err
	}
	created := sharing.PresentManaged(record)
	created["token"] = token
	created["url"] = "/share/" + token
	return created, nil
}

func (sharing ThreadSharing) revokeActive(ownerEmail string, threadId string, now int64) error {
	existing, err := sharing.Shares.ListForOwner(ownerEmail)
	if err != nil {
		return err
	}
	for _, share := range existing {
		if share["threadId"] == threadId && sharing.active(share, now) {
			if err := sharing.Shares.Update(RecordValues{}.Text(share["docId"]), map[string]any{"revokedAt": EpochTime{}.Text(now)}); err != nil {
				return err
			}
		}
	}
	return nil
}

// List answers the owner's links, newest first, narrowed to the thread of
// one message when emailId is given; nil when that thread is not the
// owner's.
func (sharing ThreadSharing) List(ownerEmail string, emailId string, domains []string) ([]any, error) {
	records, err := sharing.Shares.ListForOwner(ownerEmail)
	if err != nil {
		return nil, err
	}
	if emailId != "" {
		thread, err := sharing.Threads.FindForEmail(domains, emailId, ownerEmail)
		if err != nil || thread == nil {
			return nil, err
		}
		selected := []map[string]any{}
		for _, share := range records {
			if share["threadId"] == thread.Id {
				selected = append(selected, share)
			}
		}
		records = selected
	}
	sort.SliceStable(records, func(left, right int) bool {
		return TextOrder{}.Compare(TextValue{}.Of(records[right]["createdAt"]), TextValue{}.Of(records[left]["createdAt"])) < 0
	})
	listed := []any{}
	for _, share := range records {
		listed = append(listed, sharing.PresentManaged(share))
	}
	return listed, nil
}

// Revoke revokes one of the owner's links and reports whether it was the
// owner's.
func (sharing ThreadSharing) Revoke(id string, ownerEmail string) (bool, error) {
	documentId, record, err := sharing.Shares.FindById(id)
	if err != nil || documentId == "" || record == nil || record["ownerEmail"] != strings.ToLower(ownerEmail) {
		return false, err
	}
	if !(RecordValues{}).Truthy(record["revokedAt"]) {
		return true, sharing.Shares.Update(documentId, map[string]any{"revokedAt": EpochTime{}.NowText()})
	}
	return true, nil
}

// Resolve answers the public view of a link, re-checking every message each
// time; invalid, expired and revoked links all answer nil.
func (sharing ThreadSharing) Resolve(token string) (map[string]any, error) {
	parsed, valid := CapabilityToken{}.Parse(token, 400)
	if !valid {
		return nil, nil
	}
	previous := sharing.Fylo.ActForDomain(parsed.Domain)
	defer sharing.Fylo.Restore(previous)
	record, err := sharing.share(parsed)
	if err != nil || record == nil {
		return nil, err
	}
	messages := []map[string]any{}
	for _, emailId := range (TextValue{}).List(record["emailIds"]) {
		_, email, err := sharing.Emails.FindById(TextValue{}.Of(emailId))
		if err != nil {
			return nil, err
		}
		if email == nil || email["domain"] != record["domain"] || !(EmailsRecords{}).CanAccessEmail(email, TextValue{}.Of(record["ownerEmail"])) {
			continue
		}
		attachments, err := sharing.Attachments.ListSummaries(TextValue{}.Of(email["id"]))
		if err != nil {
			return nil, err
		}
		messages = append(messages, sharing.publicMessage(email, attachments))
	}
	sort.SliceStable(messages, func(left, right int) bool {
		return TextOrder{}.Compare(messages[left]["receivedAt"].(string), messages[right]["receivedAt"].(string)) < 0
	})
	if len(messages) == 0 {
		return nil, nil
	}
	view := map[string]any{"messages": messages}
	for _, field := range []string{"id", "subject", "createdAt", "expiresAt"} {
		if value, present := record[field]; present {
			view[field] = value
		}
	}
	return view, nil
}

// ReadAttachment answers one attachment of a shared message as base64, after
// the same capability and membership checks.
func (sharing ThreadSharing) ReadAttachment(token string, attachmentId string) (map[string]any, error) {
	if attachmentId == "" || (TextValue{}).Length(attachmentId) > 200 {
		return nil, nil
	}
	parsed, valid := CapabilityToken{}.Parse(token, 400)
	if !valid {
		return nil, nil
	}
	previous := sharing.Fylo.ActForDomain(parsed.Domain)
	defer sharing.Fylo.Restore(previous)
	record, err := sharing.share(parsed)
	if err != nil || record == nil {
		return nil, err
	}
	_, attachment, err := sharing.Attachments.FindById(attachmentId)
	if err != nil || attachment == nil || attachment["domain"] != record["domain"] || !(TextValue{}).Includes(sharing.texts(record["emailIds"]), attachment["emailId"]) {
		return nil, err
	}
	_, email, err := sharing.Emails.FindById(TextValue{}.Of(attachment["emailId"]))
	if err != nil || email == nil || !(EmailsRecords{}).CanAccessEmail(email, TextValue{}.Of(record["ownerEmail"])) {
		return nil, err
	}
	content, err := sharing.Attachments.ReadContent(attachment)
	if err != nil {
		return nil, err
	}
	download := map[string]any{"contentBase64": base64.StdEncoding.EncodeToString(content)}
	for _, field := range []string{"id", "filename", "contentType", "size"} {
		if value, present := attachment[field]; present {
			download[field] = value
		}
	}
	return download, nil
}

// share answers the active share of a parsed token, or nil.
func (sharing ThreadSharing) share(parsed ParsedCapability) (map[string]any, error) {
	records, err := sharing.Shares.FindByTokenHash(parsed.Hash)
	if err != nil {
		return nil, err
	}
	for _, candidate := range records {
		if candidate["domain"] == parsed.Domain && (CapabilityToken{}).EqualHash(TextValue{}.Of(candidate["tokenHash"]), parsed.Hash) {
			if !sharing.active(candidate, EpochTime{}.Now()) {
				return nil, nil
			}
			return candidate, nil
		}
	}
	return nil, nil
}

// PresentManaged answers the owner's view of a share, without its hash.
func (sharing ThreadSharing) PresentManaged(record map[string]any) map[string]any {
	view := map[string]any{
		"messageCount": len(TextValue{}.List(record["emailIds"])),
		"revokedAt":    RecordValues{}.Or(record["revokedAt"], ""),
		"active":       sharing.active(record, time.Now().UnixMilli()),
	}
	for _, field := range []string{"id", "threadId", "subject", "createdAt", "expiresAt"} {
		if value, present := record[field]; present {
			view[field] = value
		}
	}
	return view
}

func (ThreadSharing) active(record map[string]any, now int64) bool {
	expiresAt := EpochTime{}.Parse(TextValue{}.Of(record["expiresAt"]))
	return !(RecordValues{}).Truthy(record["revokedAt"]) && !math.IsNaN(expiresAt) && !math.IsInf(expiresAt, 0) && expiresAt > float64(now)
}

func (ThreadSharing) publicMessage(email map[string]any, attachments []map[string]any) map[string]any {
	values := RecordValues{}
	to := email["to"]
	if list, isList := values.List(to); !isList || len(list) == 0 {
		to = []any{}
		if recipient := (TextValue{}).Of(values.Or(email["recipient"], "")); recipient != "" {
			to = []any{recipient}
		}
	}
	cc := email["cc"]
	if _, isList := values.List(cc); !isList {
		cc = []any{}
	}
	listed := []any{}
	for _, attachment := range attachments {
		summary := map[string]any{}
		for _, field := range []string{"id", "filename", "contentType", "size"} {
			if value, present := attachment[field]; present {
				summary[field] = value
			}
		}
		listed = append(listed, summary)
	}
	text := func(field string) string { return TextValue{}.Of(values.Or(email[field], "")) }
	return map[string]any{
		"from":        TextValue{}.Of(values.Or(email["headerFrom"], values.Or(email["sender"], ""))),
		"to":          to,
		"cc":          cc,
		"subject":     text("subject"),
		"body":        text("body"),
		"htmlBody":    text("htmlBody"),
		"receivedAt":  text("receivedAt"),
		"attachments": listed,
	}
}

func (ThreadSharing) texts(value any) []string {
	texts := []string{}
	for _, item := range (TextValue{}).List(value) {
		if text, isText := item.(string); isText {
			texts = append(texts, text)
		}
	}
	return texts
}
