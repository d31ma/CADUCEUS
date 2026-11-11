package channels

import "slices"

type Registry struct {
	documents Documents
}

func NewRegistry(documents Documents) Registry {
	return Registry{documents: documents}
}

func (registry Registry) Register(collection string) error {
	known, err := registry.known(collection)
	if err != nil {
		return err
	}
	return registry.addUnless(known, collection)
}

func (registry Registry) All() ([]string, error) {
	records, err := registry.matching(map[string]any{})
	return registry.namesOf(records), err
}

func (registry Registry) Check(collection string) error {
	known, err := registry.known(collection)
	if err != nil {
		return err
	}
	return registry.refusal(known)
}

func (registry Registry) known(collection string) (bool, error) {
	records, err := registry.matching(map[string]any{registry.nameField(): collection})
	return slices.Contains(registry.namesOf(records), collection), err
}

func (registry Registry) addUnless(known bool, collection string) error {
	if known {
		return nil
	}
	_, err := registry.documents.Insert(Names{}.Registry(), CollectionRecord{Name: collection})
	return err
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
	return &NotFound{Message: "no such outbox collection"}
}

func (registry Registry) matching(where map[string]any) ([]CollectionRecord, error) {
	records := []CollectionRecord{}
	err := registry.documents.Ensure(Names{}.Registry())
	if err != nil {
		return records, err
	}
	err = registry.documents.All(Names{}.Registry(), where, &records)
	return records, err
}

func (Registry) nameField() string {
	return "name"
}
