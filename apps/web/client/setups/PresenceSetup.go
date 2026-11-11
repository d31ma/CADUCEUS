// @Setup
type PresenceSetup struct{}

func (PresenceSetup) Configure(execution *BehaviorExecution) (*BehaviorConfiguration, error) {
	return NewBehaviorConfiguration(), nil
}
