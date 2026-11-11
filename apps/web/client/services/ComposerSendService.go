// @Service
type ComposerSendService struct {
	PendingId string
	// @Needs
	messaging *MessagingSendClient
	// @Needs
	auth *ComposerAuthService
	// @Needs
	secureRandom SecureRandom
}

func (send *ComposerSendService) Send(request SendRequest) (SendResult, error) {
	request.ClientMessageId = send.pendingId()
	authorization, failure := send.auth.Authorization()
	result, failure := send.sendUnlessFailed(authorization, request, failure)
	result, failure = send.againWhenSignedOut(request, result, failure)
	send.forgetWhenSent(failure)
	return result, failure
}

func (send *ComposerSendService) Reset() {
	send.PendingId = ""
}

func (send *ComposerSendService) pendingId() string {
	if send.PendingId == "" {
		send.PendingId = send.newId()
	}
	return send.PendingId
}

func (send *ComposerSendService) newId() string {
	id, _ := send.secureRandom.UUID()
	return id
}

func (send *ComposerSendService) sendUnlessFailed(authorization string, request SendRequest, failure error) (SendResult, error) {
	if failure != nil {
		return SendResult{}, failure
	}
	return BehaviorsAnswer[SendResult](send.messaging.Send(authorization, request))
}

func (send *ComposerSendService) againWhenSignedOut(request SendRequest, result SendResult, failure error) (SendResult, error) {
	if BehaviorErrorStatus(failure) != send.unauthorized() {
		return result, failure
	}
	authorization, _ := send.auth.Renewed()
	return BehaviorsAnswer[SendResult](send.messaging.Send(authorization, request))
}

func (send *ComposerSendService) forgetWhenSent(failure error) {
	if failure != nil {
		return
	}
	send.Reset()
}

func (send *ComposerSendService) unauthorized() int {
	return 401
}
