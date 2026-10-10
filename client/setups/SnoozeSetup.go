// @Setup
type SnoozeSetup struct{}

func (SnoozeSetup) Configure(execution *BehaviorExecution) (*BehaviorConfiguration, error) {
	return NewBehaviorConfiguration(), nil
}
