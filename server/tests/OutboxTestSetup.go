// @TestSetup
type OutboxTestSetup struct{}

func (OutboxTestSetup) Configure(execution *BehaviorExecution) (*BehaviorConfiguration, error) {
	return NewBehaviorConfiguration(), nil
}
