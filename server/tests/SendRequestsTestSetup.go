// @TestSetup
type SendRequestsTestSetup struct{}

func (SendRequestsTestSetup) Configure(execution *BehaviorExecution) (*BehaviorConfiguration, error) {
	return NewBehaviorConfiguration(), nil
}
