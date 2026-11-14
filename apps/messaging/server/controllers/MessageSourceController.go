// @RequestRoute("/api/v1/messaging/messages")
// @Allow(Service)
type MessageSourceController struct {
	// @Needs
	sources *SourceRouteService
}

// @GetRoute("/{message_id}/source")
func (controller *MessageSourceController) GetSource(
	// @FromHeader("authorization")
	authorization string,
	// @FromPath("message_id")
	messageId string,
) (MessageSource, error) {
	return controller.sources.Source(authorization, messageId)
}
