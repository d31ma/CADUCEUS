// @Service
type ThreadRemoteService struct {
	// @Needs
	client *ThreadClient
	// @Needs
	session *SessionService
	// @Needs
	accounts *SessionAccountService
	// @Needs
	random SecureRandom
}

func (remote *ThreadRemoteService) Page(requested ThreadPageRequest) (MessagePage, error) {
	token, failure := remote.session.AccessToken()
	page, failure := remote.page(token, failure, requested)
	return remote.pageAfterRenew(page, failure, requested)
}

func (remote *ThreadRemoteService) Message(messageId string) (Message, error) {
	token, failure := remote.session.AccessToken()
	message, failure := remote.message(token, failure, messageId)
	return remote.messageAfterRenew(message, failure, messageId)
}

func (remote *ThreadRemoteService) Send(request ThreadReplyRequest) (ThreadSendResult, error) {
	token, failure := remote.session.AccessToken()
	result, failure := remote.send(token, failure, request)
	return remote.sendAfterRenew(result, failure, request)
}

func (remote *ThreadRemoteService) NewId() string {
	id, _ := remote.random.UUID()
	return id
}

func (remote *ThreadRemoteService) MyName() string {
	return remote.accounts.Current().DisplayName
}

func (remote *ThreadRemoteService) page(token string, failure error, requested ThreadPageRequest) (MessagePage, error) {
	return BehaviorsAnswer[MessagePage](remote.fetch(token, failure, requested))
}

func (remote *ThreadRemoteService) message(token string, failure error, messageId string) (Message, error) {
	return BehaviorsAnswer[Message](remote.read(token, failure, messageId))
}

func (remote *ThreadRemoteService) send(token string, failure error, request ThreadReplyRequest) (ThreadSendResult, error) {
	return BehaviorsAnswer[ThreadSendResult](remote.post(token, failure, request))
}

func (remote *ThreadRemoteService) pageAfterRenew(page MessagePage, failure error, requested ThreadPageRequest) (MessagePage, error) {
	if BehaviorErrorStatus(failure) != remote.unauthorized() {
		return page, failure
	}
	token, renewFailure := remote.session.Renew()
	return remote.page(token, renewFailure, requested)
}

func (remote *ThreadRemoteService) messageAfterRenew(message Message, failure error, messageId string) (Message, error) {
	if BehaviorErrorStatus(failure) != remote.unauthorized() {
		return message, failure
	}
	token, renewFailure := remote.session.Renew()
	return remote.message(token, renewFailure, messageId)
}

func (remote *ThreadRemoteService) sendAfterRenew(result ThreadSendResult, failure error, request ThreadReplyRequest) (ThreadSendResult, error) {
	if BehaviorErrorStatus(failure) != remote.unauthorized() {
		return result, failure
	}
	token, renewFailure := remote.session.Renew()
	return remote.send(token, renewFailure, request)
}

func (remote *ThreadRemoteService) fetch(token string, failure error, requested ThreadPageRequest) (BehaviorValue, error) {
	switch {
	case failure != nil:
		return nil, remote.sessionEnded()
	case requested.Cursor == "":
		return remote.client.Replies(remote.bearer(token), requested.ConversationId, requested.RootId)
	}
	return remote.client.OlderReplies(remote.bearer(token), requested.ConversationId, requested.RootId, requested.Cursor)
}

func (remote *ThreadRemoteService) read(token string, failure error, messageId string) (BehaviorValue, error) {
	if failure != nil {
		return nil, remote.sessionEnded()
	}
	return remote.client.Message(remote.bearer(token), messageId)
}

func (remote *ThreadRemoteService) post(token string, failure error, request ThreadReplyRequest) (BehaviorValue, error) {
	if failure != nil {
		return nil, remote.sessionEnded()
	}
	return remote.client.Send(remote.bearer(token), request)
}

func (remote *ThreadRemoteService) bearer(token string) string {
	return "Bearer " + token
}

func (remote *ThreadRemoteService) sessionEnded() error {
	return &BehaviorError{Status: remote.unauthorized()}
}

func (remote *ThreadRemoteService) unauthorized() int {
	return 401
}
