package storage

import (
	"cmp"

	"mesail.dev/packages/core"
)

type Registry struct {
	Documents Documents
	Names     CollectionNames
}

func (registry Registry) Register(collection string) error {
	known, failure := registry.known(collection)
	return cmp.Or(failure, registry.addUnless(known, failure, collection))
}

func (registry Registry) All() ([]string, error) {
	records, failure := registry.matching(map[string]any{})
	return registry.namesOf(records), failure
}

func (registry Registry) Check(collection string) error {
	known, failure := registry.known(collection)
	return cmp.Or(failure, registry.refusal(known))
}

func (registry Registry) known(collection string) (bool, error) {
	records, failure := registry.matching(map[string]any{registry.nameField(): collection})
	return len(records) > 0, failure
}

func (registry Registry) matching(where map[string]any) ([]CollectionRecord, error) {
	records := []CollectionRecord{}
	failure := registry.Documents.Ensure(registry.Names.Registry())
	return records, cmp.Or(failure, registry.readUnless(failure, where, &records))
}

func (registry Registry) readUnless(failure error, where map[string]any, records *[]CollectionRecord) error {
	if failure != nil {
		return nil
	}
	return registry.Documents.All(registry.Names.Registry(), where, records)
}

func (registry Registry) addUnless(known bool, failure error, collection string) error {
	if known || failure != nil {
		return nil
	}
	_, insertFailure := registry.Documents.Insert(registry.Names.Registry(), CollectionRecord{Name: collection})
	return insertFailure
}

func (Registry) namesOf(records []CollectionRecord) []string {
	names := []string{}
	for _, record := range records {
		names = append(names, record.Name)
	}
	return names
}

func (Registry) refusal(known bool) error {
	if known {
		return nil
	}
	return &core.NotFound{Message: "no such outbox collection"}
}

func (Registry) nameField() string {
	return "name"
}
