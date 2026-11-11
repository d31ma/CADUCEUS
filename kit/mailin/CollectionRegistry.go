package mailin

import "slices"

type CollectionRegistry struct {
	Storage Storage
}

func (CollectionRegistry) Name() string {
	return "outbox-collections"
}

func (registry CollectionRegistry) Register(collection string) error {
	known, err := registry.All()
	if err != nil {
		return err
	}
	if slices.Contains(known, collection) {
		return nil
	}
	_, err = registry.Storage.Insert(registry.Name(), CollectionRecord{Name: collection})
	return err
}

func (registry CollectionRegistry) All() ([]string, error) {
	err := registry.Storage.Ensure(registry.Name())
	if err != nil {
		return nil, err
	}
	records := []CollectionRecord{}
	err = registry.Storage.All(registry.Name(), map[string]any{}, &records)
	return registry.namesOf(records), err
}

func (registry CollectionRegistry) Check(collection string) error {
	known, err := registry.All()
	if err != nil {
		return err
	}
	if !slices.Contains(known, collection) {
		return Missing{Message: "no such outbox collection"}
	}
	return nil
}

func (CollectionRegistry) namesOf(records []CollectionRecord) []string {
	names := []string{}
	for _, record := range records {
		names = append(names, record.Name)
	}
	return names
}
