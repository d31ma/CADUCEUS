// @Service
type OutboxStatesService struct{}

func (states *OutboxStatesService) Pending(count int) string {
	return states.pendingState(count)
}

func (states *OutboxStatesService) Dead(count int) string {
	return states.deadState(count)
}

func (states *OutboxStatesService) Waiting() string {
	return "pending"
}

func (states *OutboxStatesService) Failed() string {
	return "dead"
}

func (states *OutboxStatesService) pendingState(count int) string {
	if count > 0 {
		return states.Waiting()
	}
	return "settled"
}

func (states *OutboxStatesService) deadState(count int) string {
	if count > 0 {
		return states.Failed()
	}
	return "none"
}
