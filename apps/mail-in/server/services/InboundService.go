import "time"

// @Service
type InboundService struct {
	// @Needs
	envelopeCheckService *EnvelopeCheckService
	// @Needs
	inboundDocumentService *InboundDocumentService
	// @Needs
	inboundRawService *InboundRawService
	// @Needs
	inboundEventService *InboundEventService
}

func (service *InboundService) Accept(envelope InboundEnvelope) (InboundAccepted, error) {
	domainId, err := service.envelopeCheckService.DomainOf(envelope)
	if err != nil {
		return InboundAccepted{}, err
	}
	collection, err := service.inboundRawService.Collection(domainId)
	if err != nil {
		return InboundAccepted{}, err
	}
	stored, err := service.findOrInsert(collection, domainId, envelope)
	if err != nil {
		return InboundAccepted{}, err
	}
	err = service.queueEvent(collection, stored, envelope)
	return InboundAccepted{InboundId: service.inboundDocumentService.WireId(stored)}, err
}

func (service *InboundService) findOrInsert(collection string, domainId string, envelope InboundEnvelope) (InboundRaw, error) {
	matches, err := service.inboundRawService.FindByBlobKey(collection, envelope.RawBlobKey)
	if err != nil || len(matches) > 0 {
		return service.first(matches), err
	}
	return service.insert(collection, domainId, envelope)
}

func (service *InboundService) first(matches []InboundRaw) InboundRaw {
	return append(matches, InboundRaw{})[0]
}

func (service *InboundService) insert(collection string, domainId string, envelope InboundEnvelope) (InboundRaw, error) {
	document := service.inboundDocumentService.New(domainId, envelope, service.now())
	return service.inboundRawService.Insert(collection, document)
}

func (service *InboundService) queueEvent(collection string, stored InboundRaw, envelope InboundEnvelope) error {
	if stored.EventQueued {
		return nil
	}
	return service.addEvent(collection, stored, envelope)
}

func (service *InboundService) addEvent(collection string, stored InboundRaw, envelope InboundEnvelope) error {
	patch, err := service.inboundEventService.Stage(stored, envelope)
	if err != nil {
		return err
	}
	return service.inboundRawService.SaveEvent(collection, stored.ID, patch)
}

func (service *InboundService) now() string {
	return time.Now().UTC().Format(time.RFC3339Nano)
}
