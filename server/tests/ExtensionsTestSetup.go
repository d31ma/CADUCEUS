// @TestSetup
type ExtensionsTestSetup struct{}

func (ExtensionsTestSetup) Configure(execution *BehaviorExecution) (*BehaviorConfiguration, error) {
	return NewBehaviorConfiguration(), nil
}
