import "mesail.dev/packages/oidc"

// @RequestRoute("/api/v1/messaging/attachments")
// @SignedIn
type AttachmentsController struct {
	// @Needs
	callers *CallerService
	// @Needs
	messaging *MessagingService
}

// @GetRoute("/{attachmentId}/access")
// @ResponseStatus(204)
func (controller *AttachmentsController) GetAccess(
	// @FromHeader("authorization")
	authorization string,
	// @FromPath
	attachmentId string,
) error {
	caller, refused := controller.callers.ChatCaller(authorization)
	return controller.access(caller, attachmentId, refused)
}

func (controller *AttachmentsController) access(caller oidc.Caller, attachmentId string, failure error) error {
	if failure != nil {
		return failure
	}
	return controller.messaging.AttachmentAccess(caller, attachmentId)
}
