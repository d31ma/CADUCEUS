// @Setup
type NotifySetup struct{}

func (NotifySetup) Configure(execution *BehaviorExecution) (*BehaviorConfiguration, error) {
	return NewBehaviorConfiguration(), nil
}
