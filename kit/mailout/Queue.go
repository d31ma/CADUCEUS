package mailout

import (
	"slices"
	"time"

	"mesail.dev/packages/collections"
	"mesail.dev/packages/events"
	"mesail.dev/packages/outbox"
)

type Queue struct {
	Store    DocumentStore
	Registry Registry
	Outbox   outbox.Settings
	Lease    time.Duration
}

func (Queue) Base() string { return "outbound-queue" }

func (queue Queue) Ref(item Item) outbox.Ref {
	return outbox.Ref{Collection: collections.CollectionName{}.ForDomain(queue.Base(), item.DomainId), Id: item.Id}
}

func (queue Queue) Insert(item Item, envelope events.Envelope) (Item, error) {
	collection, err := queue.Store.Ready(queue.Base(), item.DomainId)
	if err != nil {
		return Item{}, err
	}
	err = queue.Registry.Register(queue.Base(), item.DomainId)
	if err != nil {
		return Item{}, err
	}
	fields, err := outbox.NewOutbox(queue.Outbox).Stage(outbox.Fields{}, envelope)
	if err != nil {
		return Item{}, err
	}
	item = item.WithOutbox(fields)
	item.Id = ""
	item.Id, err = queue.Store.Documents.Insert(collection, item)
	return item, err
}

func (queue Queue) Save(item Item) error {
	current, err := queue.current(item)
	if err != nil {
		return err
	}
	return queue.patch(item.WithOutbox(current.OutboxFields()))
}

func (queue Queue) SaveWith(item Item, envelope events.Envelope) error {
	current, err := queue.current(item)
	if err != nil {
		return err
	}
	fields, err := outbox.NewOutbox(queue.Outbox).Stage(current.OutboxFields(), envelope)
	if err != nil {
		return err
	}
	return queue.patch(item.WithOutbox(fields))
}

func (queue Queue) Claim(item Item, now time.Time) (Item, bool, error) {
	current, err := queue.current(item)
	if err != nil {
		return item, false, err
	}
	if queue.leased(current, now) {
		return item, false, nil
	}
	item = item.WithOutbox(current.OutboxFields())
	item.State = States{}.Sending()
	item.NextAttemptAt = Clock{}.Format(now.Add(queue.Lease))
	return item, true, queue.patch(item)
}

func (queue Queue) FindBy(domainId string, field string, value string) (Item, bool, error) {
	collection, err := queue.Store.Ready(queue.Base(), domainId)
	if err != nil {
		return Item{}, false, err
	}
	documents, _, err := queue.Store.Page(collection, map[string]any{field: value}, "", 1)
	if err != nil || len(documents) == 0 {
		return Item{}, false, err
	}
	items, err := Decoder{}.Items(documents)
	return items[0], err == nil, err
}

func (queue Queue) Due(domainId string, now time.Time) ([]Item, error) {
	collection, err := queue.Store.Ready(queue.Base(), domainId)
	if err != nil {
		return nil, err
	}
	due := []Item{}
	states := States{}
	for _, state := range []string{states.Queued(), states.Deferred(), states.Sending()} {
		documents, err := queue.Store.All(collection, map[string]any{"state": state})
		if err != nil {
			return nil, err
		}
		items, err := Decoder{}.Items(documents)
		if err != nil {
			return nil, err
		}
		due = append(due, items...)
	}
	return slices.DeleteFunc(due, func(item Item) bool { return Clock{}.Parse(item.NextAttemptAt).After(now) }), nil
}

func (Queue) leased(current Item, now time.Time) bool {
	return current.State == States{}.Sending() && Clock{}.Parse(current.NextAttemptAt).After(now)
}

func (queue Queue) current(item Item) (Item, error) {
	current := Item{}
	_, err := queue.Store.Documents.FindById(queue.Ref(item).Collection, item.Id, &current)
	return current, err
}

func (queue Queue) patch(item Item) error {
	ref := queue.Ref(item)
	item.Id = ""
	_, err := queue.Store.Documents.Patch(ref.Collection, ref.Id, item, &Item{})
	return err
}
