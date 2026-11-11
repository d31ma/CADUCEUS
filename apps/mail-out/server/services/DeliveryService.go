import (
	"errors"
	"time"
)

// @Service
type DeliveryService struct {
	// @Needs
	outbound *OutboundStoreDelegate
	// @Needs
	suppressions *SuppressionStoreDelegate
	// @Needs
	builder *MessageBuilderService
	// @Needs
	transport *TransportService
	// @Needs
	outcomes *OutcomeService
}

func (DeliveryService) queueName() string { return "queue" }

func (delivery *DeliveryService) Deliver(item OutboundItem, now time.Time, authorize func(string) (string, error)) (OutboundItem, error) {
	claimed, free, claimErr := delivery.outbound.Claim(item, TimeUtils{}.Format(now))
	suppressed, checkErr := delivery.suppressions.Contains(item.DomainId, item.Rcpt)
	attempt := Attempt{Item: claimed, Now: TimeUtils{}.Format(now), Suppressed: suppressed, Free: free}
	outcome, prepared := delivery.attempt(attempt, errors.Join(claimErr, checkErr), authorize)
	return delivery.finish(attempt, prepared, outcome, authorize)
}

func (delivery *DeliveryService) attempt(attempt Attempt, failure error, authorize func(string) (string, error)) (Outcome, OutboundItem) {
	item := attempt.Item
	switch {
	case !attempt.Free:
		return Outcome{}, item
	case failure != nil:
		return Outcome{Status: OutboundStateUtils{}.Deferred(), Transport: delivery.queueName(), Detail: failure.Error()}, item
	case TimeUtils{}.Parse(item.ExpiresAt).Before(TimeUtils{}.Parse(attempt.Now)):
		return OutcomeUtils{}.Expired(), item
	case attempt.Suppressed:
		return OutcomeUtils{}.Suppressed(), item
	default:
		return delivery.send(item, TimeUtils{}.Parse(attempt.Now), authorize)
	}
}

func (delivery *DeliveryService) send(item OutboundItem, now time.Time, authorize func(string) (string, error)) (Outcome, OutboundItem) {
	prepared, err := delivery.builder.Prepare(item, now, authorize)
	return delivery.sendPrepared(prepared, err, authorize), prepared.Item
}

func (delivery *DeliveryService) sendPrepared(prepared Prepared, failure error, authorize func(string) (string, error)) Outcome {
	switch {
	case failure != nil:
		return OutcomeUtils{}.FromBuild(failure)
	case prepared.Refusal != "":
		return OutcomeUtils{}.Refused(prepared.Refusal)
	default:
		return delivery.transport.Deliver(prepared.Item, []byte(prepared.Raw), authorize)
	}
}

func (delivery *DeliveryService) finish(attempt Attempt, prepared OutboundItem, outcome Outcome, authorize func(string) (string, error)) (OutboundItem, error) {
	if !attempt.Free {
		return prepared, nil
	}
	return delivery.outcomes.Apply(prepared, outcome, TimeUtils{}.Parse(attempt.Now), authorize)
}
