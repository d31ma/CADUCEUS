package mailout

import (
	"errors"

	"mesail.dev/packages/collections"
)

type Registry struct {
	Store DocumentStore
}

func (Registry) collection() string { return "outbox-collections" }

func (Registry) collectionField() string { return "collection" }

func (Registry) baseField() string { return "base" }

func (registry Registry) Register(base string, domainId string) error {
	entry := CollectionEntry{Collection: collections.CollectionName{}.ForDomain(base, domainId), Base: base, DomainId: domainId}
	err := registry.Store.Documents.CreateCollection(registry.collection())
	if err != nil {
		return err
	}
	known, err := registry.Store.Exists(registry.collection(), map[string]any{registry.collectionField(): entry.Collection})
	if err != nil {
		return err
	}
	return registry.insertUnlessKnown(known, entry)
}

func (registry Registry) Collections(base string) ([]CollectionEntry, error) {
	err := registry.Store.Documents.CreateCollection(registry.collection())
	if err != nil {
		return nil, err
	}
	documents, err := registry.Store.All(registry.collection(), map[string]any{registry.baseField(): base})
	entries, decodeErr := documentDecoder[CollectionEntry]{}.decode(documents)
	return entries, errors.Join(err, decodeErr)
}

func (registry Registry) insertUnlessKnown(known bool, entry CollectionEntry) error {
	if known {
		return nil
	}
	_, err := registry.Store.Documents.Insert(registry.collection(), entry)
	return err
}
