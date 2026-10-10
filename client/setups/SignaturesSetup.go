// @Setup
type SignaturesSetup struct{}

func (SignaturesSetup) Configure(execution *BehaviorExecution) (*BehaviorConfiguration, error) {
	return NewBehaviorConfiguration(), nil
}
