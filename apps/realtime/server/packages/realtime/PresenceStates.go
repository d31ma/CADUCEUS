package realtime

type PresenceStates struct{}

func (PresenceStates) Active() string {
	return "active"
}

func (PresenceStates) Idle() string {
	return "idle"
}

func (PresenceStates) DoNotDisturb() string {
	return "dnd"
}

func (PresenceStates) Offline() string {
	return "offline"
}

func (states PresenceStates) Online() []string {
	return []string{states.Active(), states.Idle(), states.DoNotDisturb()}
}
