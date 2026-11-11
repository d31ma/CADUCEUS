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
	service.State = service.answer(event.Type)
}

// @OnEvent({"replay":true})
// @InBackground
func (service *RealtimeService) DomainChanged(changed SessionDomainChanged) {
	service.State = service.stateAfter(service.authenticate())
}

func (service *RealtimeService) answer(eventType string) string {
	switch eventType {
	case "hello", "reauth", "auth.expired":
		return service.stateAfter(service.authenticate())
	case "auth.accepted":
		return "live"
	}
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
