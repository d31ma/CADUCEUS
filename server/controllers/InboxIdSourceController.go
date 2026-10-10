import "net/http"

// @RequestRoute("/inbox/{id}/source")
// @SignedIn
type InboxIdSourceController struct {
	// @Needs
	accessHelper *AccessHelper
	// @Needs
	emailsHelper *EmailsHelper
	// @Needs
	rawMessagesHelper *RawMessagesHelper
}

// @GetRoute
func (controller *InboxIdSourceController) GetSource(
	// @FromPath
	id string,
	request *http.Request,
) *http.Response {
	return InboxSourceRoute{Parts: MailboxParts{
		Access: controller.accessHelper, Emails: controller.emailsHelper, RawMessages: controller.rawMessagesHelper,
	}}.Read(id, request)
}
