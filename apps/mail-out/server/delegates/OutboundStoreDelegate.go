import (
	"errors"
	"slices"

	"mesail.dev/packages/events"
	"mesail.dev/packages/outbox"
)

// @Delegate
// @NeedsAll()
type OutboundStoreDelegate struct {
	dataSource DataSource
	registry   *CollectionRegistryDelegate
	settings   *SettingsDelegate
}

func (OutboundStoreDelegate) pageSize() int { return 100 }

func (OutboundStoreDelegate) firstPage() string { return "" }

func (store *OutboundStoreDelegate) Insert(item OutboundItem, envelope events.Envelope) (OutboundItem, error) {
	collection, err := store.ready(item.DomainId)
	registerErr := store.registry.Register(StorageUtils{}.OutboundQueue(), item.DomainId)
	fields, stageErr := store.stage(outbox.Fields{}, envelope)
	item = OutboxUtils{}.WithFields(item, fields)
	item.Id = ""
	id, insertErr := store.insert(collection, item, errors.Join(err, registerErr, stageErr))
	item.Id = id
	return item, insertErr
}

func (store *OutboundStoreDelegate) Save(item OutboundItem) error {
	collection, err := store.ready(item.DomainId)
	current, loadErr := store.current(collection, item.Id)
	return store.patch(collection, OutboxUtils{}.WithFields(item, OutboxUtils{}.Fields(current)), errors.Join(err, loadErr))
}

func (store *OutboundStoreDelegate) SaveWith(item OutboundItem, envelope events.Envelope) error {
	collection, err := store.ready(item.DomainId)
	current, loadErr := store.current(collection, item.Id)
	fields, stageErr := store.stage(OutboxUtils{}.Fields(current), envelope)
	return store.patch(collection, OutboxUtils{}.WithFields(item, fields), errors.Join(err, loadErr, stageErr))
}

func (store *OutboundStoreDelegate) Claim(item OutboundItem, now string) (OutboundItem, bool, error) {
	collection, err := store.ready(item.DomainId)
	current, loadErr := store.current(collection, item.Id)
	free := store.free(current, now)
	item.State = OutboundStateUtils{}.Sending()
	item.NextAttemptAt = TimeUtils{}.Format(TimeUtils{}.Parse(now).Add(store.settings.Lease()))
	claimed := OutboxUtils{}.WithFields(item, OutboxUtils{}.Fields(current))
	return claimed, free, store.claim(collection, claimed, free, errors.Join(err, loadErr))
}

func (store *OutboundStoreDelegate) FindBy(domainId string, field string, value string) (OutboundItem, bool, error) {
	collection, err := store.ready(domainId)
	items, _, pageErr := store.page(collection, map[string]any{field: value}, store.firstPage())
	item, found := store.first(items)
	return item, found, errors.Join(err, pageErr)
}

func (store *OutboundStoreDelegate) Due(domainId string, now string) ([]OutboundItem, error) {
	collection, err := store.ready(domainId)
	states := OutboundStateUtils{}
	queued, queuedErr := store.inState(collection, states.Queued())
	deferred, deferredErr := store.inState(collection, states.Deferred())
	sending, sendingErr := store.inState(collection, states.Sending())
	due := store.dueOnly(slices.Concat(queued, deferred, sending), now)
	return due, errors.Join(err, queuedErr, deferredErr, sendingErr)
}

func (store *OutboundStoreDelegate) ready(domainId string) (string, error) {
	collection := StorageUtils{}.CollectionName(StorageUtils{}.OutboundQueue(), domainId)
	return collection, store.dataSource.CreateCollection(collection)
}

func (store *OutboundStoreDelegate) stage(fields outbox.Fields, envelope events.Envelope) (outbox.Fields, error) {
	settings, err := store.settings.Outbox()
	staged, stageErr := outbox.NewOutbox(settings).Stage(fields, envelope)
	return staged, errors.Join(err, stageErr)
}

func (store *OutboundStoreDelegate) current(collection string, id string) (OutboundItem, error) {
	current := OutboundItem{}
	_, err := store.dataSource.FindById(collection, id, &current)
	return current, err
}

func (OutboundStoreDelegate) free(current OutboundItem, now string) bool {
	taken := current.State == OutboundStateUtils{}.Sending() && TimeUtils{}.Parse(current.NextAttemptAt).After(TimeUtils{}.Parse(now))
	return !taken
}

func (store *OutboundStoreDelegate) claim(collection string, item OutboundItem, free bool, failure error) error {
	if !free || failure != nil {
		return failure
	}
	id := item.Id
	item.Id = ""
	_, patchErr := store.dataSource.Patch(collection, id, item, &OutboundItem{})
	return patchErr
}

func (store *OutboundStoreDelegate) insert(collection string, item OutboundItem, failure error) (string, error) {
	if failure != nil {
		return "", failure
	}
	return store.dataSource.Insert(collection, item)
}

func (store *OutboundStoreDelegate) patch(collection string, item OutboundItem, failure error) error {
	if failure != nil {
		return failure
	}
	id := item.Id
	item.Id = ""
	_, patchErr := store.dataSource.Patch(collection, id, item, &OutboundItem{})
	return patchErr
}

func (store *OutboundStoreDelegate) inState(collection string, state string) ([]OutboundItem, error) {
	where := map[string]any{"state": state}
	items, cursor, err := store.page(collection, where, store.firstPage())
	for cursor != store.firstPage() {
		more, next, pageErr := store.page(collection, where, cursor)
		items, cursor, err = append(items, more...), next, errors.Join(err, pageErr)
	}
	return items, err
}

func (store *OutboundStoreDelegate) page(collection string, where map[string]any, cursor string) ([]OutboundItem, string, error) {
	window, err := store.dataSource.Scroll(collection, where, cursor, store.pageSize())
	items := []OutboundItem{}
	decodeErr := window.Decode(&items)
	return items, window.NextCursor, errors.Join(err, decodeErr)
}

func (OutboundStoreDelegate) first(items []OutboundItem) (OutboundItem, bool) {
	if len(items) == 0 {
		return OutboundItem{}, false
	}
	return items[0], true
}

func (OutboundStoreDelegate) dueOnly(items []OutboundItem, now string) []OutboundItem {
	return slices.DeleteFunc(items, func(item OutboundItem) bool {
		return TimeUtils{}.Parse(item.NextAttemptAt).After(TimeUtils{}.Parse(now))
	})
}
