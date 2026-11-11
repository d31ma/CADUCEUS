import (
	"errors"
	"time"
)

// @Service
type QueueService struct {
	// @Needs
	registry *CollectionRegistryDelegate
	// @Needs
	outbound *OutboundStoreDelegate
	// @Needs
	delivery *DeliveryService
}

func (QueueService) batch() int { return 50 }

func (queue *QueueService) Run(now time.Time, authorize func(string) (string, error)) (QueueReport, error) {
	entries, err := queue.registry.Collections(StorageUtils{}.OutboundQueue())
	due, dueErr := queue.due(entries, now)
	items := due[:min(len(due), queue.batch())]
	counts, deliverErr := queue.deliverAll(items, now, authorize)
	states := OutboundStateUtils{}
	report := QueueReport{Picked: len(items), Sent: counts[states.Sent()], Deferred: counts[states.Deferred()], Bounced: counts[states.Bounced()], Expired: counts[states.Expired()], Failed: counts[states.Failed()]}
	return report, errors.Join(err, dueErr, deliverErr)
}

func (queue *QueueService) due(entries []CollectionEntry, now time.Time) ([]OutboundItem, error) {
	items := []OutboundItem{}
	var err error
	for _, entry := range entries {
		due, dueErr := queue.outbound.Due(entry.DomainId, TimeUtils{}.Format(now))
		items, err = append(items, due...), errors.Join(err, dueErr)
	}
	return items, err
}

func (queue *QueueService) deliverAll(items []OutboundItem, now time.Time, authorize func(string) (string, error)) (map[string]int, error) {
	counts := map[string]int{}
	var err error
	for _, item := range items {
		done, deliverErr := queue.delivery.Deliver(item, now, authorize)
		counts[done.State]++
		err = errors.Join(err, deliverErr)
	}
	return counts, err
}
