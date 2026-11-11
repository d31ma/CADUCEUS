// @Setup
type IdentitySetup struct{}

func (IdentitySetup) Configure(execution *BehaviorExecution) (*BehaviorConfiguration, error) {
	return NewBehaviorConfiguration(), nil
}
