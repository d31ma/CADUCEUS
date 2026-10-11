// @Setup
type ExtensionsSetup struct{}

func (ExtensionsSetup) Configure(execution *BehaviorExecution) (*BehaviorConfiguration, error) {
	return NewBehaviorConfiguration(), nil
}
