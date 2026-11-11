package mailout

import (
	"errors"
	"time"
)

type QueueRun struct {
	Registry Registry
	Queue    Queue
	Delivery Delivery
	Batch    int
}

func (run QueueRun) Run(now time.Time) (QueueReport, error) {
	entries, err := run.Registry.Collections(run.Queue.Base())
	if err != nil {
		return QueueReport{}, err
	}
	due := []Item{}
	for _, entry := range entries {
		items, err := run.Queue.Due(entry.DomainId, now)
		if err != nil {
			return QueueReport{}, err
		}
		due = append(due, items...)
	}
	due = due[:min(len(due), run.Batch)]
	counts := map[string]int{}
	for _, item := range due {
		done, deliverErr := run.Delivery.Deliver(item, now)
		counts[done.State]++
		err = errors.Join(err, deliverErr)
	}
	states := States{}
	return QueueReport{Picked: len(due), Sent: counts[states.Sent()], Deferred: counts[states.Deferred()], Bounced: counts[states.Bounced()], Expired: counts[states.Expired()], Failed: counts[states.Failed()]}, err
}
