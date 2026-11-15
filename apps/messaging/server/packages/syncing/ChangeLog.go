package syncing

import (
	"sync"
	"time"

	"mesail.dev/packages/core"
	"mesail.dev/packages/events"
	"mesail.dev/packages/outbox"
)

type ChangeLog struct {
	next     outbox.Pusher
	rows     changeRows
	heads    changeHeads
	recorded map[string]bool
	lock     *sync.Mutex
}

func NewChangeLog(next outbox.Pusher, stores core.Stores, now func() time.Time) ChangeLog {
	return ChangeLog{next: next, rows: changeRows{changes: stores.Changes, now: now}, heads: changeHeads{heads: stores.ChangeHeads}, recorded: map[string]bool{}, lock: &sync.Mutex{}}
}

func (log ChangeLog) Push(subscriber outbox.Subscriber, envelope events.Envelope) error {
	err := log.Record(envelope)
	if err != nil {
		return err
	}
	return log.next.Push(subscriber, envelope)
}

func (log ChangeLog) Record(envelope events.Envelope) error {
	log.lock.Lock()
	defer log.lock.Unlock()
	return log.recordOnce(envelope)
}

func (log ChangeLog) recordOnce(envelope events.Envelope) error {
	if log.recorded[envelope.EventID] {
		return nil
	}
	err := log.recordNow(envelope)
	log.recorded[envelope.EventID] = err == nil
	return err
}

func (log ChangeLog) recordNow(envelope events.Envelope) error {
	entries, err := changeEntries{}.of(envelope)
	if err != nil {
		return err
	}
	rows, err := log.rows.ensured(envelope, entries)
	if err != nil {
		return err
	}
	return log.heads.raised(envelope.DomainId, rows)
}
