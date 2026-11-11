// @Service
type RealtimeService struct {
	State string
	// @Needs
	realtimeClient *RealtimeClient
	// @Needs
	token *ConversationTokenService
}

// @OnStart
func (service *RealtimeService) Start() {
	service.State = service.stateConnecting()
}

// @OnEvent
// @InBackground
func (service *RealtimeService) Received(event RealtimeEvent) {
	service.State = service.answerFor(event.Type)()
}

// @OnEvent({"replay":true})
// @InBackground
func (service *RealtimeService) DomainChanged(changed SessionDomainChanged) {
	service.State = service.reconnect()
}

func (service *RealtimeService) answerFor(eventType string) func() string {
	switch eventType {
	case "hello", "reauth", "auth.expired":
		return service.reconnect
	case "auth.accepted":
		return service.live
	}
	return service.unchanged
}

func (service *RealtimeService) reconnect() string {
	return service.stateAfter(service.authenticate())
}

func (service *RealtimeService) live() string {
	return "live"
}

func (service *RealtimeService) unchanged() string {
	return service.State
}

func (service *RealtimeService) authenticate() error {
	authorization, failure := service.token.AccessToken()
	if failure != nil {
		return failure
	}
	return service.realtimeClient.Send(RealtimeAuthentication{Type: service.authenticationType(), Token: authorization})
}

func (service *RealtimeService) stateAfter(failure error) string {
	if failure != nil {
		return "offline"
	}
	return service.stateConnecting()
}

func (service *RealtimeService) stateConnecting() string {
	return "connecting"
}

func (service *RealtimeService) authenticationType() string {
	return "auth"
}
