// @TestSetup
type ShellTestSetup struct{}

func (ShellTestSetup) Configure(execution *BehaviorExecution) (*BehaviorConfiguration, error) {
	return NewBehaviorConfiguration(), nil
}
