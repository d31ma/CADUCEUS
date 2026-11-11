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
	if err != nil || !free {
		return item, err
	}
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
	}
	prepared, err := delivery.Builder.Prepare(item, now)
	switch {
	case err != nil:
		return prepared.Item, rules.FromBuild(err)
	case prepared.Refusal != "":
		return prepared.Item, rules.Refused(prepared.Refusal)
	default:
		return prepared.Item, delivery.Transport.Deliver(prepared.Item, prepared.Raw)
	}
}
