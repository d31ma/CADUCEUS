package recipients

type Policy struct {
	kinds      Kinds
	deliveries Deliveries
}

func (policy Policy) Allowed(kind Kind) []Delivery {
	return policy.allowedByKind()[kind]
}

func (policy Policy) Default(kind Kind) Delivery {
	return policy.defaultByKind()[kind]
}

func (policy Policy) Check(kind Kind, wanted Delivery) error {
	allowed := policy.contains(policy.Allowed(kind), wanted)
	return policy.refusal(allowed, RefusedDelivery{Kind: kind, Wanted: wanted})
}

func (policy Policy) allowedByKind() map[Kind][]Delivery {
	return map[Kind][]Delivery{
		policy.kinds.Member():      {policy.deliveries.Chat(), policy.deliveries.Email(), policy.deliveries.Both()},
		policy.kinds.Channel():     {policy.deliveries.Chat()},
		policy.kinds.OtherDomain(): {policy.deliveries.Email()},
		policy.kinds.External():    {policy.deliveries.Email()},
	}
}

func (policy Policy) defaultByKind() map[Kind]Delivery {
	return map[Kind]Delivery{
		policy.kinds.Member():      policy.deliveries.Chat(),
		policy.kinds.Channel():     policy.deliveries.Chat(),
		policy.kinds.OtherDomain(): policy.deliveries.Email(),
		policy.kinds.External():    policy.deliveries.Email(),
	}
}

func (policy Policy) contains(allowed []Delivery, wanted Delivery) bool {
	for _, delivery := range allowed {
		if delivery == wanted {
			return true
		}
	}
	return false
}

func (policy Policy) refusal(allowed bool, refused RefusedDelivery) error {
	if allowed {
		return nil
	}
	return refused
}
