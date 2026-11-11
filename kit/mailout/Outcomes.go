package mailout

import (
	"errors"
	"time"
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
	states := States{}
	item.Attempts++
	retryAt := now.Add(outcomes.Backoff.Delay(item.Attempts))
	item.State, item.NextAttemptAt, item.Detail = outcome.Status, Clock{}.Format(retryAt), outcome.Detail
	if item.State == states.Deferred() && retryAt.After(Clock{}.Parse(item.ExpiresAt)) {
		item.State = states.Expired()
	}
	err := outcomes.Attempts.Record(DeliveryAttempt{DomainId: item.DomainId, ItemId: item.Id, Rcpt: item.Rcpt, Transport: outcome.Transport, Status: item.State, Code: outcome.Code, Enhanced: outcome.Enhanced, Detail: outcome.Detail, At: Clock{}.Format(now)})
	if item.State == states.Bounced() && (OutcomeRules{}).MailboxGone(outcome.Enhanced) {
		err = errors.Join(err, outcomes.Suppressions.Add(Suppression{DomainId: item.DomainId, Address: item.Rcpt, Reason: outcome.Enhanced + " " + outcome.Detail, CreatedAt: Clock{}.Format(now)}))
	}
	change, changeErr := outcomes.Changes.Envelope(item)
	if changeErr != nil {
		return item, errors.Join(err, changeErr, outcomes.Queue.Save(item))
	}
	err = errors.Join(err, outcomes.Queue.SaveWith(item, change))
	_, relayErr := outcomes.Relayer.RelayDocument(outcomes.Queue.Ref(item), item.DomainId)
	return item, errors.Join(err, relayErr)
}
