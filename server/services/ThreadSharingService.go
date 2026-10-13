// ThreadSharingService creates and resolves expiring conversation links.
// @Service
type ThreadSharingService struct {
	// @Needs
	threadSharesHelper *ThreadSharesHelper
	// @Needs
	threadsHelper *ThreadsHelper
	// @Needs
	emailsHelper *EmailsHelper
	// @Needs
	attachmentsHelper *AttachmentsHelper
	// @Needs
	fyloHelper *FyloHelper
}

// Create mints a link; a request the caller can correct fails with a
// *ThreadShareInputError, and a conversation not the owner's answers nil.
func (service *ThreadSharingService) Create(request ThreadShareRequest) (map[string]any, error) {
	return service.sharing().Create(request)
}

// List answers the owner's links, or nil when the named thread is not the
// owner's.
func (service *ThreadSharingService) List(ownerEmail string, emailId string, domains []string) ([]any, error) {
	return service.sharing().List(ownerEmail, emailId, domains)
}

// Revoke revokes one of the owner's links.
func (service *ThreadSharingService) Revoke(id string, ownerEmail string) (bool, error) {
	return service.sharing().Revoke(id, ownerEmail)
}

// Resolve answers the public view of a link, or nil.
func (service *ThreadSharingService) Resolve(token string) (map[string]any, error) {
	return service.sharing().Resolve(token)
}

// ReadAttachment answers one shared attachment, or nil.
func (service *ThreadSharingService) ReadAttachment(token string, attachmentId string) (map[string]any, error) {
	return service.sharing().ReadAttachment(token, attachmentId)
}

func (service *ThreadSharingService) sharing() ThreadSharing {
	return ThreadSharing{Shares: service.threadSharesHelper, Threads: service.threadsHelper, Emails: service.emailsHelper, Attachments: service.attachmentsHelper, Fylo: service.fyloHelper}
}
