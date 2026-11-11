package mailout

import (
	"errors"
	"time"

	"mesail.dev/packages/events"
)

type Outcomes struct {
	Queue        Queue
	Suppressions Suppressions
	Attempts     AttemptLog
	Relayer      Relayer
	Backoff      Backoff
	Changes      ChangeEvents
}

func (outcomes Outcomes) Apply(item Item, outcome Outcome, now time.Time) (Item, error) {
	item = outcomes.advance(item, outcome, now)
	err := outcomes.Attempts.Record(outcomes.attempt(item, outcome, now))
	err = errors.Join(err, outcomes.suppressGone(item, outcome, now))
	return item, errors.Join(err, outcomes.save(item))
}

func (outcomes Outcomes) advance(item Item, outcome Outcome, now time.Time) Item {
	item.Attempts++
	retryAt := now.Add(outcomes.Backoff.Delay(item.Attempts))
	item.State = outcomes.stateAfter(outcome.Status, retryAt, item.ExpiresAt)
	item.NextAttemptAt, item.Detail = Clock{}.Format(retryAt), outcome.Detail
	return item
}

func (outcomes Outcomes) stateAfter(status string, retryAt time.Time, expiresAt string) string {
	if outcomes.retryTooLate(status, retryAt, expiresAt) {
		return States{}.Expired()
	}
	return status
}

func (Outcomes) retryTooLate(status string, retryAt time.Time, expiresAt string) bool {
	return status == States{}.Deferred() && retryAt.After(Clock{}.Parse(expiresAt))
}

func (Outcomes) attempt(item Item, outcome Outcome, now time.Time) DeliveryAttempt {
	return DeliveryAttempt{DomainId: item.DomainId, ItemId: item.Id, Rcpt: item.Rcpt, Transport: outcome.Transport, Status: item.State, Code: outcome.Code, Enhanced: outcome.Enhanced, Detail: outcome.Detail, At: Clock{}.Format(now)}
}

func (outcomes Outcomes) suppressGone(item Item, outcome Outcome, now time.Time) error {
	if !outcomes.mailboxGone(item.State, outcome.Enhanced) {
		return nil
	}
	return outcomes.Suppressions.Add(Suppression{DomainId: item.DomainId, Address: item.Rcpt, Reason: outcome.Enhanced + " " + outcome.Detail, CreatedAt: Clock{}.Format(now)})
}

func (Outcomes) mailboxGone(state string, enhanced string) bool {
	return state == States{}.Bounced() && OutcomeRules{}.MailboxGone(enhanced)
}

func (outcomes Outcomes) save(item Item) error {
	change, err := outcomes.Changes.Envelope(item)
	if err != nil {
		return errors.Join(err, outcomes.Queue.Save(item))
	}
	return outcomes.saveWith(item, change)
}

func (outcomes Outcomes) saveWith(item Item, change events.Envelope) error {
	err := outcomes.Queue.SaveWith(item, change)
	_, relayErr := outcomes.Relayer.RelayDocument(outcomes.Queue.Ref(item), item.DomainId)
	return errors.Join(err, relayErr)
}
