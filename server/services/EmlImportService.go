// EmlImportService imports RFC 822 files into one authorized mailbox
// without running the SMTP delivery pipeline.
// @Service
type EmlImportService struct {
	// @Needs
	emailsHelper *EmailsHelper
	// @Needs
	attachmentsHelper *AttachmentsHelper
	// @Needs
	rawMessagesHelper *RawMessagesHelper
}

// ImportOne stores one exact EML source and its searchable form, and
// answers the imported message's summary.
func (service *EmlImportService) ImportOne(source []byte, domain string, mailbox string, folder string) (map[string]any, error) {
	return EmlImport{Emails: service.emailsHelper, Attachments: service.attachmentsHelper, RawMessages: service.rawMessagesHelper}.One(source, domain, mailbox, folder)
}
