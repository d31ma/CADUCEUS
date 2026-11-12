type PushTurnedOff struct{}

func (failure *PushTurnedOff) Error() string {
	return "web push is turned off (notify.push.enabled is false)"
}
