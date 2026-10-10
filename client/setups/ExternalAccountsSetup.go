// @Setup
type ExternalAccountsSetup struct{}

func (ExternalAccountsSetup) Configure(execution *BehaviorExecution) (*BehaviorConfiguration, error) {
	return NewBehaviorConfiguration(), nil
}
