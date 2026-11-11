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
	due, err := run.dueIn(entries, now)
	if err != nil {
		return QueueReport{}, err
	}
	due = due[:min(len(due), run.Batch)]
	counts, err := run.deliverAll(due, now)
	return run.report(len(due), counts), err
}

func (run QueueRun) dueIn(entries []CollectionEntry, now time.Time) ([]Item, error) {
	due := []Item{}
	for _, entry := range entries {
		items, err := run.Queue.Due(entry.DomainId, now)
		if err != nil {
			return nil, err
		}
		due = append(due, items...)
	}
	return due, nil
}

func (run QueueRun) deliverAll(due []Item, now time.Time) (map[string]int, error) {
	counts := map[string]int{}
	failures := []error{}
	for _, item := range due {
		done, err := run.Delivery.Deliver(item, now)
		counts[done.State]++
		failures = append(failures, err)
	}
	return counts, errors.Join(failures...)
}

func (QueueRun) report(picked int, counts map[string]int) QueueReport {
	states := States{}
	return QueueReport{Picked: picked, Sent: counts[states.Sent()], Deferred: counts[states.Deferred()], Bounced: counts[states.Bounced()], Expired: counts[states.Expired()], Failed: counts[states.Failed()]}
}
