import "net/http"

// @RequestRoute("/reminders/tick")
// @Public
type RemindersTickController struct {
	// @Needs
	followUpReminderService *FollowUpReminderService
}

// @PostRoute
func (controller *RemindersTickController) PostTick(request *http.Request) *http.Response {
	return RemindersRoute{Reminders: controller.followUpReminderService}.Tick(request)
}
