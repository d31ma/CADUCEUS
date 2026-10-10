// ConversationArchiveService archives a conversation together with a sent
// reply, from a bounded snapshot that an undo of the send can restore.
// @Service
type ConversationArchiveService struct {
	// @Needs
	emailsHelper *EmailsHelper
	// @Needs
	threadsHelper *ThreadsHelper
	// @Needs
	categoryMappingService *CategoryMappingService
}

// Plan authorizes the source email before a send is accepted and answers
// the snapshot to archive.
func (service *ConversationArchiveService) Plan(sourceEmailId string, ownerEmail string, domains []string) (map[string]any, error) {
	return service.archive().Plan(sourceEmailId, ownerEmail, domains)
}

// Archive archives exactly the planned snapshot.
func (service *ConversationArchiveService) Archive(plan map[string]any, ownerEmail string) (map[string]any, error) {
	return service.archive().Archive(plan, ownerEmail)
}

// Serialize answers the plan as the text a send's undo keeps.
func (service *ConversationArchiveService) Serialize(plan map[string]any) string {
	return service.archive().Serialized(plan)
}

// Restore restores the rows a serialized plan archived that are still
// archived, and answers how many.
func (service *ConversationArchiveService) Restore(serialized string, ownerEmail string, domains []string) (int, error) {
	return service.archive().Restore(serialized, ownerEmail, domains)
}

func (service *ConversationArchiveService) archive() ConversationArchiving {
	return ConversationArchiving{Emails: service.emailsHelper, Threads: service.threadsHelper, Categories: service.categoryMappingService}
}
