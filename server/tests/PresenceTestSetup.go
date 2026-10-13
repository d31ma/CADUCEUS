// @TestSetup
type PresenceTestSetup struct{}

func (PresenceTestSetup) Configure(execution *BehaviorExecution) (*BehaviorConfiguration, error) {
	return NewBehaviorConfiguration(), nil
}
