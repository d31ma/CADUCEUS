import "net/http"

// @RequestRoute("/api/v1/notify/messages")
// @Allow(Service)
type MessageSourceController struct {
	// @Needs
	sources *MessageSourceService
}

// @GetRoute("/{message_id}/source")
func (controller *MessageSourceController) GetSource(
	// @FromHeader("authorization")
	authorization string,
	// @FromPath("message_id")
	messageId string,
	request *http.Request,
) (MessageSource, error) {
	return controller.sources.Source(authorization, messageId, request.URL.Query().Get("domainId"))
}
