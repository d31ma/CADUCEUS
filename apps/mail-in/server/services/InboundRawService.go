// @Service
type InboundRawService struct {
	// @Needs
	documents *DocumentsService
	// @Needs
	names *CollectionNamesService
	// @Needs
	registry *OutboxCollectionsService
	// @Needs
	relay *OutboxRelayService
}

func (service *InboundRawService) Collection(domainId string) (string, error) {
	collection := service.names.InboundRaw(domainId)
	err := service.documents.Ensure(collection)
	if err != nil {
		return "", err
	}
	return collection, service.registry.Register(collection)
}

func (service *InboundRawService) FindByBlobKey(collection string, key string) ([]InboundRaw, error) {
	found := []InboundRaw{}
	err := service.documents.All(collection, map[string]any{"rawBlobKey": key}, &found)
	return found, err
}

func (service *InboundRawService) Insert(collection string, document InboundRaw) (InboundRaw, error) {
	id, err := service.documents.Insert(collection, document)
	document.ID = id
	return document, err
}

func (service *InboundRawService) SaveEvent(collection string, id string, patch InboundEventPatch) error {
	err := service.documents.Replace(collection, id, patch)
	if err != nil {
		return err
	}
	service.relay.Drain(collection, id)
	return nil
}
