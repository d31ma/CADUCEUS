type EventStatusUtils struct{}

func (EventStatusUtils) ForState(state string) string {
	status, found := map[string]string{"expired": "failed"}[state]
	return EventStatusUtils{}.pick(status, found, state)
}

func (EventStatusUtils) pick(status string, found bool, state string) string {
	if found {
		return status
	}
	return state
}
