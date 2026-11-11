import (
	"cmp"

	"mesail.dev/packages/core"
)

// @Service
type OutboxCollectionsService struct {
	// @Needs
	documents *DocumentsService
	// @Needs
	names *CollectionNamesService
}

func (registry *OutboxCollectionsService) Register(collection string) error {
	known, failure := registry.known(collection)
	return cmp.Or(failure, registry.addUnless(known, failure, collection))
}

func (registry *OutboxCollectionsService) All() ([]string, error) {
	records, failure := registry.matching(map[string]any{})
	return registry.namesOf(records), failure
}

func (registry *OutboxCollectionsService) Check(collection string) error {
	known, failure := registry.known(collection)
	return cmp.Or(failure, registry.refusal(known))
}

func (registry *OutboxCollectionsService) known(collection string) (bool, error) {
	records, failure := registry.matching(map[string]any{registry.nameField(): collection})
	return len(records) > 0, failure
}

func (registry *OutboxCollectionsService) matching(where map[string]any) ([]CollectionRecord, error) {
	records := []CollectionRecord{}
	failure := registry.documents.Ensure(registry.names.Registry())
	return records, cmp.Or(failure, registry.readUnless(failure, where, &records))
}

func (registry *OutboxCollectionsService) readUnless(failure error, where map[string]any, records *[]CollectionRecord) error {
	if failure != nil {
		return nil
	}
	return registry.documents.All(registry.names.Registry(), where, records)
}

func (registry *OutboxCollectionsService) addUnless(known bool, failure error, collection string) error {
	if known || failure != nil {
		return nil
	}
	_, insertFailure := registry.documents.Insert(registry.names.Registry(), CollectionRecord{Name: collection})
	return insertFailure
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
	return &core.NotFound{Message: "no such outbox collection"}
}

func (registry *OutboxCollectionsService) nameField() string {
	return "name"
}
