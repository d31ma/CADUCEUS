// @TestSetup
type InboundTestSetup struct{}

func (InboundTestSetup) Configure(execution *BehaviorExecution) (*BehaviorConfiguration, error) {
	return NewBehaviorConfiguration(), nil
}
