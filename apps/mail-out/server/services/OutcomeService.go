import (
	"errors"
	"time"

	"mesail.dev/packages/events"
)

// @Service
type OutcomeService struct {
	// @Needs
	outbound *OutboundStoreDelegate
	// @Needs
	suppressions *SuppressionStoreDelegate
	// @Needs
	attempts *AttemptStoreDelegate
	// @Needs
	relay *OutboxRelayService
	// @Needs
	settings *SettingsDelegate
}

func (outcomes *OutcomeService) Apply(item OutboundItem, outcome Outcome, now time.Time, authorize func(string) (string, error)) (OutboundItem, error) {
	item.Attempts++
	retryAt := now.Add(RetryUtils{}.Delay(item.Attempts, outcomes.settings.FirstDelay(), outcomes.settings.MaxDelay()))
	item.State = outcomes.state(outcome.Status, retryAt.After(TimeUtils{}.Parse(item.ExpiresAt)))
	item.NextAttemptAt = TimeUtils{}.Format(retryAt)
	item.Detail = outcome.Detail
	recordErr := outcomes.attempts.Record(outcomes.attempt(item, outcome, now))
	suppressErr := outcomes.suppress(item, outcome, now)
	change, changeErr := ChangeUtils{}.Envelope(item)
	saveErr := outcomes.save(item, change, changeErr)
	_, relayErr := outcomes.relay.RelayDocument(StorageUtils{}.QueueRef(item), item.DomainId, authorize)
	return item, errors.Join(recordErr, suppressErr, saveErr, relayErr)
}

func (OutcomeService) state(status string, pastExpiry bool) string {
	deferred := OutboundStateUtils{}.Deferred()
	if status == deferred && pastExpiry {
		return OutboundStateUtils{}.Expired()
	}
	return status
}

func (OutcomeService) attempt(item OutboundItem, outcome Outcome, now time.Time) DeliveryAttempt {
	return DeliveryAttempt{DomainId: item.DomainId, ItemId: item.Id, Rcpt: item.Rcpt, Transport: outcome.Transport, Status: item.State, Code: outcome.Code, Enhanced: outcome.Enhanced, Detail: outcome.Detail, At: TimeUtils{}.Format(now)}
}

func (outcomes *OutcomeService) suppress(item OutboundItem, outcome Outcome, now time.Time) error {
	bounced := OutboundStateUtils{}.Bounced()
	mailboxGone := ReplyUtils{}.MailboxGone(outcome.Enhanced)
	if item.State != bounced || !mailboxGone {
		return nil
	}
	return outcomes.suppressions.Add(Suppression{DomainId: item.DomainId, Address: item.Rcpt, Reason: outcome.Enhanced + " " + outcome.Detail, CreatedAt: TimeUtils{}.Format(now)})
}

func (outcomes *OutcomeService) save(item OutboundItem, change events.Envelope, failure error) error {
	if failure != nil {
		return errors.Join(failure, outcomes.outbound.Save(item))
	}
	return outcomes.outbound.SaveWith(item, change)
}
