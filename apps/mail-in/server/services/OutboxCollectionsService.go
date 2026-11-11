import "slices"

// @Service
type OutboxCollectionsService struct {
	// @Needs
	documents *DocumentsService
	// @Needs
	names *CollectionNamesService
}

func (registry *OutboxCollectionsService) Register(collection string) error {
	known, err := registry.All()
	if err != nil {
		return err
	}
	return registry.addUnless(slices.Contains(known, collection), collection)
}

func (registry *OutboxCollectionsService) All() ([]string, error) {
	err := registry.documents.Ensure(registry.names.Registry())
	if err != nil {
		return nil, err
	}
	records := []CollectionRecord{}
	err = registry.documents.All(registry.names.Registry(), map[string]any{}, &records)
	return registry.namesOf(records), err
}

func (registry *OutboxCollectionsService) Check(collection string) error {
	known, err := registry.All()
	if err != nil {
		return err
	}
	return registry.refusal(slices.Contains(known, collection))
}

func (registry *OutboxCollectionsService) addUnless(known bool, collection string) error {
	if known {
		return nil
	}
	_, err := registry.documents.Insert(registry.names.Registry(), CollectionRecord{Name: collection})
	return err
}

func (registry *OutboxCollectionsService) namesOf(records []CollectionRecord) []string {
	names := []string{}
	for _, record := range records {
		names = append(names, record.Name)
	}
	return names
}

func (registry *OutboxCollectionsService) refusal(known bool) error {
	if known {
		return nil
	}
	return &NotFound{Message: "no such outbox collection"}
}
