import "net/http"


// MailboxParts are the helpers and services the mailbox routes share; each
// controller fills the ones its routes use.
type MailboxParts struct {
	Access      *AccessHelper
	Emails      *EmailsHelper
	Attachments *AttachmentsHelper
	RawMessages *RawMessagesHelper
	Categories  *CategoryMappingService
	Migrations  *DomainMigrationsHelper
	Calendar    *CalendarEventsHelper
	Reminders   *FollowUpRemindersHelper
}

// Owned answers the stored email with the logical id that the caller may
// read: 404 when there is none or it belongs to another mailbox of the
// domain, 403 when its domain is outside the caller's.
func (parts MailboxParts) Owned(caller RouteCaller, id string) (string, map[string]any, *http.Response) {
	documentId, email, err := parts.Emails.FindById(id)
	if err != nil {
		return "", nil, RouteAccess{}.Failure(err)
	}
	if documentId == "" || email == nil {
		return "", nil, Answer{}.NotFound("Email not found")
	}
	if !(TextValue{}).Includes(caller.Domains, email["domain"]) {
		return "", nil, Answer{}.Forbidden("Access denied")
	}
	if !(EmailsRecords{}).CanAccessEmail(email, caller.Email) {
		return "", nil, Answer{}.NotFound("Email not found")
	}
	return documentId, email, nil
}

// Presented answers emails with their migrated recipients and the owner's
// category roles.
func (parts MailboxParts) Presented(caller RouteCaller, emails []map[string]any) ([]map[string]any, error) {
	migrated, err := MigratedRecipients{Migrations: parts.Migrations}.Presented(emails)
	if err != nil {
		return nil, err
	}
	mappings, err := parts.Categories.ForOwner(caller.Email)
	return mappings.PresentEmails(migrated), err
}

// Provider answers the connected-account mail of the deployment.
func (MailboxParts) Provider() UnifiedMail {
	return UnifiedMail{Accounts: ProviderAccounts{}.FromSettings()}
}

// Mappings answers the caller's category role mappings.
func (parts MailboxParts) Mappings(caller RouteCaller) (CategoryMappings, error) {
	return parts.Categories.ForOwner(caller.Email)
}

// Listed answers the stored emails of the allowed domains that match filters.
func (parts MailboxParts) Listed(allowed []string, filters EmailListFilters) ([]map[string]any, error) {
	return parts.Emails.List(allowed, filters)
}

// Update merges fields into one stored email.
func (parts MailboxParts) Update(documentId string, patch map[string]any) error {
	return parts.Emails.Update(documentId, patch)
}

// Delete deletes one stored email with its attachments and source.
func (parts MailboxParts) Delete(documentId string, id string) error {
	return parts.Emails.Delete(documentId, id)
}

// Source answers the retained RFC 822 source record of an email, or nil.
func (parts MailboxParts) Source(id string) (map[string]any, error) {
	return parts.RawMessages.FindForEmail(id)
}

// SourceBytes answers the bytes of a source record.
func (parts MailboxParts) SourceBytes(record map[string]any) ([]byte, error) {
	return parts.RawMessages.ReadContent(record)
}

// AttachmentRecords answers the attachment records of an email.
func (parts MailboxParts) AttachmentRecords(id string) ([]map[string]any, error) {
	return parts.Attachments.List(id)
}

// AttachmentSummaries answers the attachment summaries of an email.
func (parts MailboxParts) AttachmentSummaries(id string) ([]map[string]any, error) {
	return parts.Attachments.ListSummaries(id)
}

// Attachment answers one attachment record, or nil.
func (parts MailboxParts) Attachment(id string) (map[string]any, error) {
	_, record, err := parts.Attachments.FindById(id)
	return record, err
}

// AttachmentBytes answers the bytes of one attachment record.
func (parts MailboxParts) AttachmentBytes(record map[string]any) ([]byte, error) {
	return parts.Attachments.ReadContent(record)
}

// CategoryAccounts answers the caller's categories per domain, or nil when
// the caller has no user record.
func (parts MailboxParts) CategoryAccounts(caller RouteCaller) ([]map[string]any, error) {
	return parts.Categories.ListAccounts(caller.Email, caller.Domains)
}

// CreateCategory adds a folder or label to the caller's catalog.
func (parts MailboxParts) CreateCategory(caller RouteCaller, input any) (map[string]any, *CategoryRefusal, error) {
	return parts.Categories.CreateCategory(caller.Email, input)
}
