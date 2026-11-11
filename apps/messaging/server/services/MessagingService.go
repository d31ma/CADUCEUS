import (
	"mesail.dev/packages/core"
	"mesail.dev/packages/oidc"
	"mesail.dev/packages/reading"
	"mesail.dev/packages/sending"
)

// @Service
type MessagingService struct {
	// @Needs
	core *CoreService
	// @Needs
	failures *FailureService
}

func (service *MessagingService) Conversations(caller oidc.Caller, page core.PageRequest) (core.ConversationList, error) {
	result, err := reading.NewReader(service.core.Stores(), service.core.Ports()).Conversations(caller, page)
	return result, service.failures.Known(err)
}

func (service *MessagingService) Messages(caller oidc.Caller, query MessageQuery) (core.MessageList, error) {
	result, err := reading.NewReader(service.core.Stores(), service.core.Ports()).Messages(caller, query.ConversationId, query.ThreadRootId, query.Page)
	return result, service.failures.Known(err)
}

func (service *MessagingService) Message(caller oidc.Caller, messageId string) (core.MessageView, error) {
	result, err := reading.NewReader(service.core.Stores(), service.core.Ports()).Message(caller, messageId)
	return result, service.failures.Known(err)
}

func (service *MessagingService) AttachmentAccess(caller oidc.Caller, attachmentId string) error {
	return service.failures.Known(reading.NewAttachments(service.core.Stores()).Access(caller, attachmentId))
}

func (service *MessagingService) MoveReadPosition(caller oidc.Caller, conversationId string, messageId string) error {
	return service.failures.Known(reading.NewReadPositions(service.core.Stores(), service.core.Ports()).Move(caller, conversationId, messageId))
}

func (service *MessagingService) Send(caller oidc.Caller, authorization string, request core.SendRequest) (core.SendOutcome, error) {
	result, err := sending.NewSender(service.core.Stores(), service.core.Ports()).Send(caller, authorization, request)
	return result, service.failures.Known(err)
}

func (service *MessagingService) Edit(caller oidc.Caller, messageId string, body string) (core.MessageView, error) {
	result, err := reading.NewChanges(service.core.Stores(), service.core.Ports()).Edit(caller, messageId, body)
	return result, service.failures.Known(err)
}

func (service *MessagingService) Delete(caller oidc.Caller, messageId string) error {
	return service.failures.Known(reading.NewChanges(service.core.Stores(), service.core.Ports()).Delete(caller, messageId))
}

func (service *MessagingService) AddReaction(caller oidc.Caller, messageId string, reaction string) error {
	return service.failures.Known(reading.NewReactions(service.core.Stores(), service.core.Ports()).Add(caller, messageId, reaction))
}

func (service *MessagingService) RemoveReaction(caller oidc.Caller, messageId string, reaction string) error {
	return service.failures.Known(reading.NewReactions(service.core.Stores(), service.core.Ports()).Remove(caller, messageId, reaction))
}

func (service *MessagingService) Original(caller oidc.Caller, messageId string) (core.Download, error) {
	result, err := reading.NewSources(service.core.Stores(), service.core.Ports()).Original(caller, messageId)
	return result, service.failures.Known(err)
}

func (service *MessagingService) Source(caller oidc.Caller, messageId string) (core.MessageSource, error) {
	result, err := reading.NewSources(service.core.Stores(), service.core.Ports()).Source(caller, messageId)
	return result, service.failures.Known(err)
}
