// @Service
type ComposerSendService struct {
	PendingIds map[string]string
	// @Needs
	messaging *MessagingSendClient
	// @Needs
	auth *ComposerAuthService
	// @Needs
	secureRandom SecureRandom
}

func (send *ComposerSendService) Send(composerName string, request SendRequest) (SendResult, error) {
	request.ClientMessageId = send.pendingId(composerName)
	authorization, failure := send.auth.Authorization()
	result, failure := send.sendUnlessFailed(authorization, request, failure)
	result, failure = send.againWhenSignedOut(request, result, failure)
	send.forgetWhenSent(composerName, failure)
	return result, failure
}

func (send *ComposerSendService) Reset(composerName string) {
	delete(send.PendingIds, composerName)
}

func (send *ComposerSendService) pendingId(composerName string) string {
	send.PendingIds = send.withIdWhenMissing(send.idsCopy(), composerName)
	return send.PendingIds[composerName]
}

func (send *ComposerSendService) idsCopy() map[string]string {
	ids := map[string]string{}
	for composerName, id := range send.PendingIds {
		ids[composerName] = id
	}
	return ids
}

func (send *ComposerSendService) withIdWhenMissing(ids map[string]string, composerName string) map[string]string {
	if ids[composerName] != "" {
		return ids
	}
	ids[composerName] = send.newId()
	return ids
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

func (send *ComposerSendService) forgetWhenSent(composerName string, failure error) {
	if failure != nil {
		return
	}
	send.Reset(composerName)
}

func (send *ComposerSendService) unauthorized() int {
	return 401
}
