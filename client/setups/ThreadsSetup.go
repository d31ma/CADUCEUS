// @Setup
type ThreadsSetup struct{}

func (ThreadsSetup) Configure(execution *BehaviorExecution) (*BehaviorConfiguration, error) {
	return NewBehaviorConfiguration(), nil
}
