package mailout

import (
	"encoding/json"
	"errors"

	"mesail.dev/packages/collections"
)

type Registry struct {
	Store DocumentStore
}

func (Registry) collection() string { return "outbox-collections" }

func (registry Registry) Register(base string, domainId string) error {
	entry := CollectionEntry{Collection: collections.CollectionName{}.ForDomain(base, domainId), Base: base, DomainId: domainId}
	err := registry.Store.Documents.CreateCollection(registry.collection())
	if err != nil {
		return err
	}
	known, err := registry.Store.Exists(registry.collection(), map[string]any{"collection": entry.Collection})
	if err != nil || known {
		return err
	}
	_, err = registry.Store.Documents.Insert(registry.collection(), entry)
	return err
}

func (registry Registry) Collections(base string) ([]CollectionEntry, error) {
	err := registry.Store.Documents.CreateCollection(registry.collection())
	if err != nil {
		return nil, err
	}
	documents, err := registry.Store.All(registry.collection(), map[string]any{"base": base})
	entries := []CollectionEntry{}
	for _, document := range documents {
		entry := CollectionEntry{}
		err = errors.Join(err, json.Unmarshal(document, &entry))
		entries = append(entries, entry)
	}
	return entries, err
}
