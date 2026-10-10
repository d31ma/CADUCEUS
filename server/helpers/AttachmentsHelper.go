// AttachmentsHelper reads and writes email attachments: their records and
// the files in the attachment bucket.
// @Helper
type AttachmentsHelper struct {
	// @Needs
	fyloHelper *FyloHelper
}

// List answers the attachment records of one email, each with its docId.
func (helper *AttachmentsHelper) List(emailId string) ([]map[string]any, error) {
	stored, err := helper.attachments().Where("emailId", emailId)
	if err != nil {
		return nil, err
	}
	records := []map[string]any{}
	for _, record := range stored {
		records = append(records, record.WithDocId())
	}
	return records, nil
}

// ListSummaries answers the summaries of one email's attachments.
func (helper *AttachmentsHelper) ListSummaries(emailId string) ([]map[string]any, error) {
	records, err := helper.List(emailId)
	return helper.summaries(records), err
}

// ListSummariesByEmail answers the attachment summaries of the allowed
// domains, grouped by email id.
func (helper *AttachmentsHelper) ListSummariesByEmail(allowedDomains []string) (map[string][]map[string]any, error) {
	stored, err := helper.attachments().All()
	if err != nil {
		return nil, err
	}
	return AttachmentsRecords{}.Grouped(stored, allowedDomains), nil
}

// FindById answers the document id and record of one attachment, or "" and
// nil when there is none.
func (helper *AttachmentsHelper) FindById(id string) (string, map[string]any, error) {
	record, err := helper.attachments().First("id", id)
	return record.Id, record.Data, err
}

// SaveForEmail stores an email's attachments and answers their summaries.
func (helper *AttachmentsHelper) SaveForEmail(emailId string, domain string, attachments []ParsedAttachment) ([]map[string]any, error) {
	database := helper.fyloHelper.Database()
	saved := []map[string]any{}
	for _, attachment := range attachments {
		record, err := AttachmentsRecords{}.PersistOne(database, emailId, domain, attachment)
		if err != nil {
			return nil, err
		}
		saved = append(saved, record)
	}
	return helper.summaries(saved), nil
}

// ReplaceForEmail replaces a draft's whole attachment set. The old set is
// deleted only once every replacement file is stored; a failed replacement
// removes what it stored and keeps the old set.
func (helper *AttachmentsHelper) ReplaceForEmail(emailId string, domain string, attachments []ParsedAttachment) ([]map[string]any, error) {
	database := helper.fyloHelper.Database()
	previous, err := helper.List(emailId)
	if err != nil {
		return nil, err
	}
	saved := []map[string]any{}
	for _, attachment := range attachments {
		record, err := AttachmentsRecords{}.PersistOne(database, emailId, domain, attachment)
		if err != nil {
			_ = helper.deleteAll(saved)
			return nil, err
		}
		saved = append(saved, record)
	}
	return helper.summaries(saved), helper.deleteAll(previous)
}

// DeleteForEmail deletes every attachment of one email.
func (helper *AttachmentsHelper) DeleteForEmail(emailId string) error {
	attachments, err := helper.List(emailId)
	if err != nil {
		return err
	}
	return helper.deleteAll(attachments)
}

// ReadContent answers the bytes of one attachment, from the bucket or from
// a legacy file path inside the attachment root.
func (helper *AttachmentsHelper) ReadContent(attachment map[string]any) ([]byte, error) {
	return AttachmentsRecords{}.ReadContent(helper.fyloHelper.Database(), attachment)
}

// deleteAll deletes every record, each one even when another fails, and
// answers the first failure.
func (helper *AttachmentsHelper) deleteAll(records []map[string]any) error {
	database := helper.fyloHelper.Database()
	var first error
	for _, record := range records {
		if err := (AttachmentsRecords{}).DeleteRecord(database, record); err != nil && first == nil {
			first = err
		}
	}
	return first
}

func (*AttachmentsHelper) summaries(records []map[string]any) []map[string]any {
	summaries := []map[string]any{}
	for _, record := range records {
		summaries = append(summaries, AttachmentsRecords{}.ToSummary(record))
	}
	return summaries
}

func (helper *AttachmentsHelper) attachments() FyloCollection {
	return FyloCollection{Database: helper.fyloHelper.Database(), Name: FyloCollections{}.Named().Attachments}
}
