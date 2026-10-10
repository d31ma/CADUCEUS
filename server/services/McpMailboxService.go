// McpMailboxService runs the MCP mailbox tools for an authenticated
// capability.
// @Service
type McpMailboxService struct {
	// @Needs
	attachmentsHelper *AttachmentsHelper
	// @Needs
	emailsHelper *EmailsHelper
	// @Needs
	scheduledHelper *ScheduledHelper
	// @Needs
	threadsHelper *ThreadsHelper
	// @Needs
	usersHelper *UsersHelper
}

// Run answers the result of one tool with parsed arguments.
func (service *McpMailboxService) Run(principal McpPrincipal, name string, input map[string]any) (any, error) {
	mailbox := McpMailbox{
		Principal: principal, Users: service.usersHelper, Emails: service.emailsHelper,
		Threads: service.threadsHelper, Attachments: service.attachmentsHelper, Scheduled: service.scheduledHelper,
	}
	return mailbox.Run(name, input)
}
