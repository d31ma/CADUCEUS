// @Service
type InboundRawService struct {
	// @Needs
	dataSource DataSource
}

func (service *InboundRawService) EnsureCollection(collection string) error {
	exists, err := service.dataSource.CollectionExists(collection)
	if err != nil {
		return service.failure("collection check", err)
	}
	return service.failure("create collection", service.create(collection, exists))
}

func (service *InboundRawService) FindByBlobKey(collection string, key string) ([]InboundRaw, error) {
	window, err := service.dataSource.Scroll(collection, map[string]any{"rawBlobKey": key}, "", 1)
	if err != nil {
		return nil, service.failure("find", err)
	}
	found := []InboundRaw{}
	return found, service.failure("decode", window.Decode(&found))
}

func (service *InboundRawService) Insert(collection string, document InboundRaw) (InboundRaw, error) {
	id, err := service.dataSource.Insert(collection, document)
	document.ID = id
	return document, service.failure("insert", err)
}

func (service *InboundRawService) AddEvent(collection string, id string, patch InboundEventPatch) error {
	_, err := service.dataSource.Patch(collection, id, patch, nil)
	return service.failure("event", err)
}

func (service *InboundRawService) create(collection string, exists bool) error {
	if exists {
		return nil
	}
	return service.dataSource.CreateCollection(collection)
}

func (service *InboundRawService) failure(operation string, err error) error {
	if err == nil {
		return nil
	}
	return &StorageUnavailable{Operation: operation}
}
