// @Service
type ThreadRemoteService struct {
	// @Needs
	client *ThreadClient
	// @Needs
	tokens *SessionTokenService
	// @Needs
	accounts *SessionAccountService
	// @Needs
	random SecureRandom
}

func (remote *ThreadRemoteService) Page(requested ThreadPageRequest) (ThreadMessagePage, error) {
	authorization, failure := remote.authorization()
	return BehaviorsAnswer[ThreadMessagePage](remote.fetch(authorization, failure, requested))
}

func (remote *ThreadRemoteService) Message(messageId string) (ThreadMessage, error) {
	authorization, failure := remote.authorization()
	return BehaviorsAnswer[ThreadMessage](remote.read(authorization, failure, messageId))
}

func (remote *ThreadRemoteService) Send(request ThreadReplyRequest) (ThreadSendResult, error) {
	authorization, failure := remote.authorization()
	return BehaviorsAnswer[ThreadSendResult](remote.post(authorization, failure, request))
}

func (remote *ThreadRemoteService) NewId() string {
	id, _ := remote.random.UUID()
	return id
}

func (remote *ThreadRemoteService) MyName() string {
	return remote.accounts.Current().DisplayName
}

func (remote *ThreadRemoteService) authorization() (string, error) {
	token, failure := remote.tokens.AccessToken(remote.accounts.CurrentDomain)
	if failure != nil {
		return "", &BehaviorError{Status: remote.unauthorized()}
	}
	return "Bearer " + token, nil
}

func (remote *ThreadRemoteService) fetch(authorization string, failure error, requested ThreadPageRequest) (BehaviorValue, error) {
	switch {
	case failure != nil:
		return nil, failure
	case requested.Cursor == "":
		return remote.client.Replies(authorization, requested.ConversationId, requested.RootId)
	}
	return remote.client.OlderReplies(authorization, requested.ConversationId, requested.RootId, requested.Cursor)
}

func (remote *ThreadRemoteService) read(authorization string, failure error, messageId string) (BehaviorValue, error) {
	if failure != nil {
		return nil, failure
	}
	return remote.client.Message(authorization, messageId)
}

func (remote *ThreadRemoteService) post(authorization string, failure error, request ThreadReplyRequest) (BehaviorValue, error) {
	if failure != nil {
		return nil, failure
	}
	return remote.client.Send(authorization, request)
}

func (remote *ThreadRemoteService) unauthorized() int {
	return 401
}
