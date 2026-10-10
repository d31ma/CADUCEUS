// ThreadsHelper groups the mailbox's messages into conversations.
// @Helper
type ThreadsHelper struct {
	// @Needs
	emailsHelper *EmailsHelper
}

// List answers the conversations of the allowed domains, newest first:
// those with a message in the folder (the domain's own folder first), or
// every conversation for no folder or "all".
func (helper *ThreadsHelper) List(allowedDomains []string, folder string, ownerEmail string, folderByDomain map[string]string) ([]Thread, error) {
	emails, err := helper.emailsHelper.List(allowedDomains, EmailListFilters{Folder: helper.everyFolder(), OwnerEmail: ownerEmail})
	if err != nil {
		return nil, err
	}
	threads := ThreadsRecords{}.GroupMessagesIntoThreads(emails, ownerEmail)
	return ThreadsRecords{}.InFolder(threads, folder, folderByDomain), nil
}

// FindForEmail answers the whole authorized conversation holding one
// message, whatever folders its messages are in, or nil.
func (helper *ThreadsHelper) FindForEmail(allowedDomains []string, emailId string, ownerEmail string) (*Thread, error) {
	if emailId == "" {
		return nil, nil
	}
	threads, err := helper.List(allowedDomains, helper.everyFolder(), ownerEmail, nil)
	if err != nil {
		return nil, err
	}
	return ThreadsRecords{}.Holding(threads, emailId), nil
}

func (*ThreadsHelper) everyFolder() string { return "all" }
