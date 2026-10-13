// InboxStreamController keeps the live new-mail socket of a signed-in
// mailbox open; see InboxStreamRoute. Tachyon gives a socket no principal,
// so the route authenticates the ?token= (or bearer header) itself, the way
// the HTTP routes do, and answers a refused token with an unauthorized
// message.
// @SocketRoute("/inbox/stream")
// @Public
type InboxStreamController struct {
	// @Needs
	accessHelper *AccessHelper
	// @Needs
	categoryMappingService *CategoryMappingService
	// @Needs
	emailsHelper *EmailsHelper
}

// @OnOpen
func (controller *InboxStreamController) OnOpen(session *SocketSession) error {
	return controller.route().Open(session)
}

// @OnMessage
func (controller *InboxStreamController) OnMessage(session *SocketSession, message InboxStreamMessage) error {
	return controller.route().Received(session, message)
}

func (controller *InboxStreamController) route() InboxStreamRoute {
	return InboxStreamRoute{Access: controller.accessHelper, Categories: controller.categoryMappingService, Emails: controller.emailsHelper}
}
