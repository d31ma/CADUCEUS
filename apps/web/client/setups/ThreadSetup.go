// @Setup
type ThreadSetup struct{}

func (ThreadSetup) Configure(execution *BehaviorExecution) (*BehaviorConfiguration, error) {
	return NewBehaviorConfiguration(), nil
}
