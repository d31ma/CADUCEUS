// McpMailboxService runs the MCP mailbox tools for an authenticated
// capability.
// @Service
type McpMailboxService struct {
	// @Needs
	categoryMappingService *CategoryMappingService
	// @Needs
	draftService *DraftService
	// @Needs
	emailsHelper *EmailsHelper
	// @Needs
	threadsHelper *ThreadsHelper
	// @Needs
	usersHelper *UsersHelper
}

// Run answers the result of one tool with parsed arguments.
func (service *McpMailboxService) Run(call McpToolCall) (any, error) {
	mailbox := McpMailbox{
		Principal: call.Principal, Users: service.usersHelper, Emails: service.emailsHelper,
		Threads: service.threadsHelper, Drafts: service.draftService, Categories: service.categoryMappingService, Fylo: call.Fylo,
	}
	return mailbox.Run(call.Name, call.Input)
}
