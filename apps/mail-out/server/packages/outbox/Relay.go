package outbox

import (
	"errors"
	"time"
)

type Relay struct {
	store    Store
	pusher   Pusher
	settings Settings
	policy   RetryPolicy
	now      func() time.Time
}

func NewRelay(store Store, pusher Pusher, settings Settings, now func() time.Time) Relay {
	return Relay{store: store, pusher: pusher, settings: settings, policy: NewRetryPolicy(settings.Retry), now: now}
}

func (relay Relay) RunOnce(limit int) (Report, error) {
	documents, err := relay.store.Pending(limit)
	if err != nil {
		return Report{}, err
	}
	report, failed := relay.relayAll(documents)
	report.More = len(documents) >= limit
	return report, failed
}

func (relay Relay) RunWithin(limit int, budget time.Duration) (Report, error) {
	documents, err := relay.store.Pending(limit)
	if err != nil {
		return Report{}, err
	}
	report, failed := relay.relayUntil(documents, relay.now().Add(budget))
	report.More = relay.more(report, len(documents), limit)
	return report, failed
}

func (relay Relay) RelayDocument(ref Ref) (Report, error) {
	return relay.relayLoaded(ref, relay.drainedRun)
}

func (relay Relay) SweepDocument(ref Ref) (Report, error) {
	return relay.relayLoaded(ref, relay.directRun)
}

func (relay Relay) relayLoaded(ref Ref, newRun func([]PendingEvent) *documentRun) (Report, error) {
	fields, err := relay.store.Load(ref)
	if err != nil {
		return Report{}, err
	}
	return relay.relayWith(Document{Ref: ref, Fields: fields}, newRun)
}

func (relay Relay) relayAll(documents []Document) (Report, error) {
	total := Report{}
	failures := []error{}
	for _, document := range documents {
		report, err := relay.relayOne(document)
		total = total.plus(report)
		failures = append(failures, err)
	}
	return total, errors.Join(failures...)
}

func (relay Relay) relayUntil(documents []Document, deadline time.Time) (Report, error) {
	total := Report{}
	failures := []error{}
	for _, document := range documents {
		report, err := relay.relayBefore(document, deadline)
		total = total.plus(report)
		failures = append(failures, err)
	}
	return total, errors.Join(failures...)
}

func (relay Relay) relayBefore(document Document, deadline time.Time) (Report, error) {
	if !relay.now().Before(deadline) {
		return Report{More: true}, nil
	}
	return relay.relayOne(document)
}

func (Relay) more(report Report, found int, limit int) bool {
	return report.More || found >= limit
}

func (relay Relay) relayOne(document Document) (Report, error) {
	return relay.relayWith(document, relay.directRun)
}

func (relay Relay) relayWith(document Document, newRun func([]PendingEvent) *documentRun) (Report, error) {
	state, err := Codec{}.Read(document.Fields)
	if err != nil {
		return Report{}, err
	}
	outcome := newRun(state.Pending).deliver(state.Pending)
	fresh, err := relay.store.Load(document.Ref)
	if err != nil {
		return outcome.report, err
	}
	merged, err := merge{outcome: outcome}.apply(fresh)
	if err != nil {
		return outcome.report, err
	}
	return outcome.report, relay.store.Save(document.Ref, merged)
}

func (relay Relay) directRun(pending []PendingEvent) *documentRun {
	return relay.newRun(directPushes{pusher: relay.pusher, subscribers: relay.settings.subscriberMap()}, relay.now())
}

func (relay Relay) drainedRun(pending []PendingEvent) *documentRun {
	now := relay.now()
	drain := timedDrain{pusher: relay.pusher, subscribers: relay.settings.subscriberMap(), now: now, pushTimeout: relay.pushTimeout(), drainTimeout: relay.drainTimeout()}
	return relay.newRun(drain.answers(pending), now)
}

func (relay Relay) newRun(pushes pushSource, now time.Time) *documentRun {
	return &documentRun{
		pushes:  pushes,
		policy:  relay.policy,
		now:     now,
		blocked: map[string]bool{},
		outcome: runOutcome{updated: map[string]PendingEvent{}, deadLetters: []DeadLetter{}, report: Report{Documents: 1}},
	}
}

func (Relay) pushTimeout() time.Duration {
	return 2 * time.Second
}

func (Relay) drainTimeout() time.Duration {
	return 3 * time.Second
}
