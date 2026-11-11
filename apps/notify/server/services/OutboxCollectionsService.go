// @Service
type OutboxCollectionsService struct {
	// @Needs
	documents *DocumentsService
	// @Needs
	names *CollectionNamesService
}

func (registry *OutboxCollectionsService) Register(collection string) error {
	_, failure := registry.documents.InsertIfAbsent(registry.names.OutboxCollections(), registry.byName(collection), OutboxCollectionRecord{Name: collection})
	return failure
}

func (registry *OutboxCollectionsService) All(limit int) ([]string, error) {
	records := []OutboxCollectionRecord{}
	failure := registry.documents.Matching(registry.names.OutboxCollections(), map[string]any{}, limit, &records)
	return registry.namesOf(records), failure
}

func (registry *OutboxCollectionsService) Check(collection string) error {
	records := []OutboxCollectionRecord{}
	failure := registry.documents.Matching(registry.names.OutboxCollections(), registry.byName(collection), 1, &records)
	return registry.refusal(failure, len(records) > 0)
}

func (registry *OutboxCollectionsService) byName(collection string) map[string]any {
	return map[string]any{"name": collection}
}

func (registry *OutboxCollectionsService) namesOf(records []OutboxCollectionRecord) []string {
	names := []string{}
	for _, record := range records {
		names = append(names, record.Name)
	}
	return names
}

func (registry *OutboxCollectionsService) refusal(failure error, known bool) error {
	if failure != nil || known {
		return failure
	}
	return &NotFound{What: "outbox collection"}
}
