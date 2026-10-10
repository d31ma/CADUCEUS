import "net/http"

// @RequestRoute("/thread-shares/public/{token}")
// @Public
type ThreadSharesPublicTokenController struct {
	// @Needs
	threadSharingService *ThreadSharingService
}

// @GetRoute
func (controller *ThreadSharesPublicTokenController) GetSharedConversation(
	// @FromPath
	token string,
) *http.Response {
	return ThreadSharesRoute{Sharing: controller.threadSharingService}.Show(token)
}
