import "mesail.dev/packages/devicebox"

// @RequestRoute("/api/v1/devicebox/jobs")
// @Allow(Service)
type JobsController struct {
	// @Needs
	mailbox *MailboxService
}

// @PostRoute("/expire")
func (controller *JobsController) PostExpire(
	// @FromHeader("authorization")
	authorization string,
) (devicebox.ExpireReport, error) {
	return controller.mailbox.Expire(authorization)
}
