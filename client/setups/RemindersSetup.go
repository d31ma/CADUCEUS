// @Setup
type RemindersSetup struct{}

func (RemindersSetup) Configure(execution *BehaviorExecution) (*BehaviorConfiguration, error) {
	return NewBehaviorConfiguration(), nil
}
