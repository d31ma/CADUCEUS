// @TestSetup
type SuppressedTestSetup struct{}

func (SuppressedTestSetup) Configure(execution *BehaviorExecution) (*BehaviorConfiguration, error) {
	return NewBehaviorConfiguration(), nil
}
