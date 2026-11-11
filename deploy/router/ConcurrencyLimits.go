package main

type ConcurrencyLimits struct {
	Slots map[string]chan struct{}
}

func (ConcurrencyLimits) Of(functions []string, writers []string, prefix string, others int) ConcurrencyLimits {
	slots := map[string]chan struct{}{}
	for _, service := range functions {
		slots[prefix+service] = make(chan struct{}, others)
	}
	for _, service := range writers {
		slots[prefix+service] = make(chan struct{}, 1)
	}
	return ConcurrencyLimits{Slots: slots}
}

func (limits ConcurrencyLimits) Slot(function string) chan struct{} {
	return limits.Slots[function]
}
