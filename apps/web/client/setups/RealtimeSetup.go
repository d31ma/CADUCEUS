// @Setup
type RealtimeSetup struct{}

func (RealtimeSetup) Configure(execution *BehaviorExecution) (*BehaviorConfiguration, error) {
	return NewBehaviorConfiguration(), nil
}
