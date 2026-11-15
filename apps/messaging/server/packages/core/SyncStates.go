package core

type SyncStates struct{}

func (SyncStates) Changed() string {
	return "changed"
}

func (SyncStates) Unchanged() string {
	return "unchanged"
}

func (SyncStates) Reset() string {
	return "reset"
}

func (SyncStates) Gone() string {
	return "gone"
}
