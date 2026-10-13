import (
	"math"
	"strings"
)

// EmailsHelper reads and writes stored emails.
// @Helper
type EmailsHelper struct {
	// @Needs
	fyloHelper *FyloHelper
	// @Needs
	attachmentsHelper *AttachmentsHelper
	// @Needs
	rawMessagesHelper *RawMessagesHelper
}

// List answers the normalised emails of the allowed domains that match the
// filters, newest first.
func (helper *EmailsHelper) List(allowedDomains []string, filters EmailListFilters) ([]map[string]any, error) {
	stored, err := helper.emails().All()
	if err != nil {
		return nil, err
	}
	attachmentsByEmail := map[string][]map[string]any{}
	if filters.Query != "" || filters.HasAttachment != nil {
		if attachmentsByEmail, err = helper.attachmentsHelper.ListSummariesByEmail(allowedDomains); err != nil {
			return nil, err
		}
	}
	return EmailsRecords{}.Listed(stored, allowedDomains, filters, attachmentsByEmail), nil
}

// IterateFolder walks one folder of the allowed domains page by page,
// answering the normalised emails the owner may see (pageSize 0 reads pages
// of 128, at most 512).
func (helper *EmailsHelper) IterateFolder(allowedDomains []string, folder string, ownerEmail string, pageSize int) *FolderEmails {
	domains := []string{}
	for _, domain := range allowedDomains {
		domains = append(domains, strings.ToLower(domain))
	}
	if pageSize == 0 {
		pageSize = helper.pageSize()
	}
	return &FolderEmails{
		Database:   helper.fyloHelper.Database(),
		Domains:    MailAddresses{}.Unique(domains),
		Folder:     FolderEmails{}.Selected(folder),
		OwnerEmail: ownerEmail,
		PageSize:   int(math.Max(1, math.Min(float64(helper.largestPage()), float64(pageSize)))),
	}
}

func (*EmailsHelper) pageSize() int { return 128 }

func (*EmailsHelper) largestPage() int { return 512 }

// FindById answers the document id and normalised email with a logical id,
// or "" and nil when there is none.
func (helper *EmailsHelper) FindById(id string) (string, map[string]any, error) {
	record, err := helper.emails().First("id", id)
	if err != nil || record.Id == "" {
		return "", nil, err
	}
	return record.Id, EmailsRecords{}.Normalize(record.Data), nil
}

// Put stores an email and answers its document id.
func (helper *EmailsHelper) Put(email map[string]any) (string, error) {
	return helper.emails().Put(email)
}

// Update merges fields into one stored email.
func (helper *EmailsHelper) Update(documentId string, patch map[string]any) error {
	return helper.emails().Patch(documentId, patch)
}

// UpdateByIds merges the same fields into every email with one of the
// logical ids and answers the matched emails as they were before.
func (helper *EmailsHelper) UpdateByIds(ids []string, patch map[string]any) ([]map[string]any, error) {
	if len(ids) == 0 {
		return []map[string]any{}, nil
	}
	stored, err := helper.emails().All()
	if err != nil {
		return nil, err
	}
	updates, matched := EmailsRecords{}.Matched(stored, ids, patch)
	return matched, helper.emails().PatchEach(updates)
}

// Delete deletes one stored email with its attachments and raw source.
func (helper *EmailsHelper) Delete(documentId string, emailId string) error {
	if emailId != "" {
		if err := helper.attachmentsHelper.DeleteForEmail(emailId); err != nil {
			return err
		}
		if err := helper.rawMessagesHelper.DeleteForEmail(emailId); err != nil {
			return err
		}
	}
	return helper.emails().Delete(documentId)
}

func (helper *EmailsHelper) emails() FyloCollection {
	return FyloCollection{Database: helper.fyloHelper.Database(), Name: FyloCollections{}.Named().Emails}
}
