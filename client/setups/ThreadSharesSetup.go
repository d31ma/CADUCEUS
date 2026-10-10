// @Setup
type ThreadSharesSetup struct{}

func (ThreadSharesSetup) Configure(execution *BehaviorExecution) (*BehaviorConfiguration, error) {
	return NewBehaviorConfiguration(), nil
}
