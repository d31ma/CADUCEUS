import "net/http"

// @RequestRoute("/reminders")
// @SignedIn
type RemindersController struct {
	// @Needs
	accessHelper *AccessHelper
	// @Needs
	emailsHelper *EmailsHelper
	// @Needs
	followUpReminderService *FollowUpReminderService
}

// @GetRoute
func (controller *RemindersController) GetReminders(request *http.Request) *http.Response {
	return controller.route().List(request)
}

// @PostRoute
func (controller *RemindersController) PostReminder(request *http.Request) *http.Response {
	return controller.route().Create(request)
}

// @DeleteRoute
func (controller *RemindersController) DeleteReminder(request *http.Request) *http.Response {
	return controller.route().Dismiss(request)
}

func (controller *RemindersController) route() RemindersRoute {
	return RemindersRoute{Access: controller.accessHelper, Emails: controller.emailsHelper, Reminders: controller.followUpReminderService}
}
