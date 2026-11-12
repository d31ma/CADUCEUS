// @Service
type InboundService struct {
	// @Needs
	workflows *WorkflowService
	// @Needs
	envelopes *EnvelopeConversionService
	// @Needs
	contents *ContentConversionService
	// @Needs
	failures *FailureService
}

func (service *InboundService) Accept(envelope InboundEnvelope) (InboundAccepted, error) {
	inboundId, failed := service.workflows.Inbox().Accept(service.envelopes.Envelope(envelope))
	return InboundAccepted{InboundId: inboundId}, service.failures.Translate(failed)
}

func (service *InboundService) Original(authorization string, inboundId string) (Download, error) {
	download, failed := service.workflows.Messages().Original(authorization, inboundId)
	return Download{Url: download.Url, ExpiresAt: download.ExpiresAt}, service.failures.Translate(failed)
}

func (service *InboundService) Content(authorization string, inboundId string) (InboundContent, error) {
	content, failed := service.workflows.Messages().Content(authorization, inboundId)
	return service.contents.Content(content), service.failures.Translate(failed)
}
