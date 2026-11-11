package mailout

import "time"

type Delivery struct {
	Queue        Queue
	Suppressions Suppressions
	Builder      Builder
	Transport    Transport
	Outcomes     Outcomes
}

func (delivery Delivery) Deliver(item Item, now time.Time) (Item, error) {
	claimed, free, err := delivery.Queue.Claim(item, now)
	if err != nil {
		return item, err
	}
	return delivery.deliverFree(claimed, free, now)
}

func (delivery Delivery) deliverFree(claimed Item, free bool, now time.Time) (Item, error) {
	if !free {
		return claimed, nil
	}
	return delivery.process(claimed, now)
}

func (delivery Delivery) process(claimed Item, now time.Time) (Item, error) {
	attempted, outcome := delivery.attempt(claimed, now)
	return delivery.Outcomes.Apply(attempted, outcome, now)
}

func (delivery Delivery) attempt(item Item, now time.Time) (Item, Outcome) {
	rules := OutcomeRules{}
	suppressed, err := delivery.Suppressions.Contains(item.DomainId, item.Rcpt)
	switch {
	case err != nil:
		return item, rules.Unavailable(err)
	case Clock{}.Parse(item.ExpiresAt).Before(now):
		return item, rules.Expired()
	case suppressed:
		return item, rules.Suppressed()
	default:
		return delivery.prepareAndSend(item, now)
	}
}

func (delivery Delivery) prepareAndSend(item Item, now time.Time) (Item, Outcome) {
	prepared, err := delivery.Builder.Prepare(item, now)
	return delivery.send(prepared, err)
}

func (delivery Delivery) send(prepared Prepared, err error) (Item, Outcome) {
	rules := OutcomeRules{}
	switch {
	case err != nil:
		return prepared.Item, rules.FromBuild(err)
	case prepared.Refusal != "":
		return prepared.Item, rules.Refused(prepared.Refusal)
	default:
		return prepared.Item, delivery.Transport.Deliver(prepared.Item, prepared.Raw)
	}
}
