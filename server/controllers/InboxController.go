import "net/http"

// @RequestRoute("/inbox")
// @SignedIn
type InboxController struct {
	// @Needs
	accessHelper *AccessHelper
	// @Needs
	categoryMappingService *CategoryMappingService
	// @Needs
	domainMigrationsHelper *DomainMigrationsHelper
	// @Needs
	emailsHelper *EmailsHelper
	// @Needs
	followUpRemindersHelper *FollowUpRemindersHelper
}

// @GetRoute
func (controller *InboxController) GetInbox(request *http.Request) *http.Response {
	return InboxRoute{Parts: MailboxParts{
		Access: controller.accessHelper, Categories: controller.categoryMappingService,
		Migrations: controller.domainMigrationsHelper, Emails: controller.emailsHelper,
		Reminders: controller.followUpRemindersHelper,
	}}.List(request)
}
