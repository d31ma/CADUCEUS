import "errors"

// @Delegate
// @NeedsAll()
type CollectionRegistryDelegate struct {
	dataSource DataSource
}

func (CollectionRegistryDelegate) collection() string { return "outbox-collections" }

func (CollectionRegistryDelegate) pageSize() int { return 100 }

func (registry *CollectionRegistryDelegate) Register(base string, domainId string) error {
	entry := CollectionEntry{Collection: StorageUtils{}.CollectionName(base, domainId), Base: base, DomainId: domainId}
	createErr := registry.dataSource.CreateCollection(registry.collection())
	known, findErr := registry.page(map[string]any{"collection": entry.Collection}, "")
	return registry.insert(entry, len(known.Entries) > 0, errors.Join(createErr, findErr))
}

func (registry *CollectionRegistryDelegate) Collections(base string) ([]CollectionEntry, error) {
	createErr := registry.dataSource.CreateCollection(registry.collection())
	where := map[string]any{"base": base}
	first, err := registry.page(where, "")
	entries, cursor := first.Entries, first.Next
	for cursor != "" {
		more, pageErr := registry.page(where, cursor)
		entries, cursor, err = append(entries, more.Entries...), more.Next, errors.Join(err, pageErr)
	}
	return entries, errors.Join(createErr, err)
}

func (registry *CollectionRegistryDelegate) page(where map[string]any, cursor string) (CollectionPage, error) {
	window, err := registry.dataSource.Scroll(registry.collection(), where, cursor, registry.pageSize())
	entries := []CollectionEntry{}
	decodeErr := window.Decode(&entries)
	return CollectionPage{Entries: entries, Next: window.NextCursor}, errors.Join(err, decodeErr)
}

func (registry *CollectionRegistryDelegate) insert(entry CollectionEntry, known bool, failure error) error {
	if known || failure != nil {
		return failure
	}
	_, err := registry.dataSource.Insert(registry.collection(), entry)
	return err
}
